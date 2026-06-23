package api

import (
	"context"

	"github.com/qase-tms/tiden-mcp-server/internal/model"
)

type ListEnvironmentsResponse struct {
	Environments []model.Environment `json:"environments"`
	Pagination   model.Pagination    `json:"pagination"`
}

func (c *Client) ListEnvironments(ctx context.Context, productID string) (*ListEnvironmentsResponse, error) {
	var resp ListEnvironmentsResponse
	path := pathf("/v1/products/%s/environments?pageSize=100", productID)
	if err := c.Do(ctx, "GET", path, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
