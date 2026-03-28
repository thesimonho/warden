// Package client provides a Go HTTP client for the Warden API.
//
// Use [New] to create a client pointing at a running warden server:
//
//	c := client.New("http://localhost:8090")
//	projects, err := c.ListProjects(ctx)
//
// This package is the Go equivalent of web/src/lib/api.ts. If you're
// building a Go application that consumes Warden over HTTP (rather than
// embedding the engine via warden.New()), this is the package to use.
//
// # Key types
//
// Methods return types from the [engine] and [service] packages:
//
//   - [engine.Project]: ID, Name, State ("running"/"exited"), NeedsInput, NotificationType, ActiveWorktreeCount, TotalCost, NetworkMode
//   - [engine.Worktree]: ID, State ("connected"/"shell"/"background"/"disconnected"), Branch, ExitCode, NotificationType
//   - [api.ContainerResult]: ContainerID, Name (output of create/update)
//   - [api.SettingsResponse]: Runtime, AuditLogMode
//
// # Error handling
//
// All non-2xx responses are returned as [*APIError] with a machine-readable
// Code field. Use [errors.As] to inspect:
//
//	var apiErr *client.APIError
//	if errors.As(err, &apiErr) {
//	    switch apiErr.Code {
//	    case "NAME_TAKEN":
//	        // handle name collision
//	    case "NOT_FOUND":
//	        // handle missing resource
//	    }
//	}
//
// # HTTP configuration
//
// The client uses a 30-second timeout for standard requests. SSE
// connections ([SubscribeEvents]) use no timeout. All POST/PUT requests
// send Content-Type: application/json.
package client

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/coder/websocket"

	"github.com/thesimonho/warden/api"
	"github.com/thesimonho/warden/db"
	"github.com/thesimonho/warden/engine"
	"github.com/thesimonho/warden/eventbus"
	"github.com/thesimonho/warden/runtime"
)

