package api

import (
	"context"
	"encoding/hex"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/qase-tms/tiden-mcp-server/internal/model"
)

type ListRequirementIdentitiesResponse struct {
	View       string                      `json:"view"`
	Identities []model.RequirementIdentity `json:"identities"`
	Pagination *model.Pagination           `json:"pagination"`
}

// ListRequirementIdentitiesPage is always one bounded page. An old server that
// ignores view must not silently expand the response into full document bodies.
func (c *Client) ListRequirementIdentitiesPage(ctx context.Context, productID, branch string, pageSize int, pageToken string) (*ListRequirementIdentitiesResponse, error) {
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
	var response ListRequirementIdentitiesResponse
	if err := c.Do(ctx, "GET", pathf("/v1/products/%s/requirements", productID)+"?"+q.Encode(), nil, &response); err != nil {
		return nil, err
	}
	if response.View != "identity" {
		return nil, fmt.Errorf("server does not support the requested identity view; use detail explicitly if full bodies are needed")
	}
	if response.Identities == nil || response.Pagination == nil {
		return nil, fmt.Errorf("incomplete identity response")
	}
	for _, row := range response.Identities {
		if row.ID == "" || row.ProductID != productID || row.Sources == nil {
			return nil, fmt.Errorf("incomplete or foreign requirement identity")
		}
		for _, hash := range []string{row.ContentSHA256, row.TrimmedContentSHA256} {
			decoded, err := hex.DecodeString(hash)
			if err != nil || len(decoded) != 32 || hash != strings.ToLower(hash) {
				return nil, fmt.Errorf("missing or invalid requirement identity hash")
			}
		}
	}
	return &response, nil
}
