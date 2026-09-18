// Package resolve implements tiden-mcp-server's half of the TIDEN-68
// multi-workspace resolution ladder: given the parsed global config store,
// the repo-local binding, the repository's canonical identity, and any
// flag/env overrides, it decides which (baseUrl, apiToken, workspaceId) the
// server should run with.
//
// This is a READ-ONLY mirror of the CLI's ladder (internal/resolve in
// tiden-cli): it never prompts (there is no TTY for an MCP server, no
// Asker), never writes the global config file or the repo-local file, and
// never migrates a v1 file to v2. When nothing in the ladder resolves, it
// returns a typed *Error (one of the E1-E3/E2/NO_LOGIN codes) that the
// caller (cmd/tiden-mcp-server) prints verbatim to stderr and exits 2 for -
// never a silent empty token.
package resolve

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/qase-tms/tiden-mcp-server/internal/api"
	"github.com/qase-tms/tiden-mcp-server/internal/config"
	"github.com/qase-tms/tiden-mcp-server/internal/model"
	"github.com/qase-tms/tiden-mcp-server/internal/repoconfig"
)

// Source labels how a Resolved value was decided. Mirrors tiden-cli's
// config.Source constants (minus SourceFile, which has no read-only-ladder
// equivalent here).
const (
	SourceFlag             = "flag"
	SourceEnv              = "env"
	SourceRepoBinding      = "repo-binding"
	SourceProductProbe     = "product-probe"
	SourceRepositoryLookup = "repository-lookup"
	SourceSingleLogin      = "single-login"
)

// Client is the subset of api.Client the ladder needs to probe a login. A
// real *api.Client satisfies it; tests may fake it, though this package's
// own tests exercise the real client against httptest servers per the
// definition of done.
type Client interface {
	ListWorkspaces(ctx context.Context) (*api.ListWorkspacesResponse, error)
	GetProduct(ctx context.Context, id string) (*model.Product, error)
	ResolveRepository(ctx context.Context, repository string) (*api.ResolveRepositoryResponse, error)
}

// Deps bundles everything the ladder reads. Nothing here is mutated and
// nothing is persisted.
type Deps struct {
	Store    *config.Store
	RepoFile *repoconfig.File
	// RepositoryID is the canonical "host/path" identity of the origin
	// remote above the server's working directory (gitremote.OriginIdentity),
	// or "" when there is no git repository or no origin remote - in which
	// case the repository-lookup step is skipped entirely.
	RepositoryID string

	FlagBaseURL, FlagAPIToken, FlagWorkspaceID string
	EnvBaseURL, EnvAPIToken, EnvWorkspaceID    string

	// NewClient builds a Client for one login (baseUrl, apiToken). Required.
	NewClient func(baseURL, token string) Client
}

// Resolved is the outcome of a successful resolution.
type Resolved struct {
	BaseURL       string
	APIToken      string
	WorkspaceID   string
	WorkspaceName string
	Account       string
	Source        string
}

// Error is returned when the ladder cannot resolve. Code is one of
// "NO_LOGIN", "E1", "E2", "E3" (or "E5" for the rare case an override token
// itself resolves to nothing - not part of the Linear-numbered set, see
// resolveOverrideToken). The caller prints Message then each Hint on its
// own line to stderr and exits 2.
type Error struct {
	Code    string
	Message string
	Hints   []string
}

func (e *Error) Error() string { return e.Message }

// Resolve runs the ladder. See the package doc and TIDEN-68's ladder
// specification for the exact order: flag > env > repo binding > product
// probe > repository lookup > single login.
func Resolve(ctx context.Context, d Deps) (*Resolved, error) {
	store := d.Store
	if store == nil {
		store = &config.Store{}
	}
	repoFile := d.RepoFile
	if repoFile == nil {
		repoFile = &repoconfig.File{}
	}

	token, tokenSrc := pick(d.FlagAPIToken, d.EnvAPIToken)
	ws, wsSrc := pick(d.FlagWorkspaceID, d.EnvWorkspaceID)
	baseURL, _ := pick(d.FlagBaseURL, d.EnvBaseURL)

	if token != "" {
		return resolveOverrideToken(ctx, d, store, repoFile, token, tokenSrc, baseURL, ws)
	}
	if ws != "" {
		// "a workspace id override without a token selects that entry" -
		// skip straight to step 3 (token for the target workspace).
		return resolveTokenForWorkspace(ctx, d, store, ws, wsSrc)
	}

	if !store.HasAnyLogin() {
		return nil, &Error{Code: "NO_LOGIN", Message: "Not logged in. Run `tiden setup`."}
	}

	targetWS, source, err := determineTargetWorkspace(ctx, d, store, repoFile)
	if err != nil {
		return nil, err
	}
	return resolveTokenForWorkspace(ctx, d, store, targetWS, source)
}

