package api

import (
	"context"
	"net/url"

	"github.com/qase-tms/tiden-mcp-server/internal/model"
)

// ----- list / get / mutate -----

type ListTestsResponse struct {
	Tests      []model.Test     `json:"tests"`
	Pagination model.Pagination `json:"pagination"`
}

type CreateTestRequest struct {
	Kind         string           `json:"kind"`
	Title        string           `json:"title"`
	Description  string           `json:"description,omitempty"`
	ParentID     *string          `json:"parentId,omitempty"`
	Branch       string           `json:"branch,omitempty"`
	Status       string           `json:"status,omitempty"`
	Priority     string           `json:"priority,omitempty"`
	Type         string           `json:"type,omitempty"`
	Layer        string           `json:"layer,omitempty"`
	Muted        bool             `json:"muted,omitempty"`
	ComponentID  *string          `json:"componentId,omitempty"`
	AssigneeID   *string          `json:"assigneeId,omitempty"`
	Tags         []string         `json:"tags,omitempty"`
	CustomFields map[string]any   `json:"customFields,omitempty"`
	Steps        []model.TestStep `json:"steps,omitempty"`
	Framework    string           `json:"framework,omitempty"`
	FilePath     string           `json:"filePath,omitempty"`
}

type CreateTestResponse struct {
	Test model.Test `json:"test"`
}

func (c *Client) ListTests(ctx context.Context, productID string, branch ...string) (*ListTestsResponse, error) {
	var resp ListTestsResponse
	path := pathf("/v1/products/%s/tests?pagination.pageSize=100", productID)
	if len(branch) > 0 && branch[0] != "" {
		path += "&branch=" + url.QueryEscape(branch[0])
	}
	if err := c.Do(ctx, "GET", path, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) GetTest(ctx context.Context, id string) (*model.Test, error) {
	var resp struct {
		Test model.Test `json:"test"`
	}
	if err := c.Do(ctx, "GET", pathf("/v1/tests/%s", id), nil, &resp); err != nil {
		return nil, err
	}
	return &resp.Test, nil
}

func (c *Client) CreateTest(ctx context.Context, productID string, body CreateTestRequest) (*model.Test, error) {
	var resp CreateTestResponse
	path := pathf("/v1/products/%s/tests", productID)
	if err := c.Do(ctx, "POST", path, body, &resp); err != nil {
		return nil, err
	}
	return &resp.Test, nil
}

type UpdateTestRequest struct {
	Title       *string `json:"title,omitempty"`
	Description *string `json:"description,omitempty"`
	Status      *string `json:"status,omitempty"`
	Priority    *string `json:"priority,omitempty"`
	Type        *string `json:"type,omitempty"`
	Layer       *string `json:"layer,omitempty"`
	Muted       *bool   `json:"muted,omitempty"`
	ComponentID *string `json:"componentId,omitempty"`
	AssigneeID  *string `json:"assigneeId,omitempty"`
	Branch      string  `json:"branch,omitempty"`
}

type UpdateTestResponse struct {
	Test model.Test `json:"test"`
}

func (c *Client) UpdateTest(ctx context.Context, id string, body UpdateTestRequest) (*model.Test, error) {
	var resp UpdateTestResponse
	if err := c.Do(ctx, "PUT", pathf("/v1/tests/%s", id), body, &resp); err != nil {
		return nil, err
	}
	return &resp.Test, nil
}

type LinkRequirementRequest struct {
	RequirementID string `json:"requirementId"`
	Branch        string `json:"branch,omitempty"`
}

func (c *Client) LinkRequirement(ctx context.Context, testID, requirementID, branch string) error {
	body := LinkRequirementRequest{RequirementID: requirementID, Branch: branch}
	path := pathf("/v1/tests/%s/links", testID)
	return c.Do(ctx, "POST", path, body, nil)
}
