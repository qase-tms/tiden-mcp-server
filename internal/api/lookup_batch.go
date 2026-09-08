package api

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/qase-tms/tiden-mcp-server/internal/model"
)

func (c *Client) ResolveFeatureContextBatch(ctx context.Context, productID, branch string, plan model.LookupPlan, maxRequirements int) (*model.LookupBatchResult, error) {
	request := struct {
		Version         int                `json:"version"`
		Items           []model.LookupItem `json:"items"`
		Branch          string             `json:"branch,omitempty"`
		MaxRequirements int                `json:"maxRequirements,omitempty"`
	}{plan.Version, plan.Items, branch, maxRequirements}
	var result model.LookupBatchResult
	err := c.Do(ctx, "POST", pathf("/v1/products/%s/feature-context:batch", productID), request, &result)
	if err != nil {
		if errors.Is(err, ErrNotFound) || strings.Contains(err.Error(), "HTTP 501") {
			return nil, fmt.Errorf("batch lookup unavailable or branch not found: %w; verify the branch and upgrade the server for batch support; older servers require separate `tiden intent lookup <question> --format json` calls", err)
		}
		return nil, err
	}
	if result.Version != 1 || result.ProductID != productID || result.BranchID == "" || len(result.Items) != len(plan.Items) {
		return nil, fmt.Errorf("invalid batch lookup response: version, scope or item count mismatch")
	}
	failures := 0
	dictionary := map[string]bool{}
	for _, row := range result.Requirements {
		if row.ID == "" || dictionary[row.ID] {
			return nil, fmt.Errorf("invalid batch dictionary identity")
		}
		dictionary[row.ID] = true
	}
	for i, item := range result.Items {
		if item.ID != plan.Items[i].ID || item.Question != plan.Items[i].Question {
			return nil, fmt.Errorf("invalid batch lookup response: item mapping mismatch")
		}
		switch item.Status {
		case "ok", "no_match", "truncated":
		case "error":
			failures++
		default:
			return nil, fmt.Errorf("invalid batch item status %q", item.Status)
		}
		if len(item.CandidateRequirementIDs) > 80 {
			return nil, fmt.Errorf("batch candidate limit exceeded")
		}
		candidates := map[string]bool{}
		for _, id := range item.CandidateRequirementIDs {
			if candidates[id] || id == "" {
				return nil, fmt.Errorf("invalid candidate identity")
			}
			candidates[id] = true
		}
		for _, match := range item.Matches {
			if !candidates[match.RequirementID] || !dictionary[match.RequirementID] {
				return nil, fmt.Errorf("batch match missing candidate or dictionary row")
			}
		}
		for _, id := range item.CoverageGapIDs {
			if !candidates[id] {
				return nil, fmt.Errorf("batch coverage gap outside item candidates")
			}
		}
		if item.Status == "no_match" && len(candidates) > 0 {
			return nil, fmt.Errorf("no_match item contains candidates")
		}
	}
	expected := "complete"
	if failures == len(result.Items) {
		expected = "failed"
	} else if failures > 0 {
		expected = "partial"
	}
	if result.RetrievalStatus != expected {
		return nil, fmt.Errorf("batch retrieval status contradicts item statuses")
	}

	return &result, nil
}
