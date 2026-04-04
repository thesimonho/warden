package client

import (
	"context"

	"github.com/thesimonho/warden/access"
	"github.com/thesimonho/warden/api"
)

// --- Access Items ---

// ListAccessItems returns all access items (built-in + user-created)
// with host detection status.
// API: GET /api/v1/access
func (c *Client) ListAccessItems(ctx context.Context) (*api.AccessItemListResponse, error) {
	var resp api.AccessItemListResponse
	if err := c.get(ctx, "/api/v1/access", &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetAccessItem returns a single access item by ID.
// API: GET /api/v1/access/{id}
func (c *Client) GetAccessItem(ctx context.Context, id string) (*api.AccessItemResponse, error) {
	var resp api.AccessItemResponse
	if err := c.get(ctx, "/api/v1/access/"+id, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// CreateAccessItem creates a user-defined access item.
// API: POST /api/v1/access
func (c *Client) CreateAccessItem(ctx context.Context, req api.CreateAccessItemRequest) (*access.Item, error) {
	var item access.Item
	if err := c.post(ctx, "/api/v1/access", req, &item); err != nil {
		return nil, err
	}
	return &item, nil
}

// UpdateAccessItem updates a user-defined access item.
// API: PUT /api/v1/access/{id}
func (c *Client) UpdateAccessItem(ctx context.Context, id string, req api.UpdateAccessItemRequest) (*access.Item, error) {
	var item access.Item
	if err := c.put(ctx, "/api/v1/access/"+id, req, &item); err != nil {
		return nil, err
	}
	return &item, nil
}

// DeleteAccessItem removes a user-defined access item.
// API: DELETE /api/v1/access/{id}
func (c *Client) DeleteAccessItem(ctx context.Context, id string) error {
	return c.delete(ctx, "/api/v1/access/"+id)
}

// ResetAccessItem restores a built-in access item to its default.
// API: POST /api/v1/access/{id}/reset
func (c *Client) ResetAccessItem(ctx context.Context, id string) (*access.Item, error) {
	var item access.Item
	if err := c.post(ctx, "/api/v1/access/"+id+"/reset", nil, &item); err != nil {
		return nil, err
	}
	return &item, nil
}

// ResolveAccessItems resolves the given access items for preview/testing.
// API: POST /api/v1/access/resolve
func (c *Client) ResolveAccessItems(ctx context.Context, req api.ResolveAccessItemsRequest) (*api.ResolveAccessItemsResponse, error) {
	var resp api.ResolveAccessItemsResponse
	if err := c.post(ctx, "/api/v1/access/resolve", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
