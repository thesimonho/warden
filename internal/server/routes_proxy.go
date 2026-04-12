package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"sync"

	"github.com/thesimonho/warden/api"
)

// proxySubdomainSuffix is the Host suffix that identifies a proxy request.
// Browsers resolve *.localhost to 127.0.0.1 per RFC 6761.
const proxySubdomainSuffix = ".localhost"

// proxyKey identifies a cached reverse proxy transport.
type proxyKey struct {
	projectID string
	agentType string
	port      int
}

// cachedProxy holds a reverse proxy transport and the target address it
// was created for. When the target changes (e.g. container restart on
// native Docker, or new bridge port on Desktop), the cached entry is
// replaced. The transport is shared across requests for connection reuse.
type cachedProxy struct {
	targetAddr string // "host:port" used for staleness check
	target     *url.URL
	transport  http.RoundTripper
}

// proxyRouter intercepts requests to {projectId}-{agentType}-{port}.localhost
// and reverse-proxies them to the container. Non-matching requests are
// passed to the next handler (the normal API mux).
type proxyRouter struct {
	svc  proxyService
	next http.Handler

	mu    sync.Mutex
	cache map[proxyKey]*cachedProxy
}

// proxyService is the subset of service.Service needed by the proxy router.
type proxyService interface {
	ProxyPort(ctx context.Context, projectID, agentType string, port int) (string, int, error)
}

// newProxyRouter wraps an http.Handler with subdomain-based port forwarding.
func newProxyRouter(svc proxyService, next http.Handler) *proxyRouter {
	return &proxyRouter{
		svc:   svc,
		next:  next,
		cache: make(map[proxyKey]*cachedProxy),
	}
}

// ServeHTTP checks the Host header for the proxy subdomain pattern.
// Format: {projectId}-{agentType}-{port}.localhost[:serverPort]
func (pr *proxyRouter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	host := r.Host

	// Strip the server port (e.g. ":8090") to isolate the hostname.
	hostname := host
	if idx := strings.LastIndex(host, ":"); idx != -1 {
		hostname = host[:idx]
	}

	// Check for the .localhost suffix.
	if !strings.HasSuffix(hostname, proxySubdomainSuffix) {
		pr.next.ServeHTTP(w, r)
		return
	}

	// Extract the subdomain prefix (everything before .localhost).
	subdomain := strings.TrimSuffix(hostname, proxySubdomainSuffix)
	if subdomain == "" {
		pr.next.ServeHTTP(w, r)
		return
	}

	// Parse {projectId}-{agentType}-{port} from the subdomain.
	projectID, agentType, port, ok := parseProxySubdomain(subdomain)
	if !ok {
		writeError(w, ErrCodeInvalidBody, "invalid proxy subdomain format", http.StatusBadRequest)
		return
	}

	targetHost, targetPort, err := pr.svc.ProxyPort(r.Context(), projectID, agentType, port)
	if err != nil {
		if writeServiceError(w, err) {
			return
		}
		writeError(w, ErrCodeInternal, err.Error(), http.StatusInternalServerError)
		return
	}

	cached := pr.getOrCreateProxy(projectID, agentType, port, targetHost, targetPort)

	proxy := &httputil.ReverseProxy{
		Transport: cached.transport,
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(cached.target)
			pr.Out.URL.Path = r.URL.Path
			pr.Out.URL.RawQuery = r.URL.RawQuery
			pr.Out.Host = cached.target.Host
			// Preserve the original browser Host so upstream CSRF
			// checks (e.g. Next.js Server Actions) see the proxy
			// hostname, not the internal backend address.
			pr.Out.Header.Set("X-Forwarded-Host", r.Host)
		},
		ErrorHandler: func(rw http.ResponseWriter, _ *http.Request, proxyErr error) {
			slog.Warn("proxy error", "project", projectID, "port", port, "err", proxyErr)
			if strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
				return
			}
			writeError(rw, ErrCodeProxyError, "container unreachable", http.StatusBadGateway)
		},
	}

	proxy.ServeHTTP(w, r)
}

// parseProxySubdomain extracts projectID, agentType, and port from a
// subdomain like "04f400635297-claude-code-5173". The port is always
// the last segment. The agent type may contain hyphens (e.g. "claude-code").
func parseProxySubdomain(subdomain string) (projectID, agentType string, port int, ok bool) {
	// Port is the last hyphen-separated segment.
	lastDash := strings.LastIndex(subdomain, "-")
	if lastDash == -1 || lastDash == 0 {
		return "", "", 0, false
	}

	portStr := subdomain[lastDash+1:]
	port, err := strconv.Atoi(portStr)
	if err != nil || !api.IsValidPort(port) {
		return "", "", 0, false
	}

	remainder := subdomain[:lastDash]

	// Project ID is the first hyphen-separated segment (12-char hex).
	firstDash := strings.Index(remainder, "-")
	if firstDash == -1 {
		return "", "", 0, false
	}

	projectID = remainder[:firstDash]
	agentType = remainder[firstDash+1:]

	if projectID == "" || agentType == "" {
		return "", "", 0, false
	}

	return projectID, agentType, port, true
}

// getOrCreateProxy returns a cached transport and target URL for the
// given project port, creating one if the cache is empty or the target
// address has changed (e.g. container restart or new bridge port).
// containerPort is the stable port from the subdomain (used as cache
// key); targetHost:targetPort is the actual backend address (which may
// differ on Docker Desktop where an ephemeral bridge port is used).
func (pr *proxyRouter) getOrCreateProxy(projectID, agentType string, containerPort int, targetHost string, targetPort int) *cachedProxy {
	key := proxyKey{projectID: projectID, agentType: agentType, port: containerPort}
	addr := fmt.Sprintf("%s:%d", targetHost, targetPort)

	pr.mu.Lock()
	defer pr.mu.Unlock()

	if cached, ok := pr.cache[key]; ok && cached.targetAddr == addr {
		return cached
	}

	target, _ := url.Parse(fmt.Sprintf("http://%s", addr))
	cached := &cachedProxy{
		targetAddr: addr,
		target:     target,
		transport:  http.DefaultTransport,
	}
	pr.cache[key] = cached
	return cached
}

// validateForwardedPorts checks that all ports are in the valid range.
// Returns a non-empty error message on failure.
func validateForwardedPorts(ports []int) string {
	for _, p := range ports {
		if !api.IsValidPort(p) {
			return fmt.Sprintf("invalid forwarded port: %d (must be 1-65535)", p)
		}
	}
	return ""
}