// Client talks to a running Warden server over HTTP.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// New creates a Client pointing at the given Warden server URL
// (e.g. "http://localhost:8090").
func New(baseURL string) *Client {
	return &Client{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// --- Projects ---

// ListProjects returns all configured projects with container state, cost,
// and attention data. Each [engine.Project] includes State ("running",
// "exited", "not-found"), NeedsInput (true when Claude needs attention),
// NotificationType ("permission_prompt", "idle_prompt", "elicitation_dialog"),
// ActiveWorktreeCount, TotalCost (USD), and NetworkMode.
func (c *Client) ListProjects(ctx context.Context) ([]engine.Project, error) {
	var projects []engine.Project
	if err := c.get(ctx, "/api/v1/projects", &projects); err != nil {
		return nil, err
	}
	return projects, nil
}

// AddProject registers a project directory in Warden.
func (c *Client) AddProject(ctx context.Context, name, hostPath string) (*api.ProjectResult, error) {
	var resp api.ProjectResult
	body := map[string]string{"name": name, "projectPath": hostPath}
	if err := c.post(ctx, "/api/v1/projects", body, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// RemoveProject removes a project from the database by project ID.
func (c *Client) RemoveProject(ctx context.Context, projectID string) (*api.ProjectResult, error) {
	var resp api.ProjectResult
	if err := c.deleteWithBody(ctx, "/api/v1/projects/"+url.PathEscape(projectID), &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// StopProject stops the container with the given ID.
func (c *Client) StopProject(ctx context.Context, id string) (*api.ProjectResult, error) {
	var resp api.ProjectResult
	if err := c.post(ctx, "/api/v1/projects/"+id+"/stop", nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// RestartProject restarts the container with the given ID.
func (c *Client) RestartProject(ctx context.Context, id string) (*api.ProjectResult, error) {
	var resp api.ProjectResult
	if err := c.post(ctx, "/api/v1/projects/"+id+"/restart", nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// --- Worktrees ---

// ListWorktrees returns all worktrees for the given container.
func (c *Client) ListWorktrees(ctx context.Context, projectID string) ([]engine.Worktree, error) {
	var worktrees []engine.Worktree
	if err := c.get(ctx, "/api/v1/projects/"+projectID+"/worktrees", &worktrees); err != nil {
		return nil, err
	}
	return worktrees, nil
}

// CreateWorktree creates a new git worktree and connects a terminal.
func (c *Client) CreateWorktree(ctx context.Context, projectID, name string) (*api.WorktreeResult, error) {
	var resp api.WorktreeResult
	if err := c.post(ctx, "/api/v1/projects/"+projectID+"/worktrees", map[string]string{"name": name}, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// ConnectTerminal starts or reconnects the terminal process (abduco session)
// for a worktree. Must be called before [AttachTerminal] to ensure the
// process exists. The terminal process continues running in the background
// even if no viewer is attached, allowing Claude Code to work independently.
//
// If the terminal is already running, this is a no-op that returns success.
func (c *Client) ConnectTerminal(ctx context.Context, projectID, worktreeID string) (*api.WorktreeResult, error) {
	var resp api.WorktreeResult
	if err := c.post(ctx, "/api/v1/projects/"+projectID+"/worktrees/"+worktreeID+"/connect", nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// DisconnectTerminal closes the terminal viewer for a worktree.
func (c *Client) DisconnectTerminal(ctx context.Context, projectID, worktreeID string) (*api.WorktreeResult, error) {
	var resp api.WorktreeResult
	if err := c.post(ctx, "/api/v1/projects/"+projectID+"/worktrees/"+worktreeID+"/disconnect", nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// KillWorktreeProcess kills the terminal process for a worktree.
func (c *Client) KillWorktreeProcess(ctx context.Context, projectID, worktreeID string) (*api.WorktreeResult, error) {
	var resp api.WorktreeResult
	if err := c.post(ctx, "/api/v1/projects/"+projectID+"/worktrees/"+worktreeID+"/kill", nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// RemoveWorktree fully removes a worktree.
func (c *Client) RemoveWorktree(ctx context.Context, projectID, worktreeID string) (*api.WorktreeResult, error) {
	var resp api.WorktreeResult
	if err := c.deleteWithBody(ctx, "/api/v1/projects/"+projectID+"/worktrees/"+worktreeID, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// CleanupWorktrees removes orphaned worktree directories.
func (c *Client) CleanupWorktrees(ctx context.Context, projectID string) ([]string, error) {
	var resp struct {
		Removed []string `json:"removed"`
	}
	if err := c.post(ctx, "/api/v1/projects/"+projectID+"/worktrees/cleanup", nil, &resp); err != nil {
		return nil, err
	}
	return resp.Removed, nil
}

// GetWorktreeDiff returns uncommitted changes for a worktree.
func (c *Client) GetWorktreeDiff(ctx context.Context, projectID, worktreeID string) (*api.DiffResponse, error) {
	var resp api.DiffResponse
	if err := c.get(ctx, "/api/v1/projects/"+projectID+"/worktrees/"+worktreeID+"/diff", &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// --- Containers ---

// CreateContainer creates a new container for the given project.
func (c *Client) CreateContainer(ctx context.Context, projectID string, req engine.CreateContainerRequest) (*api.ContainerResult, error) {
	var resp api.ContainerResult
	if err := c.post(ctx, "/api/v1/projects/"+projectID+"/container", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// ResetProjectCosts removes all cost history for a project.
func (c *Client) ResetProjectCosts(ctx context.Context, projectID string) error {
	return c.delete(ctx, "/api/v1/projects/"+projectID+"/costs")
}

// PurgeProjectAudit removes all audit events for a project.
func (c *Client) PurgeProjectAudit(ctx context.Context, projectID string) error {
	return c.delete(ctx, "/api/v1/projects/"+projectID+"/audit")
}

// DeleteContainer stops and removes the container for the given project.
func (c *Client) DeleteContainer(ctx context.Context, projectID string) (*api.ContainerResult, error) {
	var resp api.ContainerResult
	if err := c.deleteWithBody(ctx, "/api/v1/projects/"+projectID+"/container", &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// InspectContainer returns the configuration of the project's container.
func (c *Client) InspectContainer(ctx context.Context, projectID string) (*engine.ContainerConfig, error) {
	var cfg engine.ContainerConfig
	if err := c.get(ctx, "/api/v1/projects/"+projectID+"/container/config", &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// UpdateContainer recreates the project's container with updated configuration.
func (c *Client) UpdateContainer(ctx context.Context, projectID string, req engine.CreateContainerRequest) (*api.ContainerResult, error) {
	var resp api.ContainerResult
	if err := c.put(ctx, "/api/v1/projects/"+projectID+"/container", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// ValidateContainer checks whether the project's container has Warden infrastructure.
func (c *Client) ValidateContainer(ctx context.Context, projectID string) (*api.ValidateContainerResult, error) {
	var resp api.ValidateContainerResult
	if err := c.get(ctx, "/api/v1/projects/"+projectID+"/container/validate", &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// --- Settings ---

// GetSettings returns the server-side settings.
func (c *Client) GetSettings(ctx context.Context) (*api.SettingsResponse, error) {
	var resp api.SettingsResponse
	if err := c.get(ctx, "/api/v1/settings", &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// UpdateSettings updates server-side settings.
func (c *Client) UpdateSettings(ctx context.Context, req api.UpdateSettingsRequest) (*api.UpdateSettingsResult, error) {
	var resp api.UpdateSettingsResult
	if err := c.put(ctx, "/api/v1/settings", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// --- Host Utilities ---

// GetDefaults returns server-resolved defaults for the create container form.
func (c *Client) GetDefaults(ctx context.Context) (*api.DefaultsResponse, error) {
	var resp api.DefaultsResponse
	if err := c.get(ctx, "/api/v1/defaults", &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// ListDirectories returns filesystem entries at a path for the browser.
// When includeFiles is true, files are returned alongside directories.
func (c *Client) ListDirectories(ctx context.Context, path string, includeFiles bool) ([]api.DirEntry, error) {
	q := "/api/v1/filesystem/directories?path=" + url.QueryEscape(path)
	if includeFiles {
		q += "&mode=file"
	}
	var entries []api.DirEntry
	if err := c.get(ctx, q, &entries); err != nil {
		return nil, err
	}
	return entries, nil
}

// ListRuntimes returns available container runtimes.
func (c *Client) ListRuntimes(ctx context.Context) ([]runtime.RuntimeInfo, error) {
	var runtimes []runtime.RuntimeInfo
	if err := c.get(ctx, "/api/v1/runtimes", &runtimes); err != nil {
		return nil, err
	}
	return runtimes, nil
}

// --- Audit Log ---

// auditParams converts audit filter fields to url.Values.
func auditParams(filters api.AuditFilters) url.Values {
	params := url.Values{}
	if filters.ProjectID != "" {
		params.Set("project_id", filters.ProjectID)
	}
	if filters.Worktree != "" {
		params.Set("worktree", filters.Worktree)
	}
	if filters.Source != "" {
		params.Set("source", filters.Source)
	}
	if filters.Level != "" {
		params.Set("level", filters.Level)
	}
	if filters.Since != "" {
		params.Set("since", filters.Since)
	}
	if filters.Until != "" {
		params.Set("until", filters.Until)
	}
	if string(filters.Category) != "" {
		params.Set("category", string(filters.Category))
	}
	if filters.Limit > 0 {
		params.Set("limit", strconv.Itoa(filters.Limit))
	}
	if filters.Offset > 0 {
		params.Set("offset", strconv.Itoa(filters.Offset))
	}
	return params
}

// GetAuditLog returns filtered audit events.
func (c *Client) GetAuditLog(ctx context.Context, filters api.AuditFilters) ([]db.Entry, error) {
	params := auditParams(filters)
	path := "/api/v1/audit"
	if encoded := params.Encode(); encoded != "" {
		path += "?" + encoded
	}

	var entries []db.Entry
	if err := c.get(ctx, path, &entries); err != nil {
		return nil, err
	}
	return entries, nil
}

// GetAuditSummary returns aggregate audit statistics.
func (c *Client) GetAuditSummary(ctx context.Context, filters api.AuditFilters) (*api.AuditSummary, error) {
	params := auditParams(filters)
	path := "/api/v1/audit/summary"
	if encoded := params.Encode(); encoded != "" {
		path += "?" + encoded
	}

	var summary api.AuditSummary
	if err := c.get(ctx, path, &summary); err != nil {
		return nil, err
	}
	return &summary, nil
}

// GetAuditProjects returns distinct project (container) names from the audit log.
func (c *Client) GetAuditProjects(ctx context.Context) ([]string, error) {
	var projects []string
	if err := c.get(ctx, "/api/v1/audit/projects", &projects); err != nil {
		return nil, err
	}
	return projects, nil
}

// PostAuditEvent writes a frontend event to the audit log.
func (c *Client) PostAuditEvent(ctx context.Context, req api.PostAuditEventRequest) error {
	return c.post(ctx, "/api/v1/audit", req, nil)
}

// DeleteAuditEvents removes events matching the given filters.
func (c *Client) DeleteAuditEvents(ctx context.Context, filters api.AuditFilters) error {
	params := auditParams(filters)
	path := "/api/v1/audit"
	if encoded := params.Encode(); encoded != "" {
		path += "?" + encoded
	}
	return c.delete(ctx, path)
}

// --- Real-time Events ---

// SubscribeEvents opens a Server-Sent Events connection and returns a
// channel of parsed events. The channel closes when the context is
// cancelled or the connection drops. Call the returned function to
// unsubscribe and clean up the connection.
//
// Event types sent on the channel:
//   - "worktree_state": worktree attention/session state changed (Data contains containerName, worktreeId, state fields)
//   - "project_state": project cost updated (Data contains containerName, totalCost)
//   - "worktree_list_changed": worktrees were created/removed (Data contains containerName)
//   - "heartbeat": periodic keepalive (Data is empty object)
//
// Each event's Data field is a json.RawMessage — unmarshal it based on
// the Event type.
//
// Example:
//
//	ch, unsub, err := c.SubscribeEvents(ctx)
//	if err != nil { return err }
//	defer unsub()
//
//	for event := range ch {
//	    switch event.Event {
//	    case eventbus.SSEWorktreeState:
//	        // A worktree's state changed — refresh worktree list
//	    case eventbus.SSEProjectState:
//	        // Cost updated — refresh project list
//	    }
//	}
func (c *Client) SubscribeEvents(ctx context.Context) (<-chan eventbus.SSEEvent, func(), error) {
	ctx, cancel := context.WithCancel(ctx)
	ch := make(chan eventbus.SSEEvent, 64)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/v1/events", nil)
	if err != nil {
		cancel()
		return nil, nil, err
	}
	req.Header.Set("Accept", "text/event-stream")

	// Use a client without timeout for the long-lived SSE connection.
	sseClient := &http.Client{}
	resp, err := sseClient.Do(req)
	if err != nil {
		cancel()
		return nil, nil, fmt.Errorf("SSE connect: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		cancel()
		return nil, nil, fmt.Errorf("SSE connect: HTTP %d", resp.StatusCode)
	}

	go readSSEStream(ctx, resp.Body, ch)

	unsub := func() {
		cancel()
		_ = resp.Body.Close()
	}
	return ch, unsub, nil
}

// readSSEStream parses an SSE text stream and sends events to the channel.
// Closes the channel when the stream ends or the context is cancelled.
func readSSEStream(ctx context.Context, body io.ReadCloser, ch chan<- eventbus.SSEEvent) {
	defer close(ch)
	defer body.Close() //nolint:errcheck

	scanner := bufio.NewScanner(body)
	var eventType string
	var dataLines []byte

	for scanner.Scan() {
		if ctx.Err() != nil {
			return
		}

		line := scanner.Text()

		switch {
		case strings.HasPrefix(line, "event:"):
			eventType = strings.TrimSpace(strings.TrimPrefix(line, "event:"))

		case strings.HasPrefix(line, "data:"):
			data := strings.TrimPrefix(line, "data:")
			data = strings.TrimSpace(data)
			dataLines = []byte(data)

		case line == "":
			// Empty line = end of event.
			if eventType != "" && dataLines != nil {
				event := eventbus.SSEEvent{
					Event: eventbus.SSEEventType(eventType),
					Data:  json.RawMessage(dataLines),
				}
				select {
				case ch <- event:
				case <-ctx.Done():
					return
				}
			}
			eventType = ""
			dataLines = nil
		}
	}
}

// --- Terminal Attachment ---

// AttachTerminal opens a WebSocket connection to a worktree's terminal
// and returns a [TerminalConnection] for bidirectional PTY I/O.
//
// Prerequisites: [ConnectTerminal] must be called first to ensure the
// terminal process (abduco session) is running. AttachTerminal creates
// a viewer into the existing session.
//
// The terminal lifecycle:
//  1. [ConnectTerminal] — start the abduco session (idempotent)
//  2. AttachTerminal — open a viewer connection
//  3. Read/Write on the [TerminalConnection] — PTY I/O
//  4. Close the connection (or the user presses the disconnect key)
//  5. [DisconnectTerminal] — notify the server the viewer closed
//
// The abduco session survives viewer disconnects. Call
// [KillWorktreeProcess] to fully terminate the session.
func (c *Client) AttachTerminal(ctx context.Context, projectID, worktreeID string) (TerminalConnection, error) {
	wsURL := strings.Replace(c.baseURL, "http://", "ws://", 1)
	wsURL = strings.Replace(wsURL, "https://", "wss://", 1)
	wsURL += "/api/v1/projects/" + projectID + "/ws/" + worktreeID

	conn, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{
		CompressionMode: websocket.CompressionDisabled,
	})
	if err != nil {
		return nil, fmt.Errorf("websocket dial: %w", err)
	}

	conn.SetReadLimit(128 * 1024) // 128KB, matches proxy.go

	return &wsTerminalConn{
		conn: conn,
		ctx:  ctx,
	}, nil
}

// wsTerminalConn wraps a WebSocket connection as a TerminalConnection.
type wsTerminalConn struct {
	conn *websocket.Conn
	ctx  context.Context
}

// Read reads PTY output from the WebSocket (binary frames).
func (w *wsTerminalConn) Read(p []byte) (int, error) {
	_, data, err := w.conn.Read(w.ctx)
	if err != nil {
		return 0, err
	}
	n := copy(p, data)
	return n, nil
}

// Write sends keyboard input via the WebSocket (binary frames).
func (w *wsTerminalConn) Write(p []byte) (int, error) {
	err := w.conn.Write(w.ctx, websocket.MessageBinary, p)
	if err != nil {
		return 0, err
	}
	return len(p), nil
}

// Close closes the WebSocket connection.
func (w *wsTerminalConn) Close() error {
	return w.conn.Close(websocket.StatusNormalClosure, "")
}

// Resize sends a JSON resize command via the WebSocket (text frame).
func (w *wsTerminalConn) Resize(cols, rows uint) error {
	msg, err := json.Marshal(map[string]any{
		"type": "resize",
		"cols": cols,
		"rows": rows,
	})
	if err != nil {
		return err
	}
	return w.conn.Write(w.ctx, websocket.MessageText, msg)
}

// TerminalConnection provides raw bidirectional I/O to a container terminal.
// Read returns PTY output, Write sends keyboard input, and Resize updates
// the remote terminal dimensions.
//
// In HTTP mode, this wraps a WebSocket connection (binary frames for I/O,
// text frames for resize commands). In embedded mode, this wraps a docker
// exec session attached to the abduco viewer.
//
// Example:
//
//	conn, err := c.AttachTerminal(ctx, projectID, worktreeID)
//	if err != nil { return err }
//	defer conn.Close()
//
//	conn.Resize(80, 24) // set initial terminal size
//	conn.Write([]byte("ls\n"))
//
//	buf := make([]byte, 4096)
//	n, _ := conn.Read(buf)
//	fmt.Print(string(buf[:n]))
type TerminalConnection interface {
	io.ReadWriteCloser

	// Resize changes the terminal dimensions of the remote PTY.
	Resize(cols, rows uint) error
}

// --- HTTP helpers ---

// APIError represents a non-2xx response from the Warden API.
// Use [errors.As] to extract it from returned errors. Match on the Code
// field for programmatic handling instead of parsing Message.
//
// Common error codes:
//   - "NOT_FOUND": resource (project, worktree, container) does not exist
//   - "NAME_TAKEN": container or project name is already in use
//   - "INVALID_BODY": malformed request body
//   - "NOT_CONFIGURED": Docker/Podman runtime not available
//   - "INTERNAL": unexpected server error
//
// Code may be empty for non-JSON error responses. See the integration
// guide for the full list of error codes.
type APIError struct {
	// StatusCode is the HTTP status code (400, 404, 409, 500, 503).
	StatusCode int
	// Code is a machine-readable error identifier (e.g. "NOT_FOUND").
	// May be empty for non-JSON responses.
	Code string
	// Message is a human-readable error description.
	Message string
}

func (e *APIError) Error() string {
	if e.Code != "" {
		return fmt.Sprintf("warden API error %d [%s]: %s", e.StatusCode, e.Code, e.Message)
	}
	return fmt.Sprintf("warden API error %d: %s", e.StatusCode, e.Message)
}

func (c *Client) get(ctx context.Context, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return err
	}
	return c.do(req, out)
}

func (c *Client) post(ctx context.Context, path string, body any, out any) error {
	return c.doWithBody(ctx, http.MethodPost, path, body, out)
}

func (c *Client) put(ctx context.Context, path string, body any, out any) error {
	return c.doWithBody(ctx, http.MethodPut, path, body, out)
}

func (c *Client) delete(ctx context.Context, path string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.baseURL+path, nil)
	if err != nil {
		return err
	}
	return c.do(req, nil)
}

func (c *Client) deleteWithBody(ctx context.Context, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.baseURL+path, nil)
	if err != nil {
		return err
	}
	return c.do(req, out)
}

func (c *Client) doWithBody(ctx context.Context, method, path string, body any, out any) error {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bodyReader)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return c.do(req, out)
}

func (c *Client) do(req *http.Request, out any) error {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode >= 400 {
		// Read the full body (capped at 4KB) so the TCP connection can be
		// returned to the pool even if the response isn't valid JSON.
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		// Mirrors the apiError struct in internal/server/errors.go.
		// If the server error shape changes, update this too.
		var errBody struct {
			Error string `json:"error"`
			Code  string `json:"code"`
		}
		if json.Unmarshal(body, &errBody) == nil && errBody.Error != "" {
			return &APIError{StatusCode: resp.StatusCode, Code: errBody.Code, Message: errBody.Error}
		}
		return &APIError{StatusCode: resp.StatusCode, Message: resp.Status}
	}

	if out != nil && resp.StatusCode != http.StatusNoContent {
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return nil
}
