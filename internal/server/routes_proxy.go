package server

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
)

// cachedProxy holds a reverse proxy transport and the container IP it was
// created for. When the container IP changes (e.g. after a restart), the
// cached entry is replaced. The transport is shared across requests to
// enable HTTP connection reuse.
type cachedProxy struct {
	containerIP string
	target      *url.URL
	transport   http.RoundTripper
}

// handleProxy reverse-proxies HTTP and WebSocket traffic to a declared
// port inside the project's container. The port must be in the project's
// forwardedPorts list.
//
//	@Summary      Proxy to container port
//	@Description  Reverse-proxies HTTP and WebSocket requests to the specified port
//	@Description  inside the project's container. The port must be declared in the
//	@Description  project's forwardedPorts list. Supports all HTTP methods and
//	@Description  WebSocket upgrade for HMR.
//	@Tags         proxy
//	@Param        projectId  path  string  true  "Project ID"
//	@Param        agentType  path  string  true  "Agent type"
//	@Param        port       path  int     true  "Container port number (1-65535)"
//	@Param        path       path  string  true  "Path to proxy"
//	@Success      200  "Proxied response from container"
//	@Failure      400  {object}  apiError  "Invalid port number"
//	@Failure      404  {object}  apiError  "Port not declared or project not found"
//	@Failure      502  {object}  apiError  "Container unreachable"
//	@Router       /api/v1/projects/{projectId}/{agentType}/proxy/{port}/{path} [get]
func (rt *routes) handleProxy(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectId")
	agentType, ok := extractAgentType(r)
	if !ok {
		writeError(w, ErrCodeInvalidBody, "invalid agent type", http.StatusBadRequest)
		return
	}

	portStr := r.PathValue("port")
	port, err := strconv.Atoi(portStr)
	if err != nil || !isValidPort(port) {
		writeError(w, ErrCodeInvalidBody, "invalid port number", http.StatusBadRequest)
		return
	}

	containerIP, err := rt.svc.ProxyPort(r.Context(), projectID, agentType, port)
	if err != nil {
		if writeServiceError(w, err) {
			return
		}
		writeError(w, ErrCodeInternal, err.Error(), http.StatusInternalServerError)
		return
	}

	cached := rt.getOrCreateProxy(projectID, agentType, port, containerIP)
	proxyPath := "/" + r.PathValue("path")

	proxy := &httputil.ReverseProxy{
		Transport: cached.transport,
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(cached.target)
			pr.Out.URL.Path = proxyPath
			pr.Out.URL.RawQuery = r.URL.RawQuery
			pr.Out.Host = cached.target.Host
		},
		ErrorHandler: func(rw http.ResponseWriter, req *http.Request, proxyErr error) {
			slog.Warn("proxy error", "project", projectID, "port", port, "err", proxyErr)
			if strings.EqualFold(req.Header.Get("Upgrade"), "websocket") {
				return
			}
			writeError(rw, ErrCodeProxyError, "container unreachable", http.StatusBadGateway)
		},
	}

	proxy.ServeHTTP(w, r)
}

// getOrCreateProxy returns a cached transport and target URL for the given
// container, creating one if the cache is empty or the container IP has
// changed. The transport is shared across requests to enable HTTP
// connection reuse to the container.
func (rt *routes) getOrCreateProxy(projectID, agentType string, port int, containerIP string) *cachedProxy {
	key := proxyKey{projectID: projectID, agentType: agentType, port: port}

	rt.proxyMu.Lock()
	defer rt.proxyMu.Unlock()

	if cached, ok := rt.proxyCache[key]; ok && cached.containerIP == containerIP {
		return cached
	}

	target, _ := url.Parse(fmt.Sprintf("http://%s:%d", containerIP, port))
	cached := &cachedProxy{
		containerIP: containerIP,
		target:      target,
		transport:   http.DefaultTransport,
	}
	rt.proxyCache[key] = cached
	return cached
}

// isValidPort reports whether p is a valid TCP/UDP port number.
func isValidPort(p int) bool {
	return p >= 1 && p <= 65535
}

// validateForwardedPorts checks that all ports are in the valid range.
// Returns a non-empty error message on failure.
func validateForwardedPorts(ports []int) string {
	for _, p := range ports {
		if !isValidPort(p) {
			return fmt.Sprintf("invalid forwarded port: %d (must be 1-65535)", p)
		}
	}
	return ""
}
