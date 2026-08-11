package api

import (
	"context"
	"net/url"
	"strconv"

	"github.com/qase-tms/tiden-mcp-server/internal/model"
)

// ListIssuesOptions mirrors the ListIssues query parameters. Empty fields are
// omitted. IDs are already resolved — callers pass UUIDs, resolving names via
// list_environments / list_releases / list_components first.
type ListIssuesOptions struct {
	Status        string
	EnvironmentID string
	ReleaseID     string
	ComponentID   string
	Platforms     []string
	Levels        []string
	Period        string
	Sort          string
	PageSize      int
	PageToken     string
}

type ListIssuesResponse struct {
	Issues     []model.Issue    `json:"issues"`
	Pagination model.Pagination `json:"pagination"`
	Processing bool             `json:"processing,omitempty"`
}

func (c *Client) ListIssues(ctx context.Context, productID string, opts ListIssuesOptions) (*ListIssuesResponse, error) {
	q := url.Values{}
	if opts.Status != "" {
		q.Set("status", opts.Status)
	}
	if opts.EnvironmentID != "" {
		q.Set("environmentId", opts.EnvironmentID)
	}
	if opts.ReleaseID != "" {
		q.Set("releaseId", opts.ReleaseID)
	}
	if opts.ComponentID != "" {
		q.Set("componentId", opts.ComponentID)
	}
	for _, p := range opts.Platforms {
		q.Add("platforms", p)
	}
	for _, l := range opts.Levels {
		q.Add("levels", l)
	}
	if opts.Period != "" {
		q.Set("period", opts.Period)
	}
	if opts.Sort != "" {
		q.Set("sort", opts.Sort)
	}
	pageSize := opts.PageSize
	if pageSize <= 0 {
		pageSize = 50
	}
	q.Set("pagination.pageSize", strconv.Itoa(pageSize))
	if opts.PageToken != "" {
		q.Set("pagination.pageToken", opts.PageToken)
	}

	var resp ListIssuesResponse
	path := pathf("/v1/products/%s/issues", productID) + "?" + q.Encode()
	if err := c.Do(ctx, "GET", path, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

type GetIssueResponse struct {
	Issue       model.Issue       `json:"issue"`
	LatestEvent *model.IssueEvent `json:"latestEvent,omitempty"`
}

func (c *Client) GetIssue(ctx context.Context, id string) (*GetIssueResponse, error) {
	var resp GetIssueResponse
	if err := c.Do(ctx, "GET", pathf("/v1/issues/%s", id), nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

type ListIssueEventsResponse struct {
	Events     []model.IssueEvent `json:"events"`
	Pagination model.Pagination   `json:"pagination"`
}

func (c *Client) ListIssueEvents(ctx context.Context, id string, pageSize int, pageToken string) (*ListIssueEventsResponse, error) {
	q := url.Values{}
	if pageSize <= 0 {
		pageSize = 50
	}
	q.Set("pagination.pageSize", strconv.Itoa(pageSize))
	if pageToken != "" {
		q.Set("pagination.pageToken", pageToken)
	}
	var resp ListIssueEventsResponse
	path := pathf("/v1/issues/%s/events", id) + "?" + q.Encode()
	if err := c.Do(ctx, "GET", path, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

type getIssueEventResponse struct {
	Event model.IssueEvent `json:"event"`
}

func (c *Client) GetIssueEvent(ctx context.Context, issueID, eventID string) (*model.IssueEvent, error) {
	var resp getIssueEventResponse
	if err := c.Do(ctx, "GET", pathf("/v1/issues/%s/events/%s", issueID, eventID), nil, &resp); err != nil {
		return nil, err
	}
	return &resp.Event, nil
}

func (c *Client) GetIssueEventStats(ctx context.Context, id string) (*model.IssueEventStats, error) {
	var resp model.IssueEventStats
	if err := c.Do(ctx, "GET", pathf("/v1/issues/%s/stats", id), nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

type getIssueFixContextResponse struct {
	Context model.IssueFixContext `json:"context"`
}

// GetIssueFixContext returns the composed triage pack for one issue.
func (c *Client) GetIssueFixContext(ctx context.Context, productID, issueID, branch string, maxFrames int) (*model.IssueFixContext, error) {
	q := url.Values{}
	if branch != "" {
		q.Set("branch", branch)
	}
	if maxFrames > 0 {
		q.Set("maxFrames", strconv.Itoa(maxFrames))
	}
	path := pathf("/v1/products/%s/issues/%s/fix-context", productID, issueID)
	if enc := q.Encode(); enc != "" {
		path += "?" + enc
	}
	var resp getIssueFixContextResponse
	if err := c.Do(ctx, "GET", path, nil, &resp); err != nil {
		return nil, err
	}
	return &resp.Context, nil
}

type updateIssueStatusRequest struct {
	Status string `json:"status"`
}

type updateIssueStatusResponse struct {
	Issue model.Issue `json:"issue"`
}

func (c *Client) UpdateIssueStatus(ctx context.Context, id, status string) (*model.Issue, error) {
	var resp updateIssueStatusResponse
	body := updateIssueStatusRequest{Status: status}
	if err := c.Do(ctx, "POST", pathf("/v1/issues/%s:setStatus", id), body, &resp); err != nil {
		return nil, err
	}
	return &resp.Issue, nil
}

type bulkUpdateIssueStatusRequest struct {
	IDs    []string `json:"ids"`
	Status string   `json:"status"`
}

type bulkUpdateIssueStatusResponse struct {
	UpdatedCount int `json:"updatedCount"`
}

func (c *Client) BulkUpdateIssueStatus(ctx context.Context, productID string, ids []string, status string) (int, error) {
	var resp bulkUpdateIssueStatusResponse
	body := bulkUpdateIssueStatusRequest{IDs: ids, Status: status}
	path := pathf("/v1/products/%s/issues:bulkSetStatus", productID)
	if err := c.Do(ctx, "POST", path, body, &resp); err != nil {
		return 0, err
	}
	return resp.UpdatedCount, nil
}

// ListReleaseIssuesResponse carries the issues first seen in a release plus a
// count of all issues seen during it. seenCount is int64 over protojson.
type ListReleaseIssuesResponse struct {
	NewIssues []model.Issue `json:"newIssues"`
	SeenCount int64         `json:"seenCount,string"`
}

func (c *Client) ListReleaseIssues(ctx context.Context, releaseID string) (*ListReleaseIssuesResponse, error) {
	var resp ListReleaseIssuesResponse
	if err := c.Do(ctx, "GET", pathf("/v1/releases/%s/issues", releaseID), nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
