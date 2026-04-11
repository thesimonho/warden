package client

import (
	"context"

	"github.com/thesimonho/warden/api"
)

// ReportFocus reports the client's viewer focus state to the server.
func (c *Client) ReportFocus(ctx context.Context, req api.FocusRequest) error {
	return c.post(ctx, "/api/v1/focus", req, nil)
}

// GetFocusState returns the aggregated viewer focus state.
func (c *Client) GetFocusState(ctx context.Context) (*api.FocusState, error) {
	var resp api.FocusState
	if err := c.get(ctx, "/api/v1/focus", &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
