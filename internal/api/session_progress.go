package api

import (
	"context"

	"github.com/qase-tms/tiden-mcp-server/internal/model"
)

// SessionProgressQuery parameterizes GetSessionProgress. RequirementIDs is the
// client-provided slice (the session record's retrieved ids + the draft id) -
// the server holds no session table, so the slice always rides in the request.
type SessionProgressQuery struct {
	SessionID      string
	RequirementIDs []string
	// IntentBranch scopes the link overlay to that branch's proposals; empty
	// means main-view (durable links only).
	IntentBranch string
}

type sessionProgressRequest struct {
	SessionID      string   `json:"sessionId"`
	RequirementIDs []string `json:"requirementIds"`
	IntentBranch   string   `json:"intentBranch,omitempty"`
}

// GetSessionProgress returns the per-requirement progress of one intent
// session: coverage ladder, session attribution, readiness, and next actions.
// POST with a body (the `:compute` precedent) - requirement ids ride in the
// body, not the URL.
func (c *Client) GetSessionProgress(ctx context.Context, productID string, q SessionProgressQuery) (*model.SessionProgress, error) {
	body := sessionProgressRequest(q)
	var resp model.SessionProgress
	if err := c.Do(ctx, "POST", pathf("/v1/products/%s/quality-gate:session-progress", productID), body, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
