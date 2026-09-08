package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"

	"github.com/qase-tms/tiden-mcp-server/internal/model"
)

type ListTestIdentitiesResponse struct {
	View       string               `json:"view"`
	Identities []model.TestIdentity `json:"identities"`
	Pagination *model.Pagination    `json:"pagination"`
}

// An explicit identity read never expands to a full legacy response. It is
// always one page; the caller decides whether/when to follow the continuation.
func (c *Client) ListTestIdentitiesPage(ctx context.Context, productID, branch string, pageSize int, pageToken string) (*ListTestIdentitiesResponse, error) {
	if pageSize == 0 {
		pageSize = 100
	}
	if pageSize < 1 || pageSize > 200 {
		return nil, fmt.Errorf("page_size must be between 1 and 200")
	}
	q := url.Values{"view": {"identity"}, "pagination.pageSize": {strconv.Itoa(pageSize)}}
	if branch != "" {
		q.Set("branch", branch)
	}
	if pageToken != "" {
		q.Set("pagination.pageToken", pageToken)
	}
	var wire struct {
		View       string             `json:"view"`
		Identities *[]json.RawMessage `json:"identities"`
		Pagination *model.Pagination  `json:"pagination"`
	}
	if err := c.Do(ctx, "GET", pathf("/v1/products/%s/tests", productID)+"?"+q.Encode(), nil, &wire); err != nil {
		return nil, err
	}
	if wire.View != "identity" {
		return nil, fmt.Errorf("server does not support the requested identity view; request detail explicitly for full tests")
	}
	if wire.Identities == nil || wire.Pagination == nil || wire.Pagination.TotalCount < 0 {
		return nil, fmt.Errorf("incomplete test identity response")
	}
	if len(*wire.Identities) > pageSize {
		return nil, fmt.Errorf("test identity response exceeds requested page size")
	}
	if wire.Pagination.NextPageToken != "" && (len(*wire.Identities) == 0 || wire.Pagination.NextPageToken == pageToken) {
		return nil, fmt.Errorf("non-progressing test identity pagination")
	}
	out := &ListTestIdentitiesResponse{View: "identity", Identities: make([]model.TestIdentity, 0, len(*wire.Identities)), Pagination: wire.Pagination}
	seen := map[string]bool{}
	for _, raw := range *wire.Identities {
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(raw, &fields); err != nil {
			return nil, err
		}
		for _, field := range []string{"id", "productId", "kind", "title", "tags", "signature", "framework", "filePath", "layer", "isAutomated", "branchStatus"} {
			if value, present := fields[field]; !present || string(value) == "null" {
				return nil, fmt.Errorf("test identity omitted required %s", field)
			}
		}
		var row model.TestIdentity
		if err := json.Unmarshal(raw, &row); err != nil {
			return nil, err
		}
		if row.ID == "" || row.ProductID != productID || (row.Kind != "suite" && row.Kind != "case") {
			return nil, fmt.Errorf("incomplete or foreign test identity")
		}
		if (row.Kind == "case" && (row.SeqNum == nil || *row.SeqNum <= 0)) || (row.Kind == "suite" && row.SeqNum != nil) {
			return nil, fmt.Errorf("invalid case/suite identity sequence")
		}
		if seen[row.ID] {
			return nil, fmt.Errorf("duplicate test identity %s", row.ID)
		}
		seen[row.ID] = true
		out.Identities = append(out.Identities, row)
	}
	return out, nil
}