// determineTargetWorkspace runs steps (a)-(d) of the ladder: repo binding
// workspaceId, else repo binding productId (product probe), else repository
// lookup, else the sole workspace across every login.
func determineTargetWorkspace(ctx context.Context, d Deps, store *config.Store, repoFile *repoconfig.File) (string, string, error) {
	if repoFile.WorkspaceID != "" {
		return repoFile.WorkspaceID, SourceRepoBinding, nil
	}

	if repoFile.ProductID != "" {
		wsID, err := probeProduct(ctx, d, store, repoFile.ProductID)
		if err != nil {
			return "", "", err
		}
		if wsID != "" {
			return wsID, SourceProductProbe, nil
		}
		return "", "", &Error{
			Code:    "E2",
			Message: fmt.Sprintf("Product %s is not visible to any logged-in account.", repoFile.ProductID),
			Hints:   []string{"tiden setup"},
		}
	}

	if d.RepositoryID != "" {
		candidates, unavailable, err := lookupRepository(ctx, d, store)
		if err != nil {
			return "", "", err
		}
		if !unavailable {
			switch {
			case len(candidates) == 1:
				return candidates[0].WorkspaceID, SourceRepositoryLookup, nil
			case len(candidates) > 1:
				return "", "", e3FromRepositoryCandidates(candidates)
			}
		}
	}

	candidates, err := gatherAllLoginWorkspaces(ctx, d, store)
	if err != nil {
		return "", "", err
	}
	switch len(candidates) {
	case 1:
		return candidates[0].id, SourceSingleLogin, nil
	case 0:
		return "", "", &Error{Code: "NO_LOGIN", Message: "No workspace found for any logged-in account. Run `tiden setup`."}
	default:
		return "", "", e3FromWorkspaceCandidates(candidates)
	}
}

// resolveTokenForWorkspace is step 3 ("Token for the target"): use the
// known entry with zero requests, else find a login whose ListWorkspaces
// lists wsID.
func resolveTokenForWorkspace(ctx context.Context, d Deps, store *config.Store, wsID, source string) (*Resolved, error) {
	if e, ok := store.Workspaces[wsID]; ok {
		return &Resolved{
			BaseURL: e.BaseURL, APIToken: e.APIToken,
			WorkspaceID: wsID, WorkspaceName: e.Name, Account: e.Account,
			Source: source,
		}, nil
	}

	for _, login := range store.DistinctLogins() {
		client := d.NewClient(login.BaseURL, login.APIToken)
		resp, err := client.ListWorkspaces(ctx)
		if err != nil {
			// A dead or misbehaving login simply doesn't contribute; try
			// the next one. Only running out of logins is an error.
			continue
		}
		for _, w := range resp.Workspaces {
			if w.ID == wsID {
				return &Resolved{
					BaseURL: login.BaseURL, APIToken: login.APIToken,
					WorkspaceID: wsID, WorkspaceName: w.Name, Account: login.Account,
					Source: source,
				}, nil
			}
		}
	}

	return nil, &Error{
		Code:    "E1",
		Message: fmt.Sprintf("This repository is bound to workspace %s, and none of your logged-in accounts is a member.", wsID),
		Hints:   []string{"tiden setup --workspace-id " + wsID},
	}
}

