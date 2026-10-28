package api

import (
	"context"
	"net/url"

	"github.com/qase-tms/tiden-mcp-server/internal/model"
)

type ListReleasesResponse struct {
	Releases   []model.Release  `json:"releases"`
	Pagination model.Pagination `json:"pagination"`
}

// releasePageSize stays under the server's 200-row page ceiling.
const releasePageSize = "100"

// ListReleases returns every release of a product, newest first. It follows
// pagination.nextPageToken across pages (capped at 100 pages); a bare
// `pageSize` query parameter is ignored by the gateway, which then answers
// with its default page of 20. NextPageToken is set only when the cap was hit.
func (c *Client) ListReleases(ctx context.Context, productID string) (*ListReleasesResponse, error) {
	var out ListReleasesResponse
	pageToken := ""
	for pages := 0; pages < 100; pages++ {
		var resp ListReleasesResponse
		q := url.Values{}
		q.Set("pagination.pageSize", releasePageSize)
		if pageToken != "" {
			q.Set("pagination.pageToken", pageToken)
		}
		path := pathf("/v1/products/%s/releases", productID) + "?" + q.Encode()
		if err := c.Do(ctx, "GET", path, nil, &resp); err != nil {
			return nil, err
		}
		out.Releases = append(out.Releases, resp.Releases...)
		out.Pagination.TotalCount = resp.Pagination.TotalCount
		pageToken = resp.Pagination.NextPageToken
		if pageToken == "" || len(resp.Releases) == 0 {
			out.Pagination.NextPageToken = ""
			return &out, nil
		}
	}
	out.Pagination.NextPageToken = pageToken
	return &out, nil
}
