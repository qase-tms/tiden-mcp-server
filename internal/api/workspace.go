package api

import (
	"context"

	"github.com/qase-tms/tiden-mcp-server/internal/model"
)

type ListWorkspacesResponse struct {
	Workspaces []model.Workspace `json:"workspaces"`
	Pagination model.Pagination  `json:"pagination"`
}

func (c *Client) ListWorkspaces(ctx context.Context) (*ListWorkspacesResponse, error) {
	var resp ListWorkspacesResponse
	if err := c.Do(ctx, "GET", "/v1/workspaces?pageSize=50", nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
