package api

import "context"

// SessionRiskAcceptance is one priced exception in a
// RecordSessionRiskAcceptances call, mirroring api.v1.SessionRiskAcceptance
// (tiden-app#365) and tiden-cli's internal/api.SessionRiskAcceptance — the
// two client shapes are kept byte-identical on purpose so they cannot drift.
// RequirementRefs are exactly as the caller typed them ("<CODE>-<N>" or a
// canonical lowercase UUID); the server resolves them against the intent
// branch's view itself.
type SessionRiskAcceptance struct {
	RequirementRefs []string `json:"requirementRefs"`
	Criterion       string   `json:"criterion"`
	Evidence        string   `json:"evidence"`
	FollowUp        string   `json:"followUp"`
}

// RecordSessionRiskAcceptancesRequest is the POST body for
// /v1/products/{productId}/quality-gate:session-acceptances. ProductID rides
// the URL (grpc-gateway path capture), not the body.
type RecordSessionRiskAcceptancesRequest struct {
	// RequirementID (v2, optional): the session's draft requirement
	// (branch-local on IntentBranch) — the legacy write path, kept for older
	// CLIs. Empty is omitted from the request body (not sent as ""): the
	// server then requires the session record (intent_sessions) to already
	// exist and writes acceptances/deferrals to intent_session_judgements
	// instead of onto a draft's provenance.
	RequirementID string `json:"requirementId,omitempty"`
	// IntentBranch is the session's Tiden branch; must be non-empty and not "main".
	IntentBranch string `json:"intentBranch"`
	// SessionID is the client-generated intent session UUID.
	SessionID string `json:"sessionId"`
	// Acceptances are the priced exceptions; empty is fine when only deferrals
	// are being recorded.
	Acceptances []SessionRiskAcceptance `json:"acceptances,omitempty"`
	// ProposedTestRequirementRefs is the deferral half of the ledger: refs
	// whose missing test is handed to a next session. They collapse into ONE
	// test_deferral row server-side.
	ProposedTestRequirementRefs []string `json:"proposedTestRequirementRefs,omitempty"`
}

// RecordSessionRiskAcceptancesResponse mirrors
// api.v1.RecordSessionRiskAcceptancesResponse.
type RecordSessionRiskAcceptancesResponse struct {
	AcceptancesRecorded  int `json:"acceptancesRecorded"`
	DeferredRequirements int `json:"deferredRequirements"`
	// ReplacedRows counts this session's equivalent rows an earlier call wrote
	// that THIS call superseded — a retry-idempotency signal (the server's
	// dedup key is (phase, session_id, sorted requirement ids), so a retry of
	// the same call is safe and simply replaces its own rows).
	ReplacedRows int `json:"replacedRows"`
}

// RecordSessionRiskAcceptances records one intent session's risk acceptances
// and test deferrals as agent_artifact rows on the session draft
// (tiden-app#365). Validation is structural only — the server never judges
// whether a reason is a good one; a rejection's Message (surfaced via
// APIError, forwarded verbatim by the caller) is what teaches the agent.
//
// # Ordering constraint (load-bearing for any future caller that also writes sources)
//
// This RPC rewrites the draft's whole `sources` array. A caller that also
// writes sources to the same draft (as tiden-cli's `intent close` does, in
// its own PUT) MUST call this FIRST and RE-FETCH the requirement before
// building that write — a request assembled from a snapshot taken before
// this call silently erases the rows this call wrote. This package has no
// such second writer today (it does not implement the intent lifecycle; see
// the record_risk_acceptances tool description), so the constraint is
// dormant here, not applied.
func (c *Client) RecordSessionRiskAcceptances(ctx context.Context, productID string, req RecordSessionRiskAcceptancesRequest) (*RecordSessionRiskAcceptancesResponse, error) {
	var resp RecordSessionRiskAcceptancesResponse
	if err := c.Do(ctx, "POST", pathf("/v1/products/%s/quality-gate:session-acceptances", productID), req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
