package api

import (
	"context"
	"net/url"
)

// MergeTestCasesRequest mirrors api.v1.MergeTestCasesRequest's body: ref is in
// the path, IntoRef is the survivor. Both are a case number (seq) or a case id.
type MergeTestCasesRequest struct {
	IntoRef string `json:"intoRef"`
	Reason  string `json:"reason,omitempty"`
}

// MergeTestCasesResponse mirrors api.v1.MergeTestCasesResponse.
type MergeTestCasesResponse struct {
	MergeID     string `json:"mergeId"`
	SurvivorSeq int    `json:"survivorSeq"`
	MergedSeq   int    `json:"mergedSeq"`
	// Queued is true until the server's identity worker has moved the rows.
	Queued bool `json:"queued"`
}

// UndoTestCaseMergeResponse mirrors api.v1.UndoTestCaseMergeResponse.
type UndoTestCaseMergeResponse struct {
	RestoredSeq int `json:"restoredSeq"`
	// AmbiguousAfterMerge counts rows that arrived on the survivor after the
	// merge and carry no evidence of which case they belong to; they stay on
	// the survivor.
	AmbiguousAfterMerge int `json:"ambiguousAfterMerge"`
}

// TestCaseMerge mirrors api.v1.TestCaseMerge. Status is queued | merged |
// undone; ActorName is empty for an automatic merge.
type TestCaseMerge struct {
	MergeID     string `json:"mergeId"`
	SurvivorID  string `json:"survivorId"`
	SurvivorSeq int    `json:"survivorSeq"`
	MergedID    string `json:"mergedId"`
	MergedSeq   int    `json:"mergedSeq"`
	Status      string `json:"status"`
	Reason      string `json:"reason,omitempty"`
	ActorName   string `json:"actorName,omitempty"`
	CreatedAt   string `json:"createdAt,omitempty"`
}

// ListTestCaseMergesResponse mirrors api.v1.ListTestCaseMergesResponse.
type ListTestCaseMergesResponse struct {
	Merges []TestCaseMerge `json:"merges"`
}

// MergeTestCases folds the case ref into the case intoRef (the survivor). A
// refusal (same case, another product, a pair an earlier undo blocked) comes
// back as an *APIError with the reason.
func (c *Client) MergeTestCases(ctx context.Context, productID, ref string, body MergeTestCasesRequest) (*MergeTestCasesResponse, error) {
	var resp MergeTestCasesResponse
	path := pathf("/v1/products/%s/tests/%s", productID, ref) + ":merge"
	if err := c.Do(ctx, "POST", path, body, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// UndoTestCaseMerge takes back one merge. Only the newest merge in the chain
// that touches either case can be undone; otherwise the *APIError names the
// newer merge to undo first.
func (c *Client) UndoTestCaseMerge(ctx context.Context, productID, mergeID string) (*UndoTestCaseMergeResponse, error) {
	var resp UndoTestCaseMergeResponse
	path := pathf("/v1/products/%s/test-merges/%s", productID, mergeID) + ":undo"
	if err := c.Do(ctx, "POST", path, struct{}{}, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// ListTestCaseMerges lists a product's merges, newest first; testRef (a case
// seq or id) narrows them to the merges that touch that case.
func (c *Client) ListTestCaseMerges(ctx context.Context, productID, testRef string) (*ListTestCaseMergesResponse, error) {
	var resp ListTestCaseMergesResponse
	path := pathf("/v1/products/%s/test-merges", productID)
	if testRef != "" {
		path += "?" + url.Values{"testRef": {testRef}}.Encode()
	}
	if err := c.Do(ctx, "GET", path, nil, &resp); err != nil {
		return nil, err
	}
	if resp.Merges == nil {
		resp.Merges = []TestCaseMerge{}
	}
	return &resp, nil
}
