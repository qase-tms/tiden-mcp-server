package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/qase-tms/tiden-mcp-server/internal/model"
)

func ptr(s string) *string { return &s }

func TestUpdateRequirement_PUTPathAndAuth(t *testing.T) {
	var gotMethod, gotPath, gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"requirement":{"id":"abc","title":"new"}}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "tok")
	if _, err := c.UpdateRequirement(context.Background(), "abc", UpdateRequirementRequest{Title: ptr("new")}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotMethod != "PUT" {
		t.Errorf("method = %s, want PUT", gotMethod)
	}
	if gotPath != "/v1/requirements/abc" {
		t.Errorf("path = %s", gotPath)
	}
	if gotAuth != "Bearer tok" {
		t.Errorf("auth = %s", gotAuth)
	}
}

func TestCreateRequirement_WithSourcesInBody(t *testing.T) {
	var rawBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/v1/products/p1/requirements" {
			t.Errorf("path = %s", r.URL.Path)
		}
		rawBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"requirement":{"id":"r1","title":"T"}}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "tok")
	_, err := c.CreateRequirementTypedWithSources(context.Background(), "p1", "T", "C", nil, nil, "Feature", []model.RequirementSourceInput{
		{SourceType: "repo_file", Title: "API", RepoPath: "internal/api.go"},
	}, "feature/x")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(rawBody, &parsed); err != nil {
		t.Fatalf("body not valid JSON: %v", err)
	}
	if parsed["branch"] != "feature/x" || parsed["type"] != "Feature" {
		t.Fatalf("body = %s", string(rawBody))
	}
	sources, ok := parsed["sources"].([]any)
	if !ok || len(sources) != 1 {
		t.Fatalf("sources = %#v, want one source; body=%s", parsed["sources"], string(rawBody))
	}
	src := sources[0].(map[string]any)
	if src["sourceType"] != "repo_file" || src["repoPath"] != "internal/api.go" {
		t.Fatalf("source = %#v", src)
	}
}

func TestUpdateRequirement_WithSourcesUpdateInBody(t *testing.T) {
	var rawBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rawBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"requirement":{"id":"abc"}}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "tok")
	_, err := c.UpdateRequirement(context.Background(), "abc", UpdateRequirementRequest{
		SourcesUpdate: &model.RequirementSourcesUpdate{Sources: []model.RequirementSourceInput{}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(rawBody, &parsed); err != nil {
		t.Fatalf("body not valid JSON: %v", err)
	}
	su, ok := parsed["sourcesUpdate"].(map[string]any)
	if !ok {
		t.Fatalf("sourcesUpdate missing in %s", string(rawBody))
	}
	sources, ok := su["sources"].([]any)
	if !ok || len(sources) != 0 {
		t.Fatalf("sourcesUpdate.sources = %#v, want empty array", su["sources"])
	}
}

func TestUpdateRequirement_OnlySetFieldsInBody(t *testing.T) {
	cases := []struct {
		name        string
		body        UpdateRequirementRequest
		wantPresent []string
		wantAbsent  []string
	}{
		{
			name:        "title only",
			body:        UpdateRequirementRequest{Title: ptr("X")},
			wantPresent: []string{"title"},
			wantAbsent:  []string{"content", "status", "priority", "type", "parentId", "componentId", "branch"},
		},
		{
			name:        "content only",
			body:        UpdateRequirementRequest{Content: ptr("body text")},
			wantPresent: []string{"content"},
			wantAbsent:  []string{"title", "status", "priority", "type", "parentId", "componentId", "branch"},
		},
		{
			name:        "branch only",
			body:        UpdateRequirementRequest{Branch: "intent/2026-04-27-foo"},
			wantPresent: []string{"branch"},
			wantAbsent:  []string{"title", "content", "status", "priority", "type", "parentId", "componentId"},
		},
		{
			name: "all fields",
			body: UpdateRequirementRequest{
				Title:       ptr("T"),
				Content:     ptr("C"),
				Status:      ptr("Active"),
				Priority:    ptr("High"),
				Type:        ptr("Feature"),
				ParentID:    ptr("uuid-parent"),
				ComponentID: ptr("uuid-comp"),
				Branch:      "intent/x",
			},
			wantPresent: []string{"title", "content", "status", "priority", "type", "parentId", "componentId", "branch"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var rawBody []byte
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				rawBody, _ = io.ReadAll(r.Body)
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"requirement":{"id":"abc"}}`))
			}))
			defer srv.Close()

			c := New(srv.URL, "tok")
			if _, err := c.UpdateRequirement(context.Background(), "abc", tc.body); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			var parsed map[string]any
			if err := json.Unmarshal(rawBody, &parsed); err != nil {
				t.Fatalf("body not valid JSON: %v", err)
			}
			for _, k := range tc.wantPresent {
				if _, ok := parsed[k]; !ok {
					t.Errorf("expected key %q in body, got %s", k, string(rawBody))
				}
			}
			for _, k := range tc.wantAbsent {
				if _, ok := parsed[k]; ok {
					t.Errorf("expected key %q to be absent, got %s", k, string(rawBody))
				}
			}
		})
	}
}

func TestUpdateRequirement_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"requirement not found"}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "tok")
	_, err := c.UpdateRequirement(context.Background(), "missing", UpdateRequirementRequest{Title: ptr("T")})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestUpdateRequirement_Unauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	c := New(srv.URL, "tok")
	_, err := c.UpdateRequirement(context.Background(), "abc", UpdateRequirementRequest{Title: ptr("T")})
	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("err = %v, want ErrUnauthorized", err)
	}
}

func TestUpdateRequirement_DecodesResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"requirement":{"id":"abc","productId":"p1","title":"renamed","status":"Active","priority":"High","type":"Feature","seqNum":7}}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "tok")
	got, err := c.UpdateRequirement(context.Background(), "abc", UpdateRequirementRequest{Title: ptr("renamed")})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != "abc" || got.Title != "renamed" || got.Status != "Active" || got.Priority != "High" || got.Type != "Feature" || got.SeqNum != 7 {
		t.Errorf("decoded = %+v", got)
	}
}

func TestUpdateRequirement_ContentTypeIsJSON(t *testing.T) {
	var gotCT string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCT = r.Header.Get("Content-Type")
		_, _ = w.Write([]byte(`{"requirement":{"id":"abc"}}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "tok")
	if _, err := c.UpdateRequirement(context.Background(), "abc", UpdateRequirementRequest{Title: ptr("T")}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(gotCT, "application/json") {
		t.Errorf("Content-Type = %s, want application/json", gotCT)
	}
}
