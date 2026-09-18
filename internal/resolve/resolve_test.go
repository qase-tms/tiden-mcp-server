package resolve

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/qase-tms/tiden-mcp-server/internal/api"
	"github.com/qase-tms/tiden-mcp-server/internal/config"
	"github.com/qase-tms/tiden-mcp-server/internal/repoconfig"
)

func noRequestsAllowed(t *testing.T) func(baseURL, token string) Client {
	return func(baseURL, token string) Client {
		t.Fatalf("unexpected client construction for baseURL=%s token=%s: fast path must make zero requests", baseURL, token)
		return nil
	}
}

func realClientFactory(timeout time.Duration) func(baseURL, token string) Client {
	return func(baseURL, token string) Client {
		return api.NewWithTimeout(baseURL, token, timeout)
	}
}

func TestResolve_FastPath_RepoWorkspaceAndEntry_ZeroRequests(t *testing.T) {
	store := &config.Store{
		FileVersion: 2,
		Workspaces: map[string]config.Entry{
			"ws-1": {BaseURL: "https://x.example", APIToken: "tok-1", Account: "a@x.dev", Name: "WS One"},
		},
	}
	deps := Deps{
		Store:     store,
		RepoFile:  &repoconfig.File{Exists: true, WorkspaceID: "ws-1"},
		NewClient: noRequestsAllowed(t),
	}
	got, err := Resolve(context.Background(), deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.BaseURL != "https://x.example" || got.APIToken != "tok-1" || got.WorkspaceID != "ws-1" ||
		got.WorkspaceName != "WS One" || got.Account != "a@x.dev" || got.Source != SourceRepoBinding {
		t.Errorf("got = %+v", got)
	}
}

func TestResolve_FlagBeatsEverythingElse(t *testing.T) {
	store := &config.Store{
		FileVersion: 2,
		Workspaces: map[string]config.Entry{
			"ws-env": {BaseURL: "https://env.example", APIToken: "tok-env"},
		},
	}
	deps := Deps{
		Store:           store,
		RepoFile:        &repoconfig.File{Exists: true, WorkspaceID: "ws-repo", ProductID: "prod-x"},
		RepositoryID:    "github.com/org/repo",
		FlagBaseURL:     "https://flag.example",
		FlagAPIToken:    "tok-flag",
		FlagWorkspaceID: "ws-flag",
		EnvAPIToken:     "tok-env",
		EnvWorkspaceID:  "ws-env",
		NewClient:       noRequestsAllowed(t),
	}
	got, err := Resolve(context.Background(), deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.BaseURL != "https://flag.example" || got.APIToken != "tok-flag" || got.WorkspaceID != "ws-flag" || got.Source != SourceFlag {
		t.Errorf("got = %+v, want flag values", got)
	}
}

func TestResolve_EnvBeatsRepoBindingWhenNoFlag(t *testing.T) {
	deps := Deps{
		Store:          &config.Store{},
		RepoFile:       &repoconfig.File{Exists: true, WorkspaceID: "ws-repo"},
		EnvAPIToken:    "tok-env",
		EnvWorkspaceID: "ws-env",
		EnvBaseURL:     "https://env.example",
		NewClient:      noRequestsAllowed(t),
	}
	got, err := Resolve(context.Background(), deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.WorkspaceID != "ws-env" || got.APIToken != "tok-env" || got.Source != SourceEnv {
		t.Errorf("got = %+v, want env values", got)
	}
}

func TestResolve_RepoBindingWorkspaceIDBeatsProductID(t *testing.T) {
	store := &config.Store{
		Workspaces: map[string]config.Entry{
			"ws-repo": {BaseURL: "https://x.example", APIToken: "tok-1", Name: "Repo WS"},
		},
	}
	deps := Deps{
		Store:     store,
		RepoFile:  &repoconfig.File{Exists: true, WorkspaceID: "ws-repo", ProductID: "prod-ignored"},
		NewClient: noRequestsAllowed(t),
	}
	got, err := Resolve(context.Background(), deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.WorkspaceID != "ws-repo" || got.Source != SourceRepoBinding {
		t.Errorf("got = %+v, want repo-binding to win over productId", got)
	}
}

// TestResolve_TransportErrorOnProductProbe_FallsThroughInsteadOfAborting is
// the F2 reviewer fix: startup now makes real API calls to resolve the
// workspace, so a transient network failure on one of those probes (server
// down, refused connection) must not abort the whole resolution with a raw
// transport error - old tiden-mcp-server binaries started fine even when
// the API was unreachable and only failed per tool call. The probe step
// must be treated as "unavailable" (skip it) so Resolve still returns a
// typed *Error the caller can print and exit 2 for, never a bare dial error
// that falls through main's generic "error: %s" / exit 1 path.
func TestResolve_TransportErrorOnProductProbe_FallsThroughInsteadOfAborting(t *testing.T) {
	// A server that is immediately closed: any request to its address is
	// refused (connection refused), simulating "unreachable".
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	deadURL := srv.URL
	srv.Close()

	store := &config.Store{
		Workspaces: map[string]config.Entry{
			"ws-other": {BaseURL: deadURL, APIToken: "tok-1"},
		},
	}
	deps := Deps{
		Store:     store,
		RepoFile:  &repoconfig.File{Exists: true, ProductID: "prod-x"},
		NewClient: realClientFactory(500 * time.Millisecond),
	}

	_, err := Resolve(context.Background(), deps)
	if err == nil {
		t.Fatal("expected an error (the product is not visible to the one login we have)")
	}
	rerr, ok := err.(*Error)
	if !ok {
		t.Fatalf("err = %T(%v), want *resolve.Error - a transport error on a probe step must not abort Resolve with a raw error", err, err)
	}
	if rerr.Code != "E2" {
		t.Errorf("Code = %q, want E2", rerr.Code)
	}
}

func TestResolve_ProductProbeBeatsRepositoryLookup(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		switch r.URL.Path {
		case "/v1/products/prod-y":
			_, _ = w.Write([]byte(`{"product":{"id":"prod-y","workspaceId":"ws-y","name":"Y"}}`))
		case "/v1/repositories/resolve":
			t.Fatalf("repository lookup must not run when a productId binding resolves the product probe")
		default:
			t.Fatalf("unexpected request: %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	store := &config.Store{
		Workspaces: map[string]config.Entry{
			"ws-y": {BaseURL: srv.URL, APIToken: "tok-1", Name: "Y Workspace"},
		},
	}
	deps := Deps{
		Store:        store,
		RepoFile:     &repoconfig.File{Exists: true, ProductID: "prod-y"},
		RepositoryID: "github.com/org/repo",
		NewClient:    realClientFactory(2 * time.Second),
	}
	got, err := Resolve(context.Background(), deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.WorkspaceID != "ws-y" || got.Source != SourceProductProbe {
		t.Errorf("got = %+v, want product-probe to ws-y", got)
	}
	if got.APIToken != "tok-1" {
		t.Errorf("APIToken = %q, want the known entry's token (fast path step 3)", got.APIToken)
	}
	if n := calls.Load(); n != 1 {
		t.Errorf("calls = %d, want 1 (only the product probe; entry present makes step 3 free)", n)
	}
}

func TestResolve_RepositoryLookupBeatsSingleLoginFallback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/repositories/resolve" {
			_, _ = w.Write([]byte(`{"candidates":[{"productId":"p1","productName":"P1","workspaceId":"ws-a","workspaceName":"A","source":"REPOSITORY_CANDIDATE_SOURCE_CI_SYNC"}]}`))
			return
		}
		t.Fatalf("unexpected request: %s", r.URL.Path)
	}))
	defer srv.Close()

	// Two entries in the store: if repository lookup were skipped, the
	// single-login fallback would see two distinct workspaces and be
	// ambiguous (E3). Repository lookup returning exactly one candidate
	// must win instead.
	store := &config.Store{
		Workspaces: map[string]config.Entry{
			"ws-a": {BaseURL: srv.URL, APIToken: "tok-1", Name: "A"},
			"ws-b": {BaseURL: srv.URL, APIToken: "tok-2", Name: "B"},
		},
	}
	deps := Deps{
		Store:        store,
		RepoFile:     &repoconfig.File{}, // no binding at all
		RepositoryID: "github.com/org/repo",
		NewClient:    realClientFactory(2 * time.Second),
	}
	got, err := Resolve(context.Background(), deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.WorkspaceID != "ws-a" || got.Source != SourceRepositoryLookup {
		t.Errorf("got = %+v, want repository-lookup to ws-a", got)
	}
}

func TestResolve_SingleLoginFallback_NoBindingNoRepository(t *testing.T) {
	store := &config.Store{
		Workspaces: map[string]config.Entry{
			"ws-only": {BaseURL: "https://x.example", APIToken: "tok-1", Name: "Only"},
		},
	}
	deps := Deps{
		Store:     store,
		RepoFile:  &repoconfig.File{},
		NewClient: noRequestsAllowed(t), // entry-derived candidates cost 0 requests
	}
	got, err := Resolve(context.Background(), deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.WorkspaceID != "ws-only" || got.Source != SourceSingleLogin {
		t.Errorf("got = %+v, want single-login fallback to ws-only", got)
	}
}

func TestResolve_501OnRepositoryLookup_NotRetriedAndStepSkipped(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/repositories/resolve" {
			calls.Add(1)
			w.WriteHeader(http.StatusNotImplemented)
			return
		}
		t.Fatalf("unexpected request: %s", r.URL.Path)
	}))
	defer srv.Close()

	store := &config.Store{
		Workspaces: map[string]config.Entry{
			"ws-only": {BaseURL: srv.URL, APIToken: "tok-1", Name: "Only"},
		},
	}
	deps := Deps{
		Store:        store,
		RepoFile:     &repoconfig.File{},
		RepositoryID: "github.com/org/repo",
		NewClient:    realClientFactory(2 * time.Second),
	}
	got, err := Resolve(context.Background(), deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.WorkspaceID != "ws-only" || got.Source != SourceSingleLogin {
		t.Errorf("got = %+v, want the step skipped and fallback to single-login", got)
	}
	if n := calls.Load(); n != 1 {
		t.Errorf("calls to /v1/repositories/resolve = %d, want 1 (501 must not be retried)", n)
	}
}

