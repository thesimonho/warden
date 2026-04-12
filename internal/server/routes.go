package server

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"regexp"
	"sync"

	"github.com/thesimonho/warden/api"
	"github.com/thesimonho/warden/constants"
	"github.com/thesimonho/warden/engine"
	"github.com/thesimonho/warden/eventbus"
	"github.com/thesimonho/warden/internal/terminal"
	"github.com/thesimonho/warden/service"
)

// containerIDRegex validates Docker short (12-char) or full (64-char) hex container IDs.
var containerIDRegex = regexp.MustCompile(`^[a-f0-9]{12,64}$`)

// domainRegex validates domain names (e.g. "api.anthropic.com", "*.github.com").
var domainRegex = regexp.MustCompile(`^(\*\.)?[a-zA-Z0-9]([a-zA-Z0-9\-]*[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9\-]*[a-zA-Z0-9])?)*$`)

// containerNameRegex validates Docker container names (alphanumeric, hyphens, underscores, dots).
var containerNameRegex = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.\-]*$`)

// viewerKey identifies a worktree's WebSocket viewer connections.
type viewerKey struct {
	containerID string
	worktreeID  string
}

// routes holds dependencies and provides HTTP handler methods.
type routes struct {
	svc           *service.Service
	broker        *eventbus.Broker
	terminalProxy *terminal.Proxy

	// viewerMu guards viewerCounts for concurrent WebSocket connect/disconnect.
	viewerMu     sync.Mutex
	viewerCounts map[viewerKey]int
}

// registerAPIRoutes attaches all API endpoint handlers to the given mux.
func registerAPIRoutes(mux *http.ServeMux, svc *service.Service, broker *eventbus.Broker, termProxy *terminal.Proxy, shutdownCh chan<- struct{}) {
	rt := &routes{
		svc: svc, broker: broker, terminalProxy: termProxy,
		viewerCounts: make(map[viewerKey]int),
	}

	mux.HandleFunc("GET /api/v1/health", handleHealth)
	mux.HandleFunc("POST /api/v1/shutdown", makeHandleShutdown(shutdownCh, broker))
	mux.HandleFunc("GET /api/v1/projects", rt.handleListProjects)
	mux.HandleFunc("POST /api/v1/projects", rt.handleAddProject)
	mux.HandleFunc("POST /api/v1/projects/batch", rt.handleBatchProjects)
	mux.HandleFunc("GET /api/v1/projects/{projectId}/{agentType}", rt.handleGetProject)
	mux.HandleFunc("DELETE /api/v1/projects/{projectId}/{agentType}", rt.handleRemoveProject)
	mux.HandleFunc("POST /api/v1/projects/{projectId}/{agentType}/stop", rt.handleStopProject)
	mux.HandleFunc("POST /api/v1/projects/{projectId}/{agentType}/restart", rt.handleRestartProject)
	mux.HandleFunc("GET /api/v1/projects/{projectId}/{agentType}/costs", rt.handleGetProjectCosts)
	mux.HandleFunc("GET /api/v1/projects/{projectId}/{agentType}/budget", rt.handleGetBudgetStatus)
	mux.HandleFunc("GET /api/v1/projects/{projectId}/{agentType}/worktrees", rt.handleListWorktrees)
	mux.HandleFunc("POST /api/v1/projects/{projectId}/{agentType}/worktrees", rt.handleCreateWorktree)
	mux.HandleFunc("GET /api/v1/projects/{projectId}/{agentType}/worktrees/{wid}", rt.handleGetWorktree)
	mux.HandleFunc("POST /api/v1/projects/{projectId}/{agentType}/worktrees/{wid}/input", rt.handleSendWorktreeInput)
	mux.HandleFunc("POST /api/v1/projects/{projectId}/{agentType}/worktrees/{wid}/connect", rt.handleConnectTerminal)
	mux.HandleFunc("POST /api/v1/projects/{projectId}/{agentType}/worktrees/{wid}/disconnect", rt.handleDisconnectTerminal)
	mux.HandleFunc("POST /api/v1/projects/{projectId}/{agentType}/worktrees/{wid}/kill", rt.handleKillWorktreeProcess)
	mux.HandleFunc("POST /api/v1/projects/{projectId}/{agentType}/worktrees/{wid}/reset", rt.handleResetWorktree)
	mux.HandleFunc("DELETE /api/v1/projects/{projectId}/{agentType}/worktrees/{wid}", rt.handleRemoveWorktree)
	mux.HandleFunc("GET /api/v1/projects/{projectId}/{agentType}/worktrees/{wid}/diff", rt.handleGetWorktreeDiff)
	mux.HandleFunc("POST /api/v1/projects/{projectId}/{agentType}/worktrees/cleanup", rt.handleCleanupWorktrees)
	mux.HandleFunc("DELETE /api/v1/projects/{projectId}/{agentType}/costs", rt.handleResetProjectCosts)
	mux.HandleFunc("DELETE /api/v1/projects/{projectId}/{agentType}/audit", rt.handlePurgeProjectAudit)
	mux.HandleFunc("POST /api/v1/containers/check-name", rt.handleCheckContainerName)
	mux.HandleFunc("POST /api/v1/projects/{projectId}/{agentType}/container", rt.handleCreateContainer)
	mux.HandleFunc("DELETE /api/v1/projects/{projectId}/{agentType}/container", rt.handleDeleteContainer)
	mux.HandleFunc("GET /api/v1/projects/{projectId}/{agentType}/container/config", rt.handleInspectContainer)
	mux.HandleFunc("PUT /api/v1/projects/{projectId}/{agentType}/container", rt.handleUpdateContainer)
	mux.HandleFunc("GET /api/v1/projects/{projectId}/{agentType}/container/validate", rt.handleValidateContainer)
	mux.HandleFunc("GET /api/v1/runtimes", rt.handleListRuntimes)
	mux.HandleFunc("GET /api/v1/settings", rt.handleGetSettings)
	mux.HandleFunc("PUT /api/v1/settings", rt.handleUpdateSettings)
	mux.HandleFunc("GET /api/v1/audit", rt.handleGetAuditLog)
	mux.HandleFunc("GET /api/v1/audit/summary", rt.handleGetAuditSummary)
	mux.HandleFunc("GET /api/v1/audit/export", rt.handleExportAuditLog)
	mux.HandleFunc("GET /api/v1/audit/projects", rt.handleGetAuditProjects)
	mux.HandleFunc("POST /api/v1/audit", rt.handlePostAuditEvent)
	mux.HandleFunc("DELETE /api/v1/audit", rt.handleDeleteAuditEvents)
	mux.HandleFunc("GET /api/v1/filesystem/directories", rt.handleListDirectories)
	mux.HandleFunc("POST /api/v1/filesystem/reveal", rt.handleRevealInFileManager)
	mux.HandleFunc("GET /api/v1/access", rt.handleListAccessItems)
	mux.HandleFunc("POST /api/v1/access", rt.handleCreateAccessItem)
	mux.HandleFunc("GET /api/v1/access/{id}", rt.handleGetAccessItem)
	mux.HandleFunc("PUT /api/v1/access/{id}", rt.handleUpdateAccessItem)
	mux.HandleFunc("DELETE /api/v1/access/{id}", rt.handleDeleteAccessItem)
	mux.HandleFunc("POST /api/v1/access/{id}/reset", rt.handleResetAccessItem)
	mux.HandleFunc("POST /api/v1/access/resolve", rt.handleResolveAccessItems)
	mux.HandleFunc("GET /api/v1/defaults", rt.handleDefaults)
	mux.HandleFunc("GET /api/v1/template", rt.handleReadTemplate)
	mux.HandleFunc("POST /api/v1/template", rt.handleValidateTemplate)
	mux.HandleFunc("POST /api/v1/focus", rt.handleReportFocus)
	mux.HandleFunc("GET /api/v1/focus", rt.handleGetFocusState)
	mux.HandleFunc("GET /api/v1/events", rt.handleSSE)
	mux.HandleFunc("POST /api/v1/projects/{projectId}/{agentType}/clipboard", rt.handleUploadClipboard)
	mux.HandleFunc("GET /api/v1/projects/{projectId}/{agentType}/ws/{wid}", rt.handleTerminalWS)
	mux.HandleFunc("GET /api/v1/projects/{projectId}/{agentType}/ws/{wid}/shell", rt.handleShellTerminalWS)
}

// --- Helpers ---

// writeServiceError maps common service-layer errors to HTTP responses.
// Returns true if it wrote an error response, false if the error was not recognized.
func writeServiceError(w http.ResponseWriter, err error) bool {
	if errors.Is(err, service.ErrNotFound) {
		writeError(w, ErrCodeNotFound, "project not found", http.StatusNotFound)
		return true
	}
	if errors.Is(err, service.ErrInvalidInput) {
		writeError(w, ErrCodeInvalidBody, err.Error(), http.StatusBadRequest)
		return true
	}
	if errors.Is(err, service.ErrBudgetExceeded) {
		writeError(w, ErrCodeBudgetExceeded, err.Error(), http.StatusForbidden)
		return true
	}
	if engine.IsStaleMountsError(err) {
		writeError(w, ErrCodeStaleMounts, err.Error(), http.StatusConflict)
		return true
	}
	return false
}

// extractAgentType reads and validates the "agentType" path parameter.
// Returns the agent type and true if valid, or empty string and false
// if the value is not a recognized agent type.
func extractAgentType(r *http.Request) (string, bool) {
	at := constants.AgentType(r.PathValue("agentType"))
	if at.Valid() {
		return string(at), true
	}
	return "", false
}

// --- Input Validation ---

// isValidContainerID checks that an ID is a valid Docker container hex ID.
func isValidContainerID(id string) bool {
	return containerIDRegex.MatchString(id)
}

// isValidWorktreeID checks that a worktree ID matches the expected pattern.
func isValidWorktreeID(id string) bool {
	return engine.IsValidWorktreeID(id)
}

// isValidContainerName checks that a name matches Docker's container naming rules.
func isValidContainerName(name string) bool {
	return containerNameRegex.MatchString(name)
}

// isValidDomain checks that a domain string is a valid hostname.
func isValidDomain(domain string) bool {
	return len(domain) > 0 && len(domain) <= 253 && domainRegex.MatchString(domain)
}

// validateNetworkConfig checks network-related fields of a container
// request. Returns a non-empty error message on failure.
func validateNetworkConfig(req api.CreateContainerRequest) string {
	if req.NetworkMode != "" && !api.IsValidNetworkMode(string(req.NetworkMode)) {
		return "invalid network mode: must be full, restricted, or none"
	}
	if req.NetworkMode == api.NetworkModeRestricted && len(req.AllowedDomains) == 0 {
		return "restricted network mode requires at least one allowed domain"
	}
	if len(req.AllowedDomains) > 0 && !validateAllowedDomains(req.AllowedDomains) {
		return "invalid domain in allowedDomains"
	}
	return ""
}

// validateProjectSource checks that exactly one of projectPath or cloneURL is
// provided. Returns a non-empty error message on failure.
func validateProjectSource(projectPath, cloneURL string) string {
	if projectPath == "" && cloneURL == "" {
		return "projectPath or cloneURL is required"
	}
	if projectPath != "" && cloneURL != "" {
		return "provide only one of projectPath or cloneURL"
	}
	return ""
}

// validateAllowedDomains checks that all domain strings in the list are valid.
func validateAllowedDomains(domains []string) bool {
	for _, d := range domains {
		if !isValidDomain(d) {
			return false
		}
	}
	return true
}

// --- JSON Response Helpers ---

// writeJSONCreated marshals v to JSON and writes it with a 201 Created status.
func writeJSONCreated(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("json encode failed", "err", err)
	}
}

// writeJSON marshals v to JSON and writes it to the response.
func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("json encode failed", "err", err)
	}
}
