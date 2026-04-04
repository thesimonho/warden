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
//   - [engine.Worktree]: ID, State ("connected"/"shell"/"background"/"stopped"), Branch, ExitCode, NotificationType
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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
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

// --- Error Types ---

// APIError represents a non-2xx response from the Warden API.
// Use [errors.As] to extract it from returned errors. Match on the Code
// field for programmatic handling instead of parsing Message.
//
// Common error codes:
//   - "NOT_FOUND": resource (project, worktree, container) does not exist
//   - "NAME_TAKEN": container or project name is already in use
//   - "INVALID_BODY": malformed request body
//   - "NOT_CONFIGURED": Docker runtime not available
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

// --- HTTP Helpers ---

// projectPath builds the base URL path for a project+agent pair.
func projectPath(projectID, agentType string) string {
	return "/api/v1/projects/" + url.PathEscape(projectID) + "/" + url.PathEscape(agentType)
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
