package api

import (
	"context"
	"fmt"
	"net/url"
	"strconv"

	"github.com/qase-tms/tiden-mcp-server/internal/model"
)

type RunSummaryProjectionOptions struct {
	View        string
	IdentityKey string
	PageSize    int
	PageToken   string
}

type RunSummaryProjection struct {
	View          string                  `json:"view"`
	Suites        []model.RunSuiteSummary `json:"suites"`
	CaseOverviews []model.RunCaseOverview `json:"caseOverviews"`
	Combos        []model.RunParamCombo   `json:"combos"`
	Pagination    *model.Pagination       `json:"pagination,omitempty"`
	CaseCount     *int64                  `json:"caseCount,omitempty,string"`
}

// GetRunSummaryProjection returns one explicitly requested read-only view. It
// never silently falls back to a full tree or traverses continuation pages.
func (c *Client) GetRunSummaryProjection(ctx context.Context, productID string, seq int, opts RunSummaryProjectionOptions) (*RunSummaryProjection, error) {
	if opts.View != "overview" && opts.View != "cases" && opts.View != "combos" {
		return nil, fmt.Errorf("summary view must be overview, cases or combos")
	}
	if (opts.View == "combos") != (opts.IdentityKey != "") {
		return nil, fmt.Errorf("identity_key is required only for combos")
	}
	if opts.View == "overview" && (opts.PageSize != 0 || opts.PageToken != "") {
		return nil, fmt.Errorf("overview does not accept pagination")
	}
	size := opts.PageSize
	if size == 0 {
		size = 100
	}
	if size < 1 || size > 200 {
		return nil, fmt.Errorf("page size must be between 1 and 200")
	}
	q := url.Values{"view": {opts.View}}
	if opts.View != "overview" {
		q.Set("pagination.pageSize", strconv.Itoa(size))
		if opts.PageToken != "" {
			q.Set("pagination.pageToken", opts.PageToken)
		}
	}
	if opts.IdentityKey != "" {
		q.Set("identityKey", opts.IdentityKey)
	}
	var resp RunSummaryProjection
	path := pathf("/v1/products/%s/runs/%s/summary", productID, strconv.Itoa(seq)) + "?" + q.Encode()
	if err := c.Do(ctx, "GET", path, nil, &resp); err != nil {
		return nil, err
	}
	if resp.View != opts.View {
		return nil, fmt.Errorf("server does not support the requested summary view; use full detail explicitly if needed")
	}
	count := 0
	switch opts.View {
	case "overview":
		if resp.Suites == nil || resp.CaseCount == nil || *resp.CaseCount < 0 {
			return nil, fmt.Errorf("incomplete summary overview")
		}
		for _, suite := range resp.Suites {
			if suite.Stats == nil {
				return nil, fmt.Errorf("summary suite is missing stats")
			}
		}
		// Other projection fields are emitted empty by protojson. Keep only this view.
		resp.Pagination, resp.CaseOverviews, resp.Combos = nil, nil, nil
		return &resp, nil
	case "cases":
		if resp.CaseOverviews == nil {
			return nil, fmt.Errorf("missing case overviews")
		}
		seen := map[string]bool{}
		for _, row := range resp.CaseOverviews {
			if row.IdentityKey == "" || seen[row.IdentityKey] || row.ComboCount < 1 {
				return nil, fmt.Errorf("invalid or duplicate case overview")
			}
			seen[row.IdentityKey] = true
		}
		count = len(resp.CaseOverviews)
		resp.Suites, resp.Combos, resp.CaseCount = nil, nil, nil
	case "combos":
		if resp.Combos == nil {
			return nil, fmt.Errorf("missing combination page")
		}
		seen := map[string]bool{}
		for _, row := range resp.Combos {
			if row.ExecutionKey == "" || seen[row.ExecutionKey] || row.ResultID == "" || row.Params == nil {
				return nil, fmt.Errorf("invalid or duplicate combination overview")
			}
			seen[row.ExecutionKey] = true
		}
		count = len(resp.Combos)
		resp.Suites, resp.CaseOverviews, resp.CaseCount = nil, nil, nil
	}
	if resp.Pagination == nil || resp.Pagination.TotalCount < count || count > size {
		return nil, fmt.Errorf("invalid summary pagination")
	}
	if resp.Pagination.NextPageToken != "" && (count == 0 || resp.Pagination.NextPageToken == opts.PageToken) {
		return nil, fmt.Errorf("non-progressing summary pagination")
	}
	return &resp, nil
}
