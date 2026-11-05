package api

import (
	"context"
	"errors"

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

// ErrTokenInfoUnsupported is GetTokenInfo's answer from a server without
// GET /v1/auth/token.
var ErrTokenInfoUnsupported = errors.New("server does not describe API tokens (GET /v1/auth/token)")

// GetTokenInfo describes the API token this client sends (GET
// /v1/auth/token): its kind (personal or product), scope, and for a product
// token the product and workspace it belongs to. It works for every token,
// a publish-only one included. A server that predates the endpoint answers
// 404 (or 501), returned as ErrTokenInfoUnsupported.
func (c *Client) GetTokenInfo(ctx context.Context) (*model.TokenInfo, error) {
	var resp model.TokenInfo
	if err := c.Do(ctx, "GET", "/v1/auth/token", nil, &resp); err != nil {
		if errors.Is(err, ErrNotFound) || errors.Is(err, ErrUnimplemented) {
			return nil, ErrTokenInfoUnsupported
		}
		return nil, err
	}
	return &resp, nil
}
