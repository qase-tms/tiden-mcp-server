package api

import (
	"context"
	"encoding/json"
	"net/url"
	"strconv"
	"strings"

	"github.com/qase-tms/tiden-mcp-server/internal/model"
)

// ----- runs (lifecycle) -----

type CreateTestRunRequest struct {
	Title          string            `json:"title,omitempty"`
	Description    string            `json:"description,omitempty"`
	Environment    string            `json:"environment,omitempty"`
	Branch         string            `json:"branch,omitempty"`
	Configurations map[string]string `json:"configurations,omitempty"`
	BuildSha       string            `json:"buildSha,omitempty"`
	ClientMeta     map[string]string `json:"clientMeta,omitempty"`
}

type runEnvelope struct {
	Run model.TestRun `json:"run"`
}

func (c *Client) CreateTestRun(ctx context.Context, productID string, body CreateTestRunRequest) (*model.TestRun, error) {
	var resp runEnvelope
	path := pathf("/v1/products/%s/runs", productID)
	if err := c.Do(ctx, "POST", path, body, &resp); err != nil {
		return nil, err
	}
	return &resp.Run, nil
}

type ListTestRunsOptions struct {
	Status      string
	Environment string
	Branch      string
	Search      string
	PageSize    int
	PageToken   string
}

type ListTestRunsResponse struct {
	Runs       []model.TestRun  `json:"runs"`
	Pagination model.Pagination `json:"pagination"`
}

