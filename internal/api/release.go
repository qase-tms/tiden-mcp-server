package api

import (
	"context"

	"github.com/qase-tms/tiden-mcp-server/internal/model"
)

type ListReleasesResponse struct {
	Releases   []model.Release  `json:"releases"`
	Pagination model.Pagination `json:"pagination"`
}

func (c *Client) ListReleases(ctx context.Context, productID string) (*ListReleasesResponse, error) {
	var resp ListReleasesResponse
	path := pathf("/v1/products/%s/releases?pageSize=100", productID)
	if err := c.Do(ctx, "GET", path, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