// probeProduct calls GetProduct(productID) with each distinct login in
// order, returning the first success's WorkspaceID. A login that cannot see
// the product (401/403/404) is skipped; any other error aborts the whole
// resolution (it is not "not visible", it is unknown).
func probeProduct(ctx context.Context, d Deps, store *config.Store, productID string) (string, error) {
	for _, login := range store.DistinctLogins() {
		client := d.NewClient(login.BaseURL, login.APIToken)
		p, err := client.GetProduct(ctx, productID)
		if err == nil {
			return p.WorkspaceID, nil
		}
		if errors.Is(err, api.ErrUnauthorized) || errors.Is(err, api.ErrForbidden) || errors.Is(err, api.ErrNotFound) {
			continue
		}
		return "", err
	}
	return "", nil
}

// lookupRepository calls ResolveRepository with each distinct login,
// merging candidates (deduped by ProductID). unavailable=true means an
// ErrUnimplemented was seen (old server without the route): the caller
// skips the whole step rather than treating an empty result as "no match".
func lookupRepository(ctx context.Context, d Deps, store *config.Store) (candidates []model.RepositoryCandidate, unavailable bool, err error) {
	seen := map[string]bool{}
	for _, login := range store.DistinctLogins() {
		client := d.NewClient(login.BaseURL, login.APIToken)
		resp, cerr := client.ResolveRepository(ctx, d.RepositoryID)
		if cerr != nil {
			if errors.Is(cerr, api.ErrUnimplemented) {
				return nil, true, nil
			}
			if errors.Is(cerr, api.ErrUnauthorized) {
				continue
			}
			return nil, false, cerr
		}
		for _, c := range resp.Candidates {
			if seen[c.ProductID] {
				continue
			}
			seen[c.ProductID] = true
			candidates = append(candidates, c)
		}
	}
	return candidates, false, nil
}

type wsCandidate struct {
	id, name, account string
}

// gatherAllLoginWorkspaces is step (d)'s fallback: every workspace already
// known via a store entry (zero requests), plus - for any loose login not
// yet placed under a workspace - whatever ListWorkspaces reports.
func gatherAllLoginWorkspaces(ctx context.Context, d Deps, store *config.Store) ([]wsCandidate, error) {
	seen := map[string]bool{}
	var out []wsCandidate

	ids := make([]string, 0, len(store.Workspaces))
	for id := range store.Workspaces {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		if seen[id] {
			continue
		}
		seen[id] = true
		e := store.Workspaces[id]
		out = append(out, wsCandidate{id: id, name: e.Name, account: e.Account})
	}

	for _, login := range store.Loose {
		client := d.NewClient(login.BaseURL, login.APIToken)
		resp, err := client.ListWorkspaces(ctx)
		if err != nil {
			continue
		}
		for _, w := range resp.Workspaces {
			if seen[w.ID] {
				continue
			}
			seen[w.ID] = true
			out = append(out, wsCandidate{id: w.ID, name: w.Name, account: login.Account})
		}
	}
	return out, nil
}

