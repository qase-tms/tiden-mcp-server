package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestDistillIntent_PostsToDistillPath(t *testing.T) {
	var gotMethod, gotPath, gotAuth string
	var rawBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		rawBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"intentBranch":"intent/2026-07-22-dark-mode","created":3,"updated":1,"dropped":0,"skipped":false}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "tok")
	resp, err := c.DistillIntent(context.Background(), "prod-1", DistillIntentRequest{
		Transcript:   "USER: add dark mode\nASSISTANT: added a theme toggle",
		Slug:         "dark-mode",
		SessionID:    "sess-9",
		Agent:        "cursor",
		ChangedFiles: []string{"internal/theme.go"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method = %s, want POST", gotMethod)
	}
	if gotPath != "/v1/products/prod-1/intent:distill" {
		t.Errorf("path = %s, want /v1/products/prod-1/intent:distill", gotPath)
	}
	if gotAuth != "Bearer tok" {
		t.Errorf("auth = %s, want Bearer tok", gotAuth)
	}

	var parsed map[string]any
	if err := json.Unmarshal(rawBody, &parsed); err != nil {
		t.Fatalf("body not valid JSON: %v", err)
	}
	if parsed["transcript"] != "USER: add dark mode\nASSISTANT: added a theme toggle" {
		t.Errorf("transcript = %v", parsed["transcript"])
	}
	if parsed["slug"] != "dark-mode" || parsed["sessionId"] != "sess-9" || parsed["agent"] != "cursor" {
		t.Errorf("provenance fields = %s", string(rawBody))
	}
	files, ok := parsed["changedFiles"].([]any)
	if !ok || len(files) != 1 || files[0] != "internal/theme.go" {
		t.Errorf("changedFiles = %#v", parsed["changedFiles"])
	}

	if resp.IntentBranch != "intent/2026-07-22-dark-mode" || resp.Created != 3 || resp.Updated != 1 {
		t.Errorf("resp = %+v", resp)
	}
}

func TestDistillIntent_OmitsEmptyOptionals(t *testing.T) {
	var rawBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rawBody, _ = io.ReadAll(r.Body)
		_, _ = w.Write([]byte(`{"intentBranch":"intent/x","skipped":false}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "tok")
	if _, err := c.DistillIntent(context.Background(), "p1", DistillIntentRequest{Transcript: "a decision"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(rawBody, &parsed); err != nil {
		t.Fatalf("body not valid JSON: %v", err)
	}
	for _, k := range []string{"slug", "sessionId", "agent", "changedFiles", "credentialId", "model"} {
		if _, ok := parsed[k]; ok {
			t.Errorf("expected key %q absent from body, got %s", k, string(rawBody))
		}
	}
	if parsed["transcript"] != "a decision" {
		t.Errorf("transcript missing: %s", string(rawBody))
	}
}

func TestDistillIntent_DecodesSkip(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"skipped":true,"skipReason":"no product-relevant decisions"}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "tok")
	resp, err := c.DistillIntent(context.Background(), "p1", DistillIntentRequest{Transcript: "hi"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.Skipped || resp.SkipReason != "no product-relevant decisions" {
		t.Errorf("resp = %+v", resp)
	}
}

func TestWithTimeout_ReturnsDistinctClientSameBaseAndAuth(t *testing.T) {
	var gotAuth, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "tok")
	c2 := c.WithTimeout(180 * time.Second)

	if c2 == c {
		t.Fatal("WithTimeout returned the same client pointer, want a copy")
	}
	if c2.httpClient == c.httpClient {
		t.Error("WithTimeout shares the original http.Client, want a distinct one")
	}
	if c2.httpClient.Timeout != 180*time.Second {
		t.Errorf("copy timeout = %v, want 180s", c2.httpClient.Timeout)
	}
	// Original client is left untouched (still the default 30s from New).
	if c.httpClient.Timeout != 30*time.Second {
		t.Errorf("original timeout = %v, want 30s (unchanged)", c.httpClient.Timeout)
	}

	// The copy still hits the same base URL with the same auth header.
	if err := c2.Do(context.Background(), "GET", "/v1/things", nil, nil); err != nil {
		t.Fatalf("unexpected error from copy: %v", err)
	}
	if gotAuth != "Bearer tok" {
		t.Errorf("auth = %s, want Bearer tok", gotAuth)
	}
	if gotPath != "/v1/things" {
		t.Errorf("path = %s, want /v1/things", gotPath)
	}
}