func TestResolve_ProductVisibleOnlyToSecondLogin_ResolvesToSecondsToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		switch {
		case r.URL.Path == "/v1/products/prod-z" && auth == "Bearer tok-a":
			w.WriteHeader(http.StatusForbidden)
		case r.URL.Path == "/v1/products/prod-z" && auth == "Bearer tok-b":
			_, _ = w.Write([]byte(`{"product":{"id":"prod-z","workspaceId":"ws-9999","name":"Z"}}`))
		case r.URL.Path == "/v1/workspaces" && auth == "Bearer tok-a":
			_, _ = w.Write([]byte(`{"workspaces":[{"id":"ws-1111","name":"Alpha"}]}`))
		case r.URL.Path == "/v1/workspaces" && auth == "Bearer tok-b":
			_, _ = w.Write([]byte(`{"workspaces":[{"id":"ws-2222","name":"Beta"},{"id":"ws-9999","name":"Z Workspace"}]}`))
		default:
			t.Fatalf("unexpected request: %s auth=%s", r.URL.Path, auth)
		}
	}))
	defer srv.Close()

	store := &config.Store{
		Workspaces: map[string]config.Entry{
			"ws-1111": {BaseURL: srv.URL, APIToken: "tok-a", Account: "a@x.dev"},
			"ws-2222": {BaseURL: srv.URL, APIToken: "tok-b", Account: "b@x.dev"},
		},
	}
	deps := Deps{
		Store:     store,
		RepoFile:  &repoconfig.File{Exists: true, ProductID: "prod-z"},
		NewClient: realClientFactory(2 * time.Second),
	}
	got, err := Resolve(context.Background(), deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.WorkspaceID != "ws-9999" || got.APIToken != "tok-b" || got.Source != SourceProductProbe {
		t.Errorf("got = %+v, want resolution via the second login's token", got)
	}
}