// resolveOverrideToken implements step 0: a flag/env API token is THE
// token. The target workspace is found by walking, in order: an explicit
// flag/env workspace id, the repo binding's workspaceId, a product probe
// against the repo binding's productId, a repository lookup, and finally
// the token's sole workspace.
func resolveOverrideToken(ctx context.Context, d Deps, store *config.Store, repoFile *repoconfig.File, token, tokenSrc, baseURL, wsOverride string) (*Resolved, error) {
	if baseURL == "" {
		if l := findLoginByToken(store, token); l != nil {
			baseURL = l.BaseURL
		}
	}
	if baseURL == "" {
		return nil, fmt.Errorf("an API token was given without a base URL: set --base-url or TIDEN_BASE_URL")
	}

	account := accountForToken(store, token)

	build := func(wsID, wsName string) *Resolved {
		if wsName == "" {
			if e, ok := store.Workspaces[wsID]; ok {
				wsName = e.Name
			}
		}
		return &Resolved{BaseURL: baseURL, APIToken: token, WorkspaceID: wsID, WorkspaceName: wsName, Account: account, Source: tokenSrc}
	}

	if wsOverride != "" {
		return build(wsOverride, ""), nil
	}
	if repoFile.WorkspaceID != "" {
		return build(repoFile.WorkspaceID, ""), nil
	}

	// Only the remaining steps need a live client - build it lazily so the
	// direct token+workspace overrides above stay a true zero-request path.
	client := d.NewClient(baseURL, token)

	if repoFile.ProductID != "" {
		p, err := client.GetProduct(ctx, repoFile.ProductID)
		switch {
		case err == nil:
			return build(p.WorkspaceID, ""), nil
		case errors.Is(err, api.ErrUnauthorized), errors.Is(err, api.ErrForbidden), errors.Is(err, api.ErrNotFound):
			// fall through to the next step
		default:
			return nil, err
		}
	}

	if d.RepositoryID != "" {
		resp, err := client.ResolveRepository(ctx, d.RepositoryID)
		switch {
		case err == nil:
			candidates := dedupeByProductID(resp.Candidates)
			switch len(candidates) {
			case 1:
				return build(candidates[0].WorkspaceID, candidates[0].WorkspaceName), nil
			default:
				if len(candidates) > 1 {
					return nil, e3FromRepositoryCandidates(candidates)
				}
			}
		case errors.Is(err, api.ErrUnimplemented), errors.Is(err, api.ErrUnauthorized):
			// fall through to the next step
		default:
			return nil, err
		}
	}

	resp, err := client.ListWorkspaces(ctx)
	if err != nil {
		if errors.Is(err, api.ErrUnauthorized) {
			return nil, &Error{Code: "E5", Message: "API token was rejected.", Hints: []string{"tiden setup"}}
		}
		return nil, err
	}
	switch len(resp.Workspaces) {
	case 1:
		return build(resp.Workspaces[0].ID, resp.Workspaces[0].Name), nil
	case 0:
		return nil, &Error{Code: "E5", Message: "API token did not resolve to any workspace.", Hints: []string{"tiden setup"}}
	default:
		candidates := make([]wsCandidate, len(resp.Workspaces))
		for i, w := range resp.Workspaces {
			candidates[i] = wsCandidate{id: w.ID, name: w.Name, account: account}
		}
		return nil, e3FromWorkspaceCandidates(candidates)
	}
}

func e3FromRepositoryCandidates(cs []model.RepositoryCandidate) *Error {
	lines := make([]string, len(cs))
	for i, c := range cs {
		lines[i] = fmt.Sprintf("  %s (%s) in workspace %s (%s)", c.ProductName, c.ProductID, c.WorkspaceName, c.WorkspaceID)
	}
	return &Error{
		Code:    "E3",
		Message: "This repository needs a choice:\n" + strings.Join(lines, "\n"),
		Hints:   []string{"tiden workspace use <id>", "tiden product bind --product-id <id>"},
	}
}

func e3FromWorkspaceCandidates(cs []wsCandidate) *Error {
	lines := make([]string, len(cs))
	for i, c := range cs {
		name := c.name
		if name == "" {
			name = c.id
		}
		account := c.account
		if account == "" {
			account = "unknown account"
		}
		lines[i] = fmt.Sprintf("  %s (%s) — %s", name, c.id, account)
	}
	return &Error{
		Code:    "E3",
		Message: "This repository needs a choice:\n" + strings.Join(lines, "\n"),
		Hints:   []string{"tiden workspace use <id>", "tiden product bind --product-id <id>"},
	}
}

func dedupeByProductID(cs []model.RepositoryCandidate) []model.RepositoryCandidate {
	seen := map[string]bool{}
	var out []model.RepositoryCandidate
	for _, c := range cs {
		if seen[c.ProductID] {
			continue
		}
		seen[c.ProductID] = true
		out = append(out, c)
	}
	return out
}

func findLoginByToken(store *config.Store, token string) *config.Login {
	for _, l := range store.DistinctLogins() {
		if l.APIToken == token {
			ll := l
			return &ll
		}
	}
	return nil
}

func accountForToken(store *config.Store, token string) string {
	for _, l := range store.DistinctLogins() {
		if l.APIToken == token {
			return l.Account
		}
	}
	return ""
}

// pick returns (flag, "flag") when flag is set, else (env, "env") when env
// is set, else ("", "").
func pick(flag, env string) (string, string) {
	if flag != "" {
		return flag, SourceFlag
	}
	if env != "" {
		return env, SourceEnv
	}
	return "", ""
}
