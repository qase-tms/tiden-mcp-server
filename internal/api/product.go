package api

import (
	"context"

	"github.com/qase-tms/tiden-mcp-server/internal/model"
)

type ListProductsResponse struct {
	Products   []model.Product  `json:"products"`
	Pagination model.Pagination `json:"pagination"`
}

func (c *Client) ListProducts(ctx context.Context, workspaceID string) (*ListProductsResponse, error) {
	var resp ListProductsResponse
	path := pathf("/v1/workspaces/%s/products?pageSize=50", workspaceID)
	if err := c.Do(ctx, "GET", path, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
