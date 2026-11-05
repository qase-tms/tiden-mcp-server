package api

import (
	"context"
	"net/url"
	"strconv"

	"github.com/qase-tms/tiden-mcp-server/internal/model"
)

// IssueSpecPair names one issue and one spec by id.
type IssueSpecPair struct {
	IssueID       string `json:"issueId"`
	RequirementID string `json:"requirementId"`
}

type updateIssueLinksRequest struct {
	Add    []IssueSpecPair `json:"add,omitempty"`
	Remove []IssueSpecPair `json:"remove,omitempty"`
}

// IssueLinkResult is the outcome of one pair: linked, already_linked,
// unlinked or not_linked.
type IssueLinkResult struct {
	IssueID       string `json:"issueId"`
	RequirementID string `json:"requirementId"`
	Outcome       string `json:"outcome"`
}

type UpdateIssueLinksResponse struct {
	Results []IssueLinkResult `json:"results"`
}

// UpdateIssueSpecLinks links (add) or unlinks (remove) issue and spec pairs in
// one all-or-nothing call.
func (c *Client) UpdateIssueSpecLinks(ctx context.Context, productID string, add, remove []IssueSpecPair) (*UpdateIssueLinksResponse, error) {
	var resp UpdateIssueLinksResponse
	path := pathf("/v1/products/%s/issue-requirement-links:batchUpdate", productID)
	if err := c.Do(ctx, "POST", path, updateIssueLinksRequest{Add: add, Remove: remove}, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

type ListSpecIssuesResponse struct {
	Issues     []model.Issue `json:"issues"`
	TotalCount int           `json:"totalCount"`
}

// ListSpecIssues returns the issues linked to one spec. limit <= 0 leaves the
// server default.
func (c *Client) ListSpecIssues(ctx context.Context, specID string, limit int) (*ListSpecIssuesResponse, error) {
	path := pathf("/v1/requirements/%s/issues", specID)
	if limit > 0 {
		path += "?" + url.Values{"limit": {strconv.Itoa(limit)}}.Encode()
	}
	var resp ListSpecIssuesResponse
	if err := c.Do(ctx, "GET", path, nil, &resp); err != nil {
		return nil, err
	}
	if resp.Issues == nil {
		resp.Issues = []model.Issue{}
	}
	return &resp, nil
}

type resolveSpecsRequest struct {
	Branch string   `json:"branch,omitempty"`
	Refs   []string `json:"refs"`
}

// ResolvedSpec is one answer of a resolve call, in request order. Status is ok,
// unresolved or ambiguous; only ok rows carry ids.
type ResolvedSpec struct {
	Ref         string `json:"ref"`
	Status      string `json:"status"`
	ViewID      string `json:"viewId,omitempty"`
	CanonicalID string `json:"canonicalId,omitempty"`
	SeqNum      int    `json:"seqNum,omitempty"`
	Title       string `json:"title,omitempty"`
}

type ResolveSpecsResponse struct {
	Requirements []ResolvedSpec `json:"requirements"`
	BranchExists bool           `json:"branchExists"`
}

// ResolveSpecs maps spec refs (KEY-N, a bare number, or a UUID) on a branch to
// ids in one call. An empty branch means main.
func (c *Client) ResolveSpecs(ctx context.Context, productID, branch string, refs []string) (*ResolveSpecsResponse, error) {
	var resp ResolveSpecsResponse
	path := pathf("/v1/products/%s/requirements:resolve", productID)
	if err := c.Do(ctx, "POST", path, resolveSpecsRequest{Branch: branch, Refs: refs}, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
