package api

import (
	"context"
)

type Branch struct {
	ID          string `json:"id"`
	ProductID   string `json:"productId"`
	Name        string `json:"name"`
	Description string `json:"description"`
	CreatedBy   string `json:"createdBy"`
	Status      string `json:"status"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`

	// AutoMerge is where the automatic merge of an intent branch stands.
	// Nil for a branch the server reports no automatic-merge state for
	// (every branch from an older server).
	AutoMerge *AutoMerge `json:"autoMerge,omitempty"`
	// MergeMode says how a merged branch was merged (for example
	// "auto" or "checkpoint"); empty for an open branch or an older server.
	MergeMode string `json:"mergeMode,omitempty"`
	// MergedByName is the display name of whoever, or whatever, merged the
	// branch.
	MergedByName string `json:"mergedByName,omitempty"`
	// ContinuesBranchID is the merged branch this branch picks up from.
	ContinuesBranchID string `json:"continuesBranchId,omitempty"`
}

// AutoMerge describes the automatic merge of an intent branch. State is kept as
// a plain string so a state a newer server adds still decodes and reaches the
// agent unchanged. Known states: in_progress, countdown, closing, partial,
// merged, checkpoint_merged, abandoned, off. Timestamps are RFC 3339 strings,
// like every other timestamp in this package.
type AutoMerge struct {
	State string `json:"state"`
	// Reason is a machine-readable code for why the branch still waits.
	Reason       string `json:"reason,omitempty"`
	ReasonDetail string `json:"reasonDetail,omitempty"`
	// NextCheckAt is when the server next evaluates the branch, in every state:
	// the abandonment date, the next check of an open pull request, now after a
	// hold was released. Only in state countdown is it the time of the merge.
	NextCheckAt string `json:"nextCheckAt,omitempty"`
	// HoldUntil is when the latest hold ends. It stays set after the hold has
	// ended, so compare it with the current time.
	HoldUntil  string `json:"holdUntil,omitempty"`
	HoldReason string `json:"holdReason,omitempty"`
	HeldByName string `json:"heldByName,omitempty"`
	// HoldSource is who set the hold: "agent" or "person".
	HoldSource string `json:"holdSource,omitempty"`
	// OpenPromises lists the texts of the promises still open on a partly
	// delivered intent.
	OpenPromises []string `json:"openPromises,omitempty"`
	// PartialUntil is when a partly delivered intent merges what shipped.
	PartialUntil        string `json:"partialUntil,omitempty"`
	SuccessorBranchID   string `json:"successorBranchId,omitempty"`
	SuccessorBranchName string `json:"successorBranchName,omitempty"`
}

type ListBranchesResponse struct {
	Branches []Branch `json:"branches"`
}

type CreateBranchRequest struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

type CreateBranchResponse struct {
	Branch Branch `json:"branch"`
}

type MergePreviewResponse struct {
	Additions     []any       `json:"additions"`
	Modifications []any       `json:"modifications"`
	Deletions     []any       `json:"deletions"`
	Stats         *MergeStats `json:"stats"`
	// AutoMerge is the branch's automatic-merge state; nil from an older
	// server.
	AutoMerge *AutoMerge `json:"autoMerge,omitempty"`
	// LinkProposals are the test-to-spec link proposals this merge turns into
	// links (status proposed or accepted). Empty from an older server.
	LinkProposals []LinkProposalBrief `json:"linkProposals,omitempty"`
}

// LinkProposalBrief is the compact form of a link proposal. The decoder drops
// the proposal and test/spec UUIDs on purpose: an agent identifies a link by
// the two sequence numbers.
type LinkProposalBrief struct {
	TestSeqNum        int    `json:"testSeqNum"`
	RequirementSeqNum int    `json:"requirementSeqNum"`
	Status            string `json:"status"`
}

type MergeStats struct {
	Additions     int `json:"additions"`
	Modifications int `json:"modifications"`
	Deletions     int `json:"deletions"`
	Conflicts     int `json:"conflicts"`
}

func (c *Client) ListBranches(ctx context.Context, productID string) (*ListBranchesResponse, error) {
	var resp ListBranchesResponse
	path := pathf("/v1/products/%s/branches", productID)
	if err := c.Do(ctx, "GET", path, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) CreateBranch(ctx context.Context, productID, name, description string) (*Branch, error) {
	body := CreateBranchRequest{Name: name, Description: description}
	var resp CreateBranchResponse
	path := pathf("/v1/products/%s/branches", productID)
	if err := c.Do(ctx, "POST", path, body, &resp); err != nil {
		return nil, err
	}
	return &resp.Branch, nil
}

func (c *Client) GetMergePreview(ctx context.Context, branchID string) (*MergePreviewResponse, error) {
	var resp MergePreviewResponse
	path := pathf("/v1/branches/%s/merge-preview", branchID)
	if err := c.Do(ctx, "GET", path, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// SetIntentHoldRequest is the body of POST /v1/branches/{branch_id}/intent-hold.
type SetIntentHoldRequest struct {
	// Until is when the hold ends (RFC 3339). Required unless Clear is set.
	Until string `json:"until,omitempty"`
	// Reason says why the branch is held. Required unless Clear is set.
	Reason string `json:"reason,omitempty"`
	// Clear releases the current hold instead of setting one.
	Clear bool `json:"clear,omitempty"`
	// Source records who set the hold: "agent" or "person".
	Source string `json:"source,omitempty"`
}

// SetIntentHoldResponse carries the branch's automatic-merge state after the
// hold was set or released.
type SetIntentHoldResponse struct {
	AutoMerge *AutoMerge `json:"autoMerge"`
}

// SetIntentHold sets or releases the hold that stops the automatic merge of an
// intent branch.
func (c *Client) SetIntentHold(ctx context.Context, branchID string, body SetIntentHoldRequest) (*SetIntentHoldResponse, error) {
	var resp SetIntentHoldResponse
	path := pathf("/v1/branches/%s/intent-hold", branchID)
	if err := c.Do(ctx, "POST", path, body, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
