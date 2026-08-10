package api

import "context"

type SessionRiskAcceptance struct {
	RequirementIDs []string `json:"requirementIds"`
	Criterion      string   `json:"criterion"`
	Evidence       string   `json:"evidence"`
	FollowUp       string   `json:"followUp"`
}

type RecordSessionRiskAcceptancesRequest struct {
	RequirementID              string                  `json:"requirementId"`
	IntentBranch               string                  `json:"intentBranch"`
	SessionID                  string                  `json:"sessionId"`
	Acceptances                []SessionRiskAcceptance `json:"acceptances"`
	ProposedTestRequirementIDs []string                `json:"proposedTestRequirementIds,omitempty"`
}

type RecordSessionRiskAcceptancesResponse struct {
	RecordedCount     int `json:"recordedCount"`
	DeduplicatedCount int `json:"deduplicatedCount"`
}

func (c *Client) RecordSessionRiskAcceptances(ctx context.Context, productID string, request RecordSessionRiskAcceptancesRequest) (*RecordSessionRiskAcceptancesResponse, error) {
	var response RecordSessionRiskAcceptancesResponse
	if err := c.Do(ctx, "POST", pathf("/v1/products/%s/quality-gate:session-acceptances", productID), request, &response); err != nil {
		return nil, err
	}
	return &response, nil
}
