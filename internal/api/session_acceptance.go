package api

import "context"

type SessionRiskAcceptance struct {
	RequirementRefs []string `json:"requirementRefs"`
	Criterion       string   `json:"criterion"`
	Evidence        string   `json:"evidence"`
	FollowUp        string   `json:"followUp"`
}

type RecordSessionRiskAcceptancesRequest struct {
	RequirementID               string                  `json:"requirementId"`
	IntentBranch                string                  `json:"intentBranch"`
	SessionID                   string                  `json:"sessionId"`
	Acceptances                 []SessionRiskAcceptance `json:"acceptances"`
	ProposedTestRequirementRefs []string                `json:"proposedTestRequirementRefs,omitempty"`
}

type RecordSessionRiskAcceptancesResponse struct {
	AcceptancesRecorded  int `json:"acceptancesRecorded"`
	DeferredRequirements int `json:"deferredRequirements"`
	ReplacedRows         int `json:"replacedRows"`
}

func (c *Client) RecordSessionRiskAcceptances(ctx context.Context, productID string, request RecordSessionRiskAcceptancesRequest) (*RecordSessionRiskAcceptancesResponse, error) {
	var response RecordSessionRiskAcceptancesResponse
	if err := c.Do(ctx, "POST", pathf("/v1/products/%s/quality-gate:session-acceptances", productID), request, &response); err != nil {
		return nil, err
	}
	return &response, nil
}
