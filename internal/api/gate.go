package api

import (
	"context"
	"net/url"

	"github.com/qase-tms/tiden-mcp-server/internal/model"
)

// scopeEnum maps a CLI scope ("release" | "branch" | "main") to the backend
// VerdictScope proto enum name. "main" = current main-live state (not tied to a
// release); the default when no release is given.
func scopeEnum(scope string) string {
	switch scope {
	case "release":
		return "VERDICT_SCOPE_RELEASE"
	case "branch":
		return "VERDICT_SCOPE_BRANCH"
	default:
		return "VERDICT_SCOPE_MAIN"
	}
}

type computeVerdictRequest struct {
	Scope     string `json:"scope"`
	ReleaseID string `json:"releaseId,omitempty"`
	Branch    string `json:"branch,omitempty"`
}

type verdictResponse struct {
	Verdict model.Verdict `json:"verdict"`
}

type traceabilityResponse struct {
	Matrix  model.TraceabilityMatrix   `json:"matrix"`
	Summary *model.TraceabilitySummary `json:"summary,omitempty"`
	View    string                     `json:"view,omitempty"`
}

// TraceabilityOptions narrows a traceability read.
type TraceabilityOptions struct {
	// View is "summary" for rows without per-test cells; empty keeps the
	// complete matrix (the default of the API).
	View string
	// FeatureID limits the answer to one spec subtree.
	FeatureID string
}

// TraceabilityResult is the answer of GetTraceabilityView. Summary is set only
// when the server returned the summary view. A server that does not know the
// requested view or filter ignores it and answers with the full matrix.
type TraceabilityResult struct {
	Matrix  model.TraceabilityMatrix
	Summary *model.TraceabilitySummary
	View    string
}

// ComputeVerdict (re)computes and persists the gate verdict. scope "main" =
// current main; "release" requires releaseID; "branch" requires branch.
func (c *Client) ComputeVerdict(ctx context.Context, productID, scope, releaseID, branch string) (*model.Verdict, error) {
	body := computeVerdictRequest{Scope: scopeEnum(scope)}
	if scope == "release" {
		body.ReleaseID = releaseID
	}
	if scope == "branch" {
		body.Branch = branch
	}
	var resp verdictResponse
	if err := c.Do(ctx, "POST", pathf("/v1/products/%s/quality-gate:compute", productID), body, &resp); err != nil {
		return nil, err
	}
	return &resp.Verdict, nil
}

// GetVerdict returns the latest non-invalidated verdict for a (scope, ref).
func (c *Client) GetVerdict(ctx context.Context, productID, scope, releaseID, branch string) (*model.Verdict, error) {
	q := gateQueryValues(scope, releaseID, branch)
	var resp verdictResponse
	path := pathf("/v1/products/%s/quality-gate", productID) + "?" + q.Encode()
	if err := c.Do(ctx, "GET", path, nil, &resp); err != nil {
		return nil, err
	}
	return &resp.Verdict, nil
}

// GetTraceability returns the requirement->test matrix the verdict was computed over.
func (c *Client) GetTraceability(ctx context.Context, productID, scope, releaseID, branch string) (*model.TraceabilityMatrix, error) {
	q := gateQueryValues(scope, releaseID, branch)
	var resp traceabilityResponse
	path := pathf("/v1/products/%s/quality-gate/traceability", productID) + "?" + q.Encode()
	if err := c.Do(ctx, "GET", path, nil, &resp); err != nil {
		return nil, err
	}
	return &resp.Matrix, nil
}

// GetTraceabilityView is GetTraceability with a view and an optional feature
// subtree. Fields the server does not know are dropped by the host, so an old
// server answers with the complete matrix.
func (c *Client) GetTraceabilityView(ctx context.Context, productID, scope, releaseID, branch string, opts TraceabilityOptions) (*TraceabilityResult, error) {
	q := gateQueryValues(scope, releaseID, branch)
	if opts.View != "" {
		q.Set("view", opts.View)
	}
	if opts.FeatureID != "" {
		q.Set("subjectType", "feature")
		q.Set("subjectId", opts.FeatureID)
	}
	var resp traceabilityResponse
	path := pathf("/v1/products/%s/quality-gate/traceability", productID) + "?" + q.Encode()
	if err := c.Do(ctx, "GET", path, nil, &resp); err != nil {
		return nil, err
	}
	return &TraceabilityResult{Matrix: resp.Matrix, Summary: resp.Summary, View: resp.View}, nil
}

// gateQueryValues builds the scope query params for verdict/traceability
// reads: releaseId only for release scope, branch only for branch scope.
func gateQueryValues(scope, releaseID, branch string) url.Values {
	q := url.Values{}
	q.Set("scope", scopeEnum(scope))
	if scope == "release" {
		q.Set("releaseId", releaseID)
	}
	if scope == "branch" {
		q.Set("branch", branch)
	}
	return q
}
