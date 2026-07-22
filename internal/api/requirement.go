package api

import (
	"context"
	"net/url"

	"github.com/qase-tms/tiden-mcp-server/internal/model"
)

type ListRequirementsResponse struct {
	Requirements []model.Requirement `json:"requirements"`
	Pagination   model.Pagination    `json:"pagination"`
}

type CreateRequirementRequest struct {
	Title       string                         `json:"title"`
	Content     string                         `json:"content"`
	ParentID    *string                        `json:"parentId,omitempty"`
	ComponentID *string                        `json:"componentId,omitempty"`
	Branch      string                         `json:"branch,omitempty"`
	Type        string                         `json:"type,omitempty"`
	Sources     []model.RequirementSourceInput `json:"sources,omitempty"`
}

type CreateRequirementResponse struct {
	Requirement model.Requirement `json:"requirement"`
}

type UpdateRequirementRequest struct {
	Title         *string                         `json:"title,omitempty"`
	Content       *string                         `json:"content,omitempty"`
	ParentID      *string                         `json:"parentId,omitempty"`
	ComponentID   *string                         `json:"componentId,omitempty"`
	Status        *string                         `json:"status,omitempty"`
	Priority      *string                         `json:"priority,omitempty"`
	Type          *string                         `json:"type,omitempty"`
	Branch        string                          `json:"branch,omitempty"`
	SourcesUpdate *model.RequirementSourcesUpdate `json:"sourcesUpdate,omitempty"`
}

type UpdateRequirementResponse struct {
	Requirement model.Requirement `json:"requirement"`
}

type GetRequirementResponse struct {
	Requirement model.Requirement `json:"requirement"`
}

func (c *Client) GetRequirement(ctx context.Context, id string) (*model.Requirement, error) {
	var resp GetRequirementResponse
	path := pathf("/v1/requirements/%s", id)
	if err := c.Do(ctx, "GET", path, nil, &resp); err != nil {
		return nil, err
	}
	return &resp.Requirement, nil
}

// ListRequirements fetches all requirements for a product/branch, following
// pagination.nextPageToken across pages until exhausted (capped at 100
// pages) rather than returning only the first page.
func (c *Client) ListRequirements(ctx context.Context, productID string, branch ...string) (*ListRequirementsResponse, error) {
	qBranch := ""
	if len(branch) > 0 {
		qBranch = branch[0]
	}
	var out ListRequirementsResponse
	pageToken := ""
	for pages := 0; pages < 100; pages++ {
		var resp ListRequirementsResponse
		q := url.Values{}
		q.Set("pagination.pageSize", "200")
		if pageToken != "" {
			q.Set("pagination.pageToken", pageToken)
		}
		if qBranch != "" {
			q.Set("branch", qBranch)
		}
		path := pathf("/v1/products/%s/requirements", productID) + "?" + q.Encode()
		if err := c.Do(ctx, "GET", path, nil, &resp); err != nil {
			return nil, err
		}
		out.Requirements = append(out.Requirements, resp.Requirements...)
		out.Pagination.TotalCount = resp.Pagination.TotalCount
		pageToken = resp.Pagination.NextPageToken
		if pageToken == "" {
			out.Pagination.NextPageToken = ""
			return &out, nil
		}
	}
	out.Pagination.NextPageToken = pageToken
	return &out, nil
}

func (c *Client) CreateRequirementTypedWithSources(ctx context.Context, productID, title, content string, parentID *string, componentID *string, reqType string, sources []model.RequirementSourceInput, branch ...string) (*model.Requirement, error) {
	body := CreateRequirementRequest{
		Title:       title,
		Content:     content,
		ParentID:    parentID,
		ComponentID: componentID,
		Type:        reqType,
		Sources:     sources,
	}
	if len(branch) > 0 && branch[0] != "" {
		body.Branch = branch[0]
	}
	var resp CreateRequirementResponse
	path := pathf("/v1/products/%s/requirements", productID)
	if err := c.Do(ctx, "POST", path, body, &resp); err != nil {
		return nil, err
	}
	return &resp.Requirement, nil
}

func (c *Client) UpdateRequirement(ctx context.Context, id string, body UpdateRequirementRequest) (*model.Requirement, error) {
	var resp UpdateRequirementResponse
	path := pathf("/v1/requirements/%s", id)
	if err := c.Do(ctx, "PUT", path, body, &resp); err != nil {
		return nil, err
	}
	return &resp.Requirement, nil
}
