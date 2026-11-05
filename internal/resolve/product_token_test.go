package resolve

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/qase-tms/tiden-mcp-server/internal/config"
	"github.com/qase-tms/tiden-mcp-server/internal/repoconfig"
)

// A product API token belongs to one product: the CLI stores it as the one
// entry for that product's workspace, with productId. The resolver accepts
// it from the store and never sends it a ListWorkspaces membership probe.

func productTokenStore() *config.Store {
	return &config.Store{
		FileVersion: 2,
		Workspaces: map[string]config.Entry{
			"ws-1": {BaseURL: "https://x.example", APIToken: "tok-p", Account: "CI (product token)", Name: "One", ProductID: "P1"},
		},
	}
}

func TestResolve_ProductBindingMatchesStoredProductToken_ZeroRequests(t *testing.T) {
	got, err := Resolve(context.Background(), Deps{
		Store:     productTokenStore(),
		RepoFile:  &repoconfig.File{Exists: true, ProductID: "P1"},
		NewClient: noRequestsAllowed(t),
	})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if got.WorkspaceID != "ws-1" || got.APIToken != "tok-p" || got.ProductID != "P1" || got.Account != "CI (product token)" || got.Source != SourceProductProbe {
		t.Errorf("got = %+v, want the stored product token for ws-1/P1", got)
	}
}

func TestResolve_OverrideTokenKnownAsProductToken_UsesItsWorkspace(t *testing.T) {
	got, err := Resolve(context.Background(), Deps{
		Store:       productTokenStore(),
		RepoFile:    &repoconfig.File{},
		EnvAPIToken: "tok-p",
		NewClient:   noRequestsAllowed(t),
	})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if got.WorkspaceID != "ws-1" || got.ProductID != "P1" || got.BaseURL != "https://x.example" || got.Source != SourceEnv {
		t.Errorf("got = %+v, want the product token's own workspace with zero requests", got)
	}
}

func TestResolve_UnknownWorkspace_ProductTokenNotProbed(t *testing.T) {
	_, err := Resolve(context.Background(), Deps{
		Store:     productTokenStore(),
		RepoFile:  &repoconfig.File{Exists: true, WorkspaceID: "ws-9"},
		NewClient: noRequestsAllowed(t),
	})
	var rerr *Error
	if !errors.As(err, &rerr) || rerr.Code != "E1" {
		t.Fatalf("err = %v, want E1 with no membership probe of the product token", err)
	}
}

// The repo binds ws-1 to product P2 while the stored ws-1 entry is a product
// token for P1: a personal login that lists ws-1 is preferred over a token
// that would answer 403 on every call.
func TestResolve_ProductTokenForAnotherProduct_PrefersPersonalLogin(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer tok-a" {
			t.Errorf("unexpected request with %q to %s", r.Header.Get("Authorization"), r.URL.Path)
			w.WriteHeader(http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"workspaces":[{"id":"ws-1","name":"One"},{"id":"ws-2","name":"Two"}]}`))
	}))
	defer srv.Close()

	store := productTokenStore()
	store.Workspaces["ws-1"] = config.Entry{BaseURL: srv.URL, APIToken: "tok-p", Account: "CI (product token)", Name: "One", ProductID: "P1"}
	store.Workspaces["ws-2"] = config.Entry{BaseURL: srv.URL, APIToken: "tok-a", Account: "a@example.dev", Name: "Two"}

	got, err := Resolve(context.Background(), Deps{
		Store:     store,
		RepoFile:  &repoconfig.File{Exists: true, WorkspaceID: "ws-1", ProductID: "P2"},
		NewClient: realClientFactory(5 * time.Second),
	})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if got.APIToken != "tok-a" || got.WorkspaceID != "ws-1" || got.ProductID != "" {
		t.Errorf("got = %+v, want the personal login for ws-1", got)
	}
}

// With no other login, the product token is still used: its tools answer the
// server's own "limited to product" refusal rather than the server refusing
// to start.
func TestResolve_ProductTokenForAnotherProduct_NoAlternativeKeepsIt(t *testing.T) {
	got, err := Resolve(context.Background(), Deps{
		Store:     productTokenStore(),
		RepoFile:  &repoconfig.File{Exists: true, WorkspaceID: "ws-1", ProductID: "P2"},
		NewClient: noRequestsAllowed(t),
	})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if got.APIToken != "tok-p" || got.ProductID != "P1" {
		t.Errorf("got = %+v, want the product token kept", got)
	}
}
