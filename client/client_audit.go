package client

import (
	"context"
	"net/url"
	"strconv"

	"github.com/thesimonho/warden/api"
)

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
func (c *Client) GetAuditLog(ctx context.Context, filters api.AuditFilters) ([]api.AuditEntry, error) {
	params := auditParams(filters)
	path := "/api/v1/audit"
	if encoded := params.Encode(); encoded != "" {
		path += "?" + encoded
	}

	var entries []api.AuditEntry
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
