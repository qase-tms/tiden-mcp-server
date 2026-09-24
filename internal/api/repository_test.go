package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/qase-tms/tiden-mcp-server/internal/model"
)

func TestDo_501IsNotRetried(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusNotImplemented)
	}))
	defer srv.Close()

	err := newTestClient(srv.URL).Do(context.Background(), "GET", "/v1/repositories/resolve", nil, nil)
	if !errors.Is(err, ErrUnimplemented) {
		t.Fatalf("err = %v, want ErrUnimplemented", err)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("calls = %d, want 1 (501 must not be retried)", got)
	}
}

func TestResolveRepository_DecodesCandidates(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/repositories/resolve" {
			t.Errorf("path = %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("repository"); got != "github.com/org/repo" {
			t.Errorf("repository query = %q", got)
		}
		_, _ = w.Write([]byte(`{"candidates":[
			{"productId":"p1","productName":"Widgets","workspaceId":"w1","workspaceName":"Acme","source":"REPOSITORY_CANDIDATE_SOURCE_CI_SYNC"},
			{"productId":"p2","productName":"Gadgets","workspaceId":"w2","workspaceName":"Beta","source":"REPOSITORY_CANDIDATE_SOURCE_CODE_LINK"}
		]}`))
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	got, err := c.ResolveRepository(context.Background(), "github.com/org/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.Candidates) != 2 {
		t.Fatalf("Candidates = %+v", got.Candidates)
	}
	c1, c2 := got.Candidates[0], got.Candidates[1]
	if c1.ProductID != "p1" || c1.ProductName != "Widgets" || c1.WorkspaceID != "w1" || c1.WorkspaceName != "Acme" || c1.Source != model.RepositoryCandidateSourceCISync {
		t.Errorf("c1 = %+v", c1)
	}
	if c2.ProductID != "p2" || c2.Source != model.RepositoryCandidateSourceCodeLink {
		t.Errorf("c2 = %+v", c2)
	}
}

func TestResolveRepository_UnknownRepositoryReturnsEmpty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"candidates":[]}`))
	}))
	defer srv.Close()

	got, err := newTestClient(srv.URL).ResolveRepository(context.Background(), "github.com/org/unknown.git")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.Candidates) != 0 {
		t.Errorf("Candidates = %+v, want empty", got.Candidates)
	}
}

func TestResolveRepository_501ReturnsErrUnimplemented(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotImplemented)
	}))
	defer srv.Close()

	_, err := newTestClient(srv.URL).ResolveRepository(context.Background(), "github.com/org/repo.git")
	if !errors.Is(err, ErrUnimplemented) {
		t.Fatalf("err = %v, want ErrUnimplemented", err)
	}
}

func TestResolveRepository_404ReturnsErrUnimplemented(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	_, err := newTestClient(srv.URL).ResolveRepository(context.Background(), "github.com/org/repo.git")
	if !errors.Is(err, ErrUnimplemented) {
		t.Fatalf("err = %v, want ErrUnimplemented (a 404 on this route means an old server without it)", err)
	}
}

func TestResolveRepository_401ReturnsErrUnauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	_, err := newTestClient(srv.URL).ResolveRepository(context.Background(), "github.com/org/repo.git")
	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("err = %v, want ErrUnauthorized", err)
	}
}

func TestRepositoryCandidate_SourceLabel(t *testing.T) {
	cases := []struct {
		source string
		want   string
	}{
		{model.RepositoryCandidateSourceCISync, "ci"},
		{model.RepositoryCandidateSourceCodeLink, "code link"},
		{model.RepositoryCandidateSourceComponent, "component"},
		{"SOMETHING_ELSE", "SOMETHING_ELSE"},
	}
	for _, tc := range cases {
		c := model.RepositoryCandidate{Source: tc.source}
		if got := c.SourceLabel(); got != tc.want {
			t.Errorf("SourceLabel(%q) = %q, want %q", tc.source, got, tc.want)
		}
	}
}
