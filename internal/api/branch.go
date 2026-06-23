package api

import (
	"context"
)

type Branch struct {
	ID          string `json:"id"`
	ProductID   string `json:"productId"`
	Name        string `json:"name"`
	Description string `json:"description"`
	CreatedBy   string `json:"createdBy"`
	Status      string `json:"status"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
}

type ListBranchesResponse struct {
	Branches []Branch `json:"branches"`
}

type CreateBranchRequest struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

type CreateBranchResponse struct {
	Branch Branch `json:"branch"`
}

type MergePreviewResponse struct {
	Additions     []any       `json:"additions"`
	Modifications []any       `json:"modifications"`
	Deletions     []any       `json:"deletions"`
	Stats         *MergeStats `json:"stats"`
}

type MergeStats struct {
	Additions     int `json:"additions"`
	Modifications int `json:"modifications"`
	Deletions     int `json:"deletions"`
	Conflicts     int `json:"conflicts"`
}

func (c *Client) ListBranches(ctx context.Context, productID string) (*ListBranchesResponse, error) {
	var resp ListBranchesResponse
	path := pathf("/v1/products/%s/branches", productID)
	if err := c.Do(ctx, "GET", path, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) CreateBranch(ctx context.Context, productID, name, description string) (*Branch, error) {
	body := CreateBranchRequest{Name: name, Description: description}
	var resp CreateBranchResponse
	path := pathf("/v1/products/%s/branches", productID)
	if err := c.Do(ctx, "POST", path, body, &resp); err != nil {
		return nil, err
	}
	return &resp.Branch, nil
}

func (c *Client) GetMergePreview(ctx context.Context, branchID string) (*MergePreviewResponse, error) {
	var resp MergePreviewResponse
	path := pathf("/v1/branches/%s/merge-preview", branchID)
	if err := c.Do(ctx, "GET", path, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