func TestResolve_AmbiguousRepositoryLookup_ReturnsE3(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"candidates":[
			{"productId":"p1","productName":"P1","workspaceId":"ws-a","workspaceName":"A","source":"REPOSITORY_CANDIDATE_SOURCE_CI_SYNC"},
			{"productId":"p2","productName":"P2","workspaceId":"ws-b","workspaceName":"B","source":"REPOSITORY_CANDIDATE_SOURCE_COMPONENT"}
		]}`))
	}))
	defer srv.Close()

	store := &config.Store{
		Workspaces: map[string]config.Entry{
			"ws-a": {BaseURL: srv.URL, APIToken: "tok-1"},
		},
	}
	deps := Deps{
		Store:        store,
		RepoFile:     &repoconfig.File{},
		RepositoryID: "github.com/org/repo",
		NewClient:    realClientFactory(2 * time.Second),
	}
	_, err := Resolve(context.Background(), deps)
	if err == nil {
		t.Fatal("expected an error")
	}
	rerr, ok := err.(*Error)
	if !ok {
		t.Fatalf("err = %T(%v), want *resolve.Error", err, err)
	}
	if rerr.Code != "E3" {
		t.Errorf("Code = %q, want E3", rerr.Code)
	}
	if !contains(rerr.Message, "This repository needs a choice:") || !contains(rerr.Message, "p1") || !contains(rerr.Message, "p2") {
		t.Errorf("Message = %q, want it to list both candidates", rerr.Message)
	}
	if !containsHint(rerr.Hints, "tiden workspace use <id>") || !containsHint(rerr.Hints, "tiden product bind --product-id <id>") {
		t.Errorf("Hints = %v, want both disambiguation hints", rerr.Hints)
	}
}

func TestResolve_AmbiguousSingleLoginFallback_ReturnsE3(t *testing.T) {
	store := &config.Store{
		Workspaces: map[string]config.Entry{
			"ws-a": {BaseURL: "https://x.example", APIToken: "tok-1", Name: "A"},
			"ws-b": {BaseURL: "https://x.example", APIToken: "tok-2", Name: "B"},
		},
	}
	deps := Deps{
		Store:     store,
		RepoFile:  &repoconfig.File{},
		NewClient: noRequestsAllowed(t),
	}
	_, err := Resolve(context.Background(), deps)
	rerr, ok := err.(*Error)
	if !ok {
		t.Fatalf("err = %T(%v), want *resolve.Error", err, err)
	}
	if rerr.Code != "E3" {
		t.Errorf("Code = %q, want E3", rerr.Code)
	}
}

func TestResolve_UnknownBoundWorkspace_ReturnsE1(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/workspaces" {
			_, _ = w.Write([]byte(`{"workspaces":[{"id":"ws-known","name":"Known"}]}`))
			return
		}
		t.Fatalf("unexpected request: %s", r.URL.Path)
	}))
	defer srv.Close()

	store := &config.Store{
		Workspaces: map[string]config.Entry{
			"ws-known": {BaseURL: srv.URL, APIToken: "tok-1"},
		},
	}
	deps := Deps{
		Store:     store,
		RepoFile:  &repoconfig.File{Exists: true, WorkspaceID: "ws-ghost"},
		NewClient: realClientFactory(2 * time.Second),
	}
	_, err := Resolve(context.Background(), deps)
	rerr, ok := err.(*Error)
	if !ok {
		t.Fatalf("err = %T(%v), want *resolve.Error", err, err)
	}
	if rerr.Code != "E1" {
		t.Errorf("Code = %q, want E1", rerr.Code)
	}
	want := "This repository is bound to workspace ws-ghost, and none of your logged-in accounts is a member."
	if rerr.Message != want {
		t.Errorf("Message = %q, want %q", rerr.Message, want)
	}
	if len(rerr.Hints) != 1 || rerr.Hints[0] != "tiden setup --workspace-id ws-ghost" {
		t.Errorf("Hints = %v", rerr.Hints)
	}
}

func TestResolve_NotLoggedIn_ReturnsSetupHint(t *testing.T) {
	deps := Deps{
		Store:     &config.Store{},
		RepoFile:  &repoconfig.File{},
		NewClient: noRequestsAllowed(t),
	}
	_, err := Resolve(context.Background(), deps)
	rerr, ok := err.(*Error)
	if !ok {
		t.Fatalf("err = %T(%v), want *resolve.Error", err, err)
	}
	if rerr.Code != "NO_LOGIN" {
		t.Errorf("Code = %q, want NO_LOGIN", rerr.Code)
	}
	if rerr.Message != "Not logged in. Run `tiden setup`." {
		t.Errorf("Message = %q", rerr.Message)
	}
}

func TestResolve_ProductNotVisible_ReturnsE2(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	store := &config.Store{
		Workspaces: map[string]config.Entry{
			"ws-1": {BaseURL: srv.URL, APIToken: "tok-1"},
		},
	}
	deps := Deps{
		Store:     store,
		RepoFile:  &repoconfig.File{Exists: true, ProductID: "prod-hidden"},
		NewClient: realClientFactory(2 * time.Second),
	}
	_, err := Resolve(context.Background(), deps)
	rerr, ok := err.(*Error)
	if !ok {
		t.Fatalf("err = %T(%v), want *resolve.Error", err, err)
	}
	if rerr.Code != "E2" {
		t.Errorf("Code = %q, want E2", rerr.Code)
	}
	want := "Product prod-hidden is not visible to any logged-in account."
	if rerr.Message != want {
		t.Errorf("Message = %q, want %q", rerr.Message, want)
	}
	if len(rerr.Hints) != 1 || rerr.Hints[0] != "tiden setup" {
		t.Errorf("Hints = %v", rerr.Hints)
	}
}

// TestResolve_OverrideToken_SeveralWorkspaces_ResolvesWithEmptyWorkspace is
// the F6m ladder amendment, mirroring the CLI: an explicitly given token
// (flag/env) must never be refused just because no single workspace could
// be chosen for it. The pre-TIDEN-68 server started the same way with a
// token-only env; list_products already reports "workspace_id is required"
// per call when neither the argument nor this default is set, so deferring
// the ambiguity to call time (rather than erroring at startup) loses
// nothing.
func TestResolve_OverrideToken_SeveralWorkspaces_ResolvesWithEmptyWorkspace(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/workspaces" {
			t.Fatalf("unexpected request: %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"workspaces":[{"id":"ws-a","name":"A"},{"id":"ws-b","name":"B"}]}`))
	}))
	defer srv.Close()

	deps := Deps{
		Store:       &config.Store{},
		RepoFile:    &repoconfig.File{},
		EnvAPIToken: "tok-env",
		EnvBaseURL:  srv.URL,
		NewClient:   realClientFactory(2 * time.Second),
	}
	got, err := Resolve(context.Background(), deps)
	if err != nil {
		t.Fatalf("unexpected error: %v (an override token must never be refused)", err)
	}
	if got.WorkspaceID != "" {
		t.Errorf("WorkspaceID = %q, want empty (several workspaces, none chosen)", got.WorkspaceID)
	}
	if got.BaseURL != srv.URL || got.APIToken != "tok-env" || got.Source != SourceEnv {
		t.Errorf("got = %+v, want the override baseUrl/token/source preserved", got)
	}
}

// TestResolve_OverrideToken_NoWorkspaces_ResolvesWithEmptyWorkspace covers
// the "decodes to nothing" half of the same amendment: ListWorkspaces
// succeeds but lists none.
func TestResolve_OverrideToken_NoWorkspaces_ResolvesWithEmptyWorkspace(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"workspaces":[]}`))
	}))
	defer srv.Close()

	deps := Deps{
		Store:       &config.Store{},
		RepoFile:    &repoconfig.File{},
		EnvAPIToken: "tok-env",
		EnvBaseURL:  srv.URL,
		NewClient:   realClientFactory(2 * time.Second),
	}
	got, err := Resolve(context.Background(), deps)
	if err != nil {
		t.Fatalf("unexpected error: %v (an override token must never be refused)", err)
	}
	if got.WorkspaceID != "" || got.BaseURL != srv.URL || got.APIToken != "tok-env" || got.Source != SourceEnv {
		t.Errorf("got = %+v", got)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || (len(substr) > 0 && indexOf(s, substr) >= 0))
}

func indexOf(s, substr string) int {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

func containsHint(hints []string, want string) bool {
	for _, h := range hints {
		if h == want {
			return true
		}
	}
	return false
}
