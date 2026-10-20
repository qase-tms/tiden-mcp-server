package api

import (
	"context"
	"fmt"
	"net/url"
	"strconv"

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

// GetRequirement fetches one requirement. An optional branch asks for that
// branch's view of the row (its copy-on-write copy when one exists).
func (c *Client) GetRequirement(ctx context.Context, id string, branch ...string) (*model.Requirement, error) {
	var resp GetRequirementResponse
	path := pathf("/v1/requirements/%s", id)
	if len(branch) > 0 && branch[0] != "" {
		path += "?" + url.Values{"branch": {branch[0]}}.Encode()
	}
	if err := c.Do(ctx, "GET", path, nil, &resp); err != nil {
		return nil, err
	}
	return &resp.Requirement, nil
}

// DefaultSpecPageSize is the detail page size when the caller gives none.
// Full spec bodies are large; 100 per page can exceed an agent's tool output
// limit.
const DefaultSpecPageSize = 25

// ListRequirementsPage fetches exactly one page of full requirements,
// retaining its continuation token.
func (c *Client) ListRequirementsPage(ctx context.Context, productID, branch string, pageSize int, pageToken string) (*ListRequirementsResponse, error) {
	if pageSize == 0 {
		pageSize = DefaultSpecPageSize
	}
	if pageSize < 1 || pageSize > 200 {
		return nil, fmt.Errorf("page_size must be between 1 and 200")
	}
	q := url.Values{}
	q.Set("pagination.pageSize", strconv.Itoa(pageSize))
	if pageToken != "" {
		q.Set("pagination.pageToken", pageToken)
	}
	if branch != "" {
		q.Set("branch", branch)
	}
	var resp ListRequirementsResponse
	if err := c.Do(ctx, "GET", pathf("/v1/products/%s/requirements", productID)+"?"+q.Encode(), nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
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
