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
// "NO_LOGIN", "E1", "E2", "E3" - all reserved for the stored-login path
// (determineTargetWorkspace). An explicit flag/env token (resolveOverrideToken)
// never produces one: per D14 it is local-only and always resolves, with
// WorkspaceID left empty when neither the flag/env workspace id nor the repo
// binding names one. The caller prints Message then each Hint on its own
// line to stderr and exits 2.
type Error struct {
	Code    string
	Message string
	Hints   []string
}

func (e *Error) Error() string { return e.Message }

// Resolve runs the ladder. An override token (flag/env) is local-only (D14):
// its workspace is the flag/env workspace id, else the repo binding's
// workspaceId, else empty - no network call. Absent an override token, the
// stored-login path applies the full ladder: repo binding > product probe >
// repository lookup > single login. See the package doc and TIDEN-68's
// ladder specification for details.
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
		wsID, unreachable := probeProduct(ctx, d, store, repoFile.ProductID)
		if wsID != "" {
			return wsID, SourceProductProbe, nil
		}
		msg := fmt.Sprintf("Product %s is not visible to any logged-in account.", repoFile.ProductID)
		if unreachable {
			msg += " (Some accounts could not be reached to check.)"
		}
		return "", "", &Error{Code: "E2", Message: msg, Hints: []string{"tiden setup"}}
	}

	if d.RepositoryID != "" {
		candidates, unavailable := lookupRepository(ctx, d, store)
		if !unavailable {
			switch {
			case len(candidates) == 1:
				return candidates[0].WorkspaceID, SourceRepositoryLookup, nil
			case len(candidates) > 1:
				return "", "", e3FromRepositoryCandidates(candidates)
			}
		}
	}

	candidates := gatherAllLoginWorkspaces(ctx, d, store)
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
// the product (401/403/404) is skipped; so is one whose server could not be
// reached at all (transport error, 5xx, rate limit) - either way that login
// simply has no answer, and this step's unavailability shows up only as a
// less certain "not visible" from the caller (E2), never as a startup
// crash. unreachable reports whether at least one login hit that second,
// non-clean-negative case, so the caller can mention it in E2's message.
func probeProduct(ctx context.Context, d Deps, store *config.Store, productID string) (wsID string, unreachable bool) {
	for _, login := range store.DistinctLogins() {
		client := d.NewClient(login.BaseURL, login.APIToken)
		p, err := client.GetProduct(ctx, productID)
		if err == nil {
			return p.WorkspaceID, false
		}
		if !isCleanNegative(err) {
			unreachable = true
		}
	}
	return "", unreachable
}

// lookupRepository calls ResolveRepository with each distinct login,
// merging candidates (deduped by ProductID). unavailable=true means an
// ErrUnimplemented was seen (old server without the route): the caller
// skips the whole step rather than treating an empty result as "no match".
// Any other failure for one login - unauthorized, forbidden, not found, a
// transport error, a 5xx - just means that login has no answer; the loop
// keeps trying the rest instead of aborting the whole resolution.
func lookupRepository(ctx context.Context, d Deps, store *config.Store) (candidates []model.RepositoryCandidate, unavailable bool) {
	seen := map[string]bool{}
	for _, login := range store.DistinctLogins() {
		client := d.NewClient(login.BaseURL, login.APIToken)
		resp, err := client.ResolveRepository(ctx, d.RepositoryID)
		if err != nil {
			if errors.Is(err, api.ErrUnimplemented) {
				return nil, true
			}
			continue
		}
		for _, c := range resp.Candidates {
			if seen[c.ProductID] {
				continue
			}
			seen[c.ProductID] = true
			candidates = append(candidates, c)
		}
	}
	return candidates, false
}

// isCleanNegative reports whether err is a definitive "no" from the server
// (the login is dead, or it plainly cannot see the resource) rather than a
// sign the server could not be reached or is misbehaving.
func isCleanNegative(err error) bool {
	return errors.Is(err, api.ErrUnauthorized) || errors.Is(err, api.ErrForbidden) || errors.Is(err, api.ErrNotFound) || errors.Is(err, api.ErrUnimplemented)
}

type wsCandidate struct {
	id, name, account string
}

// gatherAllLoginWorkspaces is step (d)'s fallback: every workspace already
// known via a store entry (zero requests), plus - for any loose login not
// yet placed under a workspace - whatever ListWorkspaces reports. A loose
// login whose server cannot be reached, or that rejects the request, simply
// contributes nothing - it is never treated as a hard failure.
func gatherAllLoginWorkspaces(ctx context.Context, d Deps, store *config.Store) []wsCandidate {
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
	return out
}

// resolveOverrideToken implements step 0: a flag/env API token is THE
// token, used exactly as given. D14 (revised): override-token mode is
// LOCAL-ONLY - the server makes NO network call to pick a workspace for
// it. The workspace is the flag/env workspace id when given, else the repo
// binding's workspaceId, else left empty (zero requests either way).
//
// An explicitly given token is never refused for lack of a workspace: when
// neither signal is present, Resolve still succeeds, with WorkspaceID left
// empty. This is what the server did before TIDEN-68 (a token-only env just
// started), it removes startup network calls for the common token-only
// deployment, and each tool call that actually needs a workspace already
// reports "workspace_id is required" when neither its argument nor this
// default is set - so nothing is lost by deferring the ambiguity past
// startup. No product probe, no repository lookup, no ListWorkspaces: those
// only run for the stored-login path (determineTargetWorkspace), which also
// keeps E1/E2/E3/NO_LOGIN unchanged. This function never returns one.
func resolveOverrideToken(_ context.Context, _ Deps, store *config.Store, repoFile *repoconfig.File, token, tokenSrc, baseURL, wsOverride string) (*Resolved, error) {
	if baseURL == "" {
		if l := findLoginByToken(store, token); l != nil {
			baseURL = l.BaseURL
		}
	}
	if baseURL == "" {
		return nil, fmt.Errorf("an API token was given without a base URL: set --base-url or TIDEN_BASE_URL")
	}

	account := accountForToken(store, token)

	build := func(wsID string) *Resolved {
		wsName := ""
		if e, ok := store.Workspaces[wsID]; ok {
			wsName = e.Name
		}
		return &Resolved{BaseURL: baseURL, APIToken: token, WorkspaceID: wsID, WorkspaceName: wsName, Account: account, Source: tokenSrc}
	}

	if wsOverride != "" {
		return build(wsOverride), nil
	}
	if repoFile.WorkspaceID != "" {
		return build(repoFile.WorkspaceID), nil
	}
	return build(""), nil
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
