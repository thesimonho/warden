package client

import (
	"context"
	"net/url"

	"github.com/thesimonho/warden/api"
	"github.com/thesimonho/warden/runtime"
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
func (c *Client) ListRuntimes(ctx context.Context) ([]runtime.RuntimeInfo, error) {
	var runtimes []runtime.RuntimeInfo
	if err := c.get(ctx, "/api/v1/runtimes", &runtimes); err != nil {
		return nil, err
	}
	return runtimes, nil
}
