package api

import (
	"context"
)

// DistillIntentRequest is the body for the backend distill endpoint
// (POST /v1/products/{product_id}/intent:distill). product_id comes from the
// path. It mirrors the CLI's DistillIntentRequest and adds the sessionId/agent
// provenance fields the MCP tool forwards (older servers ignore unknown fields).
type DistillIntentRequest struct {
	Transcript   string   `json:"transcript"`
	CredentialID string   `json:"credentialId,omitempty"`
	Model        string   `json:"model,omitempty"`
	Slug         string   `json:"slug,omitempty"`
	SessionID    string   `json:"sessionId,omitempty"`
	Agent        string   `json:"agent,omitempty"`
	ChangedFiles []string `json:"changedFiles,omitempty"`
}

// DistillIntentResponse mirrors the backend IntentService response.
type DistillIntentResponse struct {
	IntentBranch string `json:"intentBranch"`
	Created      int    `json:"created"`
	Updated      int    `json:"updated"`
	Dropped      int    `json:"dropped"`
	Skipped      bool   `json:"skipped"`
	SkipReason   string `json:"skipReason"`
}

// DistillIntent invokes the backend (engine:"backend") distill path: the
// server runs the LLM + reconciliation + branch write using a workspace
// credential. It routinely takes longer than the client's default timeout, so
// callers should wrap the client with WithTimeout before calling.
func (c *Client) DistillIntent(ctx context.Context, productID string, body DistillIntentRequest) (*DistillIntentResponse, error) {
	var resp DistillIntentResponse
	path := pathf("/v1/products/%s/intent:distill", productID)
	if err := c.Do(ctx, "POST", path, body, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
