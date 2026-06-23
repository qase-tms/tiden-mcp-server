package api

import (
	"context"

	"github.com/qase-tms/tiden-mcp-server/internal/model"
)

type GetCurrentUserResponse struct {
	User model.User `json:"user"`
}

func (c *Client) Me(ctx context.Context) (*model.User, error) {
	var resp GetCurrentUserResponse
	if err := c.Do(ctx, "GET", "/v1/auth/me", nil, &resp); err != nil {
		return nil, err
	}
	return &resp.User, nil
}
