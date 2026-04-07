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

// cachedProxy holds a reverse proxy transport and the container IP it was
// created for. When the container IP changes (e.g. after a restart), the
// cached entry is replaced. The transport is shared across requests to
// enable HTTP connection reuse.
type cachedProxy struct {
	containerIP string
	target      *url.URL
	transport   http.RoundTripper
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
	ProxyPort(ctx context.Context, projectID, agentType string, port int) (string, error)
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

	containerIP, err := pr.svc.ProxyPort(r.Context(), projectID, agentType, port)
	if err != nil {
		if writeServiceError(w, err) {
			return
		}
		writeError(w, ErrCodeInternal, err.Error(), http.StatusInternalServerError)
		return
	}

	cached := pr.getOrCreateProxy(projectID, agentType, port, containerIP)

	proxy := &httputil.ReverseProxy{
		Transport: cached.transport,
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(cached.target)
			pr.Out.URL.Path = r.URL.Path
			pr.Out.URL.RawQuery = r.URL.RawQuery
			pr.Out.Host = cached.target.Host
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

// getOrCreateProxy returns a cached transport and target URL for the given
// container, creating one if the cache is empty or the container IP has
// changed.
func (pr *proxyRouter) getOrCreateProxy(projectID, agentType string, port int, containerIP string) *cachedProxy {
	key := proxyKey{projectID: projectID, agentType: agentType, port: port}

	pr.mu.Lock()
	defer pr.mu.Unlock()

	if cached, ok := pr.cache[key]; ok && cached.containerIP == containerIP {
		return cached
	}

	target, _ := url.Parse(fmt.Sprintf("http://%s:%d", containerIP, port))
	cached := &cachedProxy{
		containerIP: containerIP,
		target:      target,
		transport:   http.DefaultTransport,
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
