package api

import (
	"context"

	"github.com/qase-tms/tiden-mcp-server/internal/model"
)

type ListComponentsResponse struct {
	Components []model.Component `json:"components"`
	Pagination model.Pagination  `json:"pagination"`
}

func (c *Client) ListComponents(ctx context.Context, productID string) (*ListComponentsResponse, error) {
	var resp ListComponentsResponse
	path := pathf("/v1/products/%s/components?pageSize=50", productID)
	if err := c.Do(ctx, "GET", path, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
