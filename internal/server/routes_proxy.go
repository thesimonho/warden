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

// proxyTransport is a shared HTTP transport for proxied requests with a
// higher idle connection limit than the default (2). Dev servers trigger
// burst loads (JS bundles, images, HMR WebSocket) that benefit from
// keeping more connections alive — especially on Docker Desktop where
// each new TCP connection spawns a docker exec process.
var proxyTransport = &http.Transport{
	MaxIdleConnsPerHost: 16,
}

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

	proxyOrigin := fmt.Sprintf("http://%s", r.Host)

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
			pr.Out.Header.Set("X-Forwarded-Proto", "http")
			// Rewrite Origin to match the target so upstream CORS
			// checks (e.g. Expo's CorsMiddleware) see a same-origin
			// request instead of the proxy subdomain origin.
			if pr.Out.Header.Get("Origin") != "" {
				pr.Out.Header.Set("Origin", cached.target.String())
			}
			// Rewrite Referer authority to match the target while
			// preserving the path+query. Dev servers (Expo, Vite)
			// validate Referer the same way they validate Origin.
			if ref := pr.Out.Header.Get("Referer"); ref != "" {
				pr.Out.Header.Set("Referer", rewriteReferer(ref, cached.target))
			}
		},
		// Rewrite Location headers in redirects from the internal
		// container address back to the proxy subdomain so the
		// browser can follow them.
		ModifyResponse: func(resp *http.Response) error {
			loc := resp.Header.Get("Location")
			if loc == "" {
				return nil
			}
			resp.Header.Set("Location", rewriteLocation(loc, cached.target, proxyOrigin))
			return nil
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

// rewriteReferer replaces the scheme+authority of a Referer URL with the
// proxy target's origin while preserving the path and query string.
// Returns the original value unchanged if parsing fails.
func rewriteReferer(referer string, target *url.URL) string {
	parsed, err := url.Parse(referer)
	if err != nil {
		return referer
	}
	parsed.Scheme = target.Scheme
	parsed.Host = target.Host
	return parsed.String()
}

// rewriteLocation replaces the upstream target origin in a Location header
// with the browser-facing proxy origin so redirects are followable.
// Relative and non-matching Location values are returned unchanged.
func rewriteLocation(loc string, target *url.URL, proxyOrigin string) string {
	targetOrigin := target.String()
	if strings.HasPrefix(loc, targetOrigin) {
		return proxyOrigin + strings.TrimPrefix(loc, targetOrigin)
	}
	return loc
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
		transport:  proxyTransport,
	}
	pr.cache[key] = cached
	return cached
}

// evictProject removes all cached proxy entries for the given project.
// Called when a project's container is deleted or stopped so stale
// entries don't accumulate indefinitely.
func (pr *proxyRouter) evictProject(projectID, agentType string) {
	if pr == nil {
		return
	}

	pr.mu.Lock()
	defer pr.mu.Unlock()

	for key := range pr.cache {
		if key.projectID == projectID && key.agentType == agentType {
			delete(pr.cache, key)
		}
	}
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
