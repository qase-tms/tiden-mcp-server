package api

import (
	"context"
	"errors"
	"fmt"
	"net/url"

	"github.com/qase-tms/tiden-mcp-server/internal/model"
)

type ResolveRepositoryResponse struct {
	Candidates []model.RepositoryCandidate `json:"candidates"`
}

// ResolveRepository looks up which product(s)/workspace(s) a repository is
// already known under (GET /v1/repositories/resolve?repository=...), so the
// resolution ladder (internal/resolve) can bind a repository without asking
// a human to pick blind. An unknown repository is not an error: the server
// answers 200 with an empty Candidates slice.
//
// A server that predates this route answers with a plain 404, which Do
// would otherwise map to ErrNotFound indistinguishably from a "repository
// not found" business response — but this endpoint never returns 404 for
// that case (unknown repository is 200 + empty candidates), so any 404 here
// means an old server without the route. Do itself maps HTTP 501 to
// ErrUnimplemented directly. Callers should treat ErrUnimplemented as "skip
// this resolution step", not as a hard failure.
func (c *Client) ResolveRepository(ctx context.Context, repository string) (*ResolveRepositoryResponse, error) {
	var resp ResolveRepositoryResponse
	path := "/v1/repositories/resolve?repository=" + url.QueryEscape(repository)
	if err := c.Do(ctx, "GET", path, nil, &resp); err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, fmt.Errorf("%w: %v", ErrUnimplemented, err)
		}
		return nil, err
	}
	return &resp, nil
}
