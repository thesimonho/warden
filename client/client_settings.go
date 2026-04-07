package client

import (
	"context"
	"encoding/json"
	"net/url"

	"github.com/thesimonho/warden/api"
	"github.com/thesimonho/warden/docker"
)

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

// --- Server Lifecycle ---

// Shutdown requests a graceful server shutdown. The server sends back
// a response before initiating the shutdown sequence.
func (c *Client) Shutdown(ctx context.Context) error {
	return c.post(ctx, "/api/v1/shutdown", nil, nil)
}

// --- Host Utilities ---

// GetDefaults returns server-resolved defaults for the create container form.
// When projectPath is non-empty, runtime detection scans that directory.
func (c *Client) GetDefaults(ctx context.Context, projectPath string) (*api.DefaultsResponse, error) {
	path := "/api/v1/defaults"
	if projectPath != "" {
		path += "?path=" + url.QueryEscape(projectPath)
	}
	var resp api.DefaultsResponse
	if err := c.get(ctx, path, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// ReadProjectTemplate reads a .warden.json project template from an
// arbitrary file path. Used by the import feature.
func (c *Client) ReadProjectTemplate(ctx context.Context, filePath string) (*api.ProjectTemplate, error) {
	path := "/api/v1/template?path=" + url.QueryEscape(filePath)
	var resp api.ProjectTemplate
	if err := c.get(ctx, path, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// ValidateProjectTemplate sends a raw .warden.json body to the server for
// validation and sanitization. Returns the cleaned template.
func (c *Client) ValidateProjectTemplate(ctx context.Context, data []byte) (*api.ProjectTemplate, error) {
	var resp api.ProjectTemplate
	if err := c.post(ctx, "/api/v1/template", json.RawMessage(data), &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// ListDirectories returns filesystem entries at a path for the browser.
// When includeFiles is true, files are returned alongside directories.
func (c *Client) ListDirectories(ctx context.Context, path string, includeFiles bool) ([]api.DirEntry, error) {
	params := url.Values{}
	params.Set("path", path)
	if includeFiles {
		params.Set("mode", "file")
	}
	var entries []api.DirEntry
	if err := c.get(ctx, "/api/v1/filesystem/directories?"+params.Encode(), &entries); err != nil {
		return nil, err
	}
	return entries, nil
}

// ListRuntimes returns available container runtimes.
func (c *Client) ListRuntimes(ctx context.Context) (*docker.Info, error) {
	var info docker.Info
	if err := c.get(ctx, "/api/v1/runtimes", &info); err != nil {
		return nil, err
	}
	return &info, nil
}