func (c *Client) ListTestRuns(ctx context.Context, productID string, opts ListTestRunsOptions) (*ListTestRunsResponse, error) {
	q := url.Values{}
	if opts.Status != "" {
		q.Set("status", opts.Status)
	}
	if opts.Environment != "" {
		q.Set("environment", opts.Environment)
	}
	if opts.Branch != "" {
		q.Set("branch", opts.Branch)
	}
	if opts.Search != "" {
		q.Set("search", opts.Search)
	}
	pageSize := opts.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 200 {
		pageSize = 200
	}
	q.Set("pagination.pageSize", strconv.Itoa(pageSize))
	if opts.PageToken != "" {
		q.Set("pagination.pageToken", opts.PageToken)
	}
	var resp ListTestRunsResponse
	path := pathf("/v1/products/%s/runs", productID) + "?" + q.Encode()
	if err := c.Do(ctx, "GET", path, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) GetTestRun(ctx context.Context, productID string, seq int) (*model.TestRun, error) {
	var resp runEnvelope
	path := pathf("/v1/products/%s/runs/%s", productID, strconv.Itoa(seq))
	if err := c.Do(ctx, "GET", path, nil, &resp); err != nil {
		return nil, err
	}
	return &resp.Run, nil
}

// CompleteTestRun computes the verdict once, locks the run, and (when the
// product has live documentation enabled) triggers the background live-doc
// sync. Idempotent on an already-completed run; INVALID_RUN_STATE on an
// aborted one.
func (c *Client) CompleteTestRun(ctx context.Context, productID string, seq int) (*model.TestRun, error) {
	var resp runEnvelope
	path := pathf("/v1/products/%s/runs/%s:complete", productID, strconv.Itoa(seq))
	if err := c.Do(ctx, "POST", path, nil, &resp); err != nil {
		return nil, err
	}
	return &resp.Run, nil
}

func (c *Client) AbortTestRun(ctx context.Context, productID string, seq int) (*model.TestRun, error) {
	var resp runEnvelope
	path := pathf("/v1/products/%s/runs/%s:abort", productID, strconv.Itoa(seq))
	if err := c.Do(ctx, "POST", path, nil, &resp); err != nil {
		return nil, err
	}
	return &resp.Run, nil
}

// ----- results -----

type ReportResultsRequest struct {
	Results []map[string]any `json:"results"`
}

// ReportError is one per-entry validation failure from ReportResults, carried
// in the 400 error envelope's details[] (api.v1.ReportError).
type ReportError struct {
	Index    int    `json:"index"`
	ResultID string `json:"resultId,omitempty"`
	Code     string `json:"code"`
	Message  string `json:"message"`
}

type ReportResultsResponse struct {
	// Status is always true on a 2xx (Qase client compat) — check errors via
	// the HTTP status, not this field.
	Status     bool          `json:"status"`
	Accepted   int64         `json:"accepted,string"`
	Duplicates int64         `json:"duplicates,string"`
	Errors     []ReportError `json:"errors"`
}

// ReportResults submits a validate-then-write batch (cap 2000): either every
// entry is accepted or nothing is written and the per-entry errors ride in
// the 400 envelope (extract with ParseReportErrors). Entries pass through
// verbatim — the MCP server never rewrites reporter payloads.
func (c *Client) ReportResults(ctx context.Context, productID string, seq int, results []map[string]any) (*ReportResultsResponse, error) {
	var resp ReportResultsResponse
	path := pathf("/v1/products/%s/runs/%s/results:report", productID, strconv.Itoa(seq))
	if err := c.Do(ctx, "POST", path, ReportResultsRequest{Results: results}, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// ParseReportErrors extracts per-entry ReportError records from a
// grpc-gateway error body, mirroring ParseIngestErrors. Matches details[]
// entries by @type suffix to avoid coupling to the full proto path.
func ParseReportErrors(body string) ([]ReportError, bool) {
	var envelope struct {
		Details []map[string]any `json:"details"`
	}
	if err := json.Unmarshal([]byte(body), &envelope); err != nil {
		return nil, false
	}
	out := make([]ReportError, 0, len(envelope.Details))
	for _, d := range envelope.Details {
		typ, _ := d["@type"].(string)
		if !strings.HasSuffix(typ, "ReportError") {
			continue
		}
		idx, _ := d["index"].(float64)
		rid, _ := d["resultId"].(string)
		code, _ := d["code"].(string)
		msg, _ := d["message"].(string)
		out = append(out, ReportError{Index: int(idx), ResultID: rid, Code: code, Message: msg})
	}
	return out, len(out) > 0
}

type ListRunResultsOptions struct {
	Status      string
	Search      string
	IdentityKey string
	LatestOnly  bool
	PageSize    int
	PageToken   string
}

type ListRunResultsResponse struct {
	Results    []model.TestRunResult `json:"results"`
	Pagination model.Pagination      `json:"pagination"`
}

func (c *Client) ListRunResults(ctx context.Context, productID string, seq int, opts ListRunResultsOptions) (*ListRunResultsResponse, error) {
	q := url.Values{}
	if opts.Status != "" {
		q.Set("status", opts.Status)
	}
	if opts.Search != "" {
		q.Set("search", opts.Search)
	}
	if opts.IdentityKey != "" {
		q.Set("identityKey", opts.IdentityKey)
	}
	if opts.LatestOnly {
		q.Set("latestOnly", "true")
	}
	pageSize := opts.PageSize
	if pageSize <= 0 {
		pageSize = 50
	}
	if pageSize > 200 {
		pageSize = 200
	}
	q.Set("pagination.pageSize", strconv.Itoa(pageSize))
	if opts.PageToken != "" {
		q.Set("pagination.pageToken", opts.PageToken)
	}
	var resp ListRunResultsResponse
	path := pathf("/v1/products/%s/runs/%s/results", productID, strconv.Itoa(seq)) + "?" + q.Encode()
	if err := c.Do(ctx, "GET", path, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

type GetRunSummaryResponse struct {
	Suites []model.RunSuiteSummary `json:"suites"`
	Cases  []model.RunCaseSummary  `json:"cases"`
}

func (c *Client) GetRunSummary(ctx context.Context, productID string, seq int) (*GetRunSummaryResponse, error) {
	var resp GetRunSummaryResponse
	path := pathf("/v1/products/%s/runs/%s/summary", productID, strconv.Itoa(seq))
	if err := c.Do(ctx, "GET", path, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
