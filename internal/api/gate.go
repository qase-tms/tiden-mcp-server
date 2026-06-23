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
	Matrix model.TraceabilityMatrix `json:"matrix"`
}

// ComputeVerdict (re)computes and persists the gate verdict. scope "main" =
// current main; "release" requires releaseID.
func (c *Client) ComputeVerdict(ctx context.Context, productID, scope, releaseID string) (*model.Verdict, error) {
	body := computeVerdictRequest{Scope: scopeEnum(scope)}
	if scope == "release" {
		body.ReleaseID = releaseID
	}
	var resp verdictResponse
	if err := c.Do(ctx, "POST", pathf("/v1/products/%s/quality-gate:compute", productID), body, &resp); err != nil {
		return nil, err
	}
	return &resp.Verdict, nil
}

// GetVerdict returns the latest non-invalidated verdict for a (scope, ref).
func (c *Client) GetVerdict(ctx context.Context, productID, scope, releaseID string) (*model.Verdict, error) {
	q := url.Values{}
	q.Set("scope", scopeEnum(scope))
	if scope == "release" {
		q.Set("releaseId", releaseID)
	}
	var resp verdictResponse
	path := pathf("/v1/products/%s/quality-gate", productID) + "?" + q.Encode()
	if err := c.Do(ctx, "GET", path, nil, &resp); err != nil {
		return nil, err
	}
	return &resp.Verdict, nil
}

// GetTraceability returns the requirement->test matrix the verdict was computed over.
func (c *Client) GetTraceability(ctx context.Context, productID, scope, releaseID string) (*model.TraceabilityMatrix, error) {
	q := url.Values{}
	q.Set("scope", scopeEnum(scope))
	if scope == "release" {
		q.Set("releaseId", releaseID)
	}
	var resp traceabilityResponse
	path := pathf("/v1/products/%s/quality-gate/traceability", productID) + "?" + q.Encode()
	if err := c.Do(ctx, "GET", path, nil, &resp); err != nil {
		return nil, err
	}
	return &resp.Matrix, nil
}
