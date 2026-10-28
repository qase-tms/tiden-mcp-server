package api

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestListReleases_WalksEveryPage: the gateway ignores a bare `pageSize` and
// answers with its default page of 20, so the listing asks with
// pagination.pageSize and follows nextPageToken until the last page (AICP-89).
func TestListReleases_WalksEveryPage(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		q := r.URL.Query()
		if r.URL.Path != "/v1/products/p1/releases" || q.Get("pagination.pageSize") != "100" || q.Has("pageSize") {
			t.Errorf("request = %s, want pagination.pageSize=100 and no bare pageSize", r.URL.String())
		}
		w.Header().Set("Content-Type", "application/json")
		switch q.Get("pagination.pageToken") {
		case "":
			_, _ = io.WriteString(w, `{"releases":[{"id":"r3","version":"3.0.0"},{"id":"r2","version":"2.0.0"}],"pagination":{"nextPageToken":"t2","totalCount":3}}`)
		case "t2":
			_, _ = io.WriteString(w, `{"releases":[{"id":"r1","version":"1.0.0","buildSha":"1111111111111111111111111111111111111111"}],"pagination":{"nextPageToken":"","totalCount":3}}`)
		default:
			t.Errorf("unexpected page token %q", q.Get("pagination.pageToken"))
		}
	}))
	defer srv.Close()

	resp, err := newTestClient(srv.URL).ListReleases(context.Background(), "p1")
	if err != nil {
		t.Fatalf("ListReleases: %v", err)
	}
	if calls != 2 || len(resp.Releases) != 3 {
		t.Fatalf("calls = %d, releases = %d; want 2 pages and 3 releases", calls, len(resp.Releases))
	}
	if resp.Releases[2].BuildSHA != "1111111111111111111111111111111111111111" || resp.Releases[0].BuildSHA != "" {
		t.Errorf("build SHAs = %q, %q; want the third release's SHA and none on the first", resp.Releases[2].BuildSHA, resp.Releases[0].BuildSHA)
	}
	if resp.Pagination.NextPageToken != "" || resp.Pagination.TotalCount != 3 {
		t.Errorf("pagination = %+v, want no token left and the total", resp.Pagination)
	}
}

// TestIsCapturePending: only the server's QG_CAPTURE_PENDING refusal counts as
// a pending release gate; every other error is still an error.
func TestIsCapturePending(t *testing.T) {
	pending := newAPIError(http.StatusBadRequest, []byte(`{"code":9,"message":"QG_CAPTURE_PENDING: the Quality Gate of this release is pending: build abc123def456 has no settled test results yet"}`))
	if !IsCapturePending(pending) {
		t.Fatal("pending refusal not recognised")
	}
	if got, want := CapturePendingMessage(pending), "the Quality Gate of this release is pending: build abc123def456 has no settled test results yet"; got != want {
		t.Errorf("message = %q, want %q", got, want)
	}
	if !IsCapturePending(fmt.Errorf("get verdict: %w", pending)) {
		t.Error("a wrapped pending refusal is not recognised")
	}
	for name, err := range map[string]error{
		"other 400":     newAPIError(http.StatusBadRequest, []byte(`{"code":9,"message":"QG_NOT_CAPTURED: quality gate was not captured for this release"}`)),
		"not found":     fmt.Errorf("%w: QG_NOT_FOUND: release not found in product", ErrNotFound),
		"pending 409":   newAPIError(http.StatusConflict, []byte(`{"message":"QG_CAPTURE_PENDING: x"}`)),
		"nil":           nil,
		"plain message": fmt.Errorf("QG_CAPTURE_PENDING: not from the server"),
	} {
		if IsCapturePending(err) {
			t.Errorf("%s: recognised as pending", name)
		}
	}
}

// TestGetVerdict_ReleaseCaptureSurvivesTheRoundTrip: a release verdict's
// capture provenance reaches the agent instead of being dropped by the model.
func TestGetVerdict_ReleaseCaptureSurvivesTheRoundTrip(t *testing.T) {
	const body = `{"verdict":{"id":"v1","productId":"p1","scope":"VERDICT_SCOPE_RELEASE","releaseId":"rel-1",
		"status":"VERDICT_STATUS_PASS","buildSha":"1111111111111111111111111111111111111111",
		"releaseCapture":{"seq":2,"source":"build_sha","trigger":"manual","buildSha":"1111111111111111111111111111111111111111",
			"capturedAt":"2026-10-28T12:00:00Z","capturedBy":"u1","releaseBuildSha":"2222222222222222222222222222222222222222","recapturePending":true}}}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, body)
	}))
	defer srv.Close()

	v, err := newTestClient(srv.URL).GetVerdict(context.Background(), "p1", "release", "rel-1", "")
	if err != nil {
		t.Fatalf("GetVerdict: %v", err)
	}
	c := v.ReleaseCapture
	if c == nil {
		t.Fatal("releaseCapture was dropped")
	}
	if c.Seq != 2 || c.Source != "build_sha" || c.Trigger != "manual" || c.CapturedBy != "u1" ||
		c.BuildSHA != "1111111111111111111111111111111111111111" || c.ReleaseBuildSHA != "2222222222222222222222222222222222222222" ||
		!c.RecapturePending || c.CapturedAt != "2026-10-28T12:00:00Z" {
		t.Errorf("releaseCapture = %+v, want the capture's provenance", *c)
	}
}
