package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestListIssues_BuildsQuery(t *testing.T) {
	var gotPath, gotAuth string
	var gotQuery url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		gotQuery = r.URL.Query()
		_, _ = w.Write([]byte(`{"issues":[{"id":"i1","title":"boom","timesSeen":"42"}],"pagination":{"nextPageToken":"n1"}}`))
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	resp, err := c.ListIssues(context.Background(), "p1", ListIssuesOptions{
		Status:        "unresolved",
		EnvironmentID: "e1",
		Levels:        []string{"error", "fatal"},
		Period:        "7d",
		Sort:          "times_seen",
		PageSize:      25,
	})
	if err != nil {
		t.Fatalf("ListIssues: %v", err)
	}
	if gotPath != "/v1/products/p1/issues" {
		t.Errorf("path = %q", gotPath)
	}
	if gotAuth != "Bearer tok" {
		t.Errorf("auth = %q", gotAuth)
	}
	if got := gotQuery.Get("status"); got != "unresolved" {
		t.Errorf("status = %q", got)
	}
	if got := gotQuery.Get("environmentId"); got != "e1" {
		t.Errorf("environmentId = %q", got)
	}
	if got := gotQuery["levels"]; len(got) != 2 {
		t.Errorf("levels = %v, want two repeated values", got)
	}
	if got := gotQuery.Get("pagination.pageSize"); got != "25" {
		t.Errorf("pageSize = %q", got)
	}
	if len(resp.Issues) != 1 || resp.Issues[0].TimesSeen != 42 {
		t.Errorf("timesSeen not decoded from its JSON string form: %+v", resp.Issues)
	}
}

// TestListIssues_DefaultPageSize pins the client-side default: an unset
// PageSize must not send pagination.pageSize=0, which the API reads as "no
// results".
func TestListIssues_DefaultPageSize(t *testing.T) {
	var gotQuery url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		_, _ = w.Write([]byte(`{"issues":[]}`))
	}))
	defer srv.Close()

	if _, err := newTestClient(srv.URL).ListIssues(context.Background(), "p1", ListIssuesOptions{}); err != nil {
		t.Fatalf("ListIssues: %v", err)
	}
	if got := gotQuery.Get("pagination.pageSize"); got != "50" {
		t.Errorf("pageSize = %q, want 50", got)
	}
	if got := gotQuery.Get("status"); got != "" {
		t.Errorf("status = %q, want it omitted", got)
	}
}

func TestGetIssue_DecodesLatestEventFrames(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`{"issue":{"id":"i1","timesSeen":"7"},"latestEvent":{"id":"e1","payload":"{\"big\":true}","frames":[{"absPath":"/app/src/cart.go","lineno":42,"inApp":true}]}}`))
	}))
	defer srv.Close()

	resp, err := newTestClient(srv.URL).GetIssue(context.Background(), "i1")
	if err != nil {
		t.Fatalf("GetIssue: %v", err)
	}
	if gotPath != "/v1/issues/i1" {
		t.Errorf("path = %q", gotPath)
	}
	if resp.Issue.TimesSeen != 7 {
		t.Errorf("timesSeen = %d, want 7", resp.Issue.TimesSeen)
	}
	if resp.LatestEvent == nil || len(resp.LatestEvent.Frames) != 1 {
		t.Fatalf("latestEvent = %+v, want one frame", resp.LatestEvent)
	}
	f := resp.LatestEvent.Frames[0]
	if f.AbsPath != "/app/src/cart.go" || f.Lineno != 42 || !f.InApp {
		t.Errorf("frame = %+v", f)
	}
	if resp.LatestEvent.Payload == "" {
		t.Error("payload should be decoded by the client; clearing it is the tool layer's job")
	}
}

func TestGetIssueFixContext_QueryAndUnwrap(t *testing.T) {
	var gotPath string
	var gotQuery url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.Query()
		_, _ = w.Write([]byte(`{"context":{"suspectPaths":["src/cart.go"],"suspectRequirements":[{"id":"r1","coverage":"no_test"}]}}`))
	}))
	defer srv.Close()

	fc, err := newTestClient(srv.URL).GetIssueFixContext(context.Background(), "p1", "i1", "feature/x", 25)
	if err != nil {
		t.Fatalf("GetIssueFixContext: %v", err)
	}
	if gotPath != "/v1/products/p1/issues/i1/fix-context" {
		t.Errorf("path = %q", gotPath)
	}
	if got := gotQuery.Get("branch"); got != "feature/x" {
		t.Errorf("branch = %q", got)
	}
	if got := gotQuery.Get("maxFrames"); got != "25" {
		t.Errorf("maxFrames = %q", got)
	}
	if len(fc.SuspectPaths) != 1 || fc.SuspectPaths[0] != "src/cart.go" {
		t.Errorf("suspectPaths = %v", fc.SuspectPaths)
	}
	if len(fc.SuspectRequirements) != 1 || fc.SuspectRequirements[0].Coverage != "no_test" {
		t.Errorf("suspectRequirements = %+v", fc.SuspectRequirements)
	}
}

// TestGetIssueFixContext_OmitsEmptyQuery guards the "?" suffix: an unset
// branch/max_frames must produce a bare path, not a trailing "?".
func TestGetIssueFixContext_OmitsEmptyQuery(t *testing.T) {
	var gotRawQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotRawQuery = r.URL.RawQuery
		_, _ = w.Write([]byte(`{"context":{}}`))
	}))
	defer srv.Close()

	if _, err := newTestClient(srv.URL).GetIssueFixContext(context.Background(), "p1", "i1", "", 0); err != nil {
		t.Fatalf("GetIssueFixContext: %v", err)
	}
	if gotRawQuery != "" {
		t.Errorf("raw query = %q, want empty", gotRawQuery)
	}
}

func TestUpdateIssueStatus_PostsStatus(t *testing.T) {
	var gotMethod, gotPath string
	var raw []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		raw, _ = io.ReadAll(r.Body)
		_, _ = w.Write([]byte(`{"issue":{"id":"i1","status":"resolved","timesSeen":"3"}}`))
	}))
	defer srv.Close()

	iss, err := newTestClient(srv.URL).UpdateIssueStatus(context.Background(), "i1", "resolved")
	if err != nil {
		t.Fatalf("UpdateIssueStatus: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method = %s, want POST", gotMethod)
	}
	if gotPath != "/v1/issues/i1:setStatus" {
		t.Errorf("path = %q", gotPath)
	}
	var body map[string]any
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if body["status"] != "resolved" {
		t.Errorf("status in body = %v", body["status"])
	}
	if iss.Status != "resolved" {
		t.Errorf("issue status = %q", iss.Status)
	}
}

func TestBulkUpdateIssueStatus_Body(t *testing.T) {
	var raw []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ = io.ReadAll(r.Body)
		_, _ = w.Write([]byte(`{"updatedCount":2}`))
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	n, err := c.BulkUpdateIssueStatus(context.Background(), "p1", []string{"a", "b"}, "resolved")
	if err != nil {
		t.Fatalf("BulkUpdateIssueStatus: %v", err)
	}
	if n != 2 {
		t.Errorf("updatedCount = %d, want 2", n)
	}
	var body map[string]any
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if body["status"] != "resolved" {
		t.Errorf("status in body = %v", body["status"])
	}
	if ids, ok := body["ids"].([]any); !ok || len(ids) != 2 {
		t.Errorf("ids in body = %v", body["ids"])
	}
}

// TestListReleaseIssues_SeenCountString pins the protojson int64-as-string
// decoding for seenCount.
func TestListReleaseIssues_SeenCountString(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`{"newIssues":[{"id":"i1","timesSeen":"1"}],"seenCount":"9"}`))
	}))
	defer srv.Close()

	resp, err := newTestClient(srv.URL).ListReleaseIssues(context.Background(), "rel1")
	if err != nil {
		t.Fatalf("ListReleaseIssues: %v", err)
	}
	if gotPath != "/v1/releases/rel1/issues" {
		t.Errorf("path = %q", gotPath)
	}
	if resp.SeenCount != 9 {
		t.Errorf("seenCount = %d, want 9", resp.SeenCount)
	}
	if len(resp.NewIssues) != 1 {
		t.Errorf("newIssues = %+v", resp.NewIssues)
	}
}

func TestGetIssueEventStats_Path(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`{"interval":"1h","last24h":12,"environments":[{"environmentName":"production","count":10}]}`))
	}))
	defer srv.Close()

	stats, err := newTestClient(srv.URL).GetIssueEventStats(context.Background(), "i1")
	if err != nil {
		t.Fatalf("GetIssueEventStats: %v", err)
	}
	if gotPath != "/v1/issues/i1/stats" {
		t.Errorf("path = %q", gotPath)
	}
	if len(stats.Environments) != 1 || stats.Environments[0].EnvironmentName != "production" {
		t.Errorf("environments = %+v", stats.Environments)
	}
	if stats.Last24h != 12 {
		t.Errorf("last24h = %d, want 12", stats.Last24h)
	}
}

func TestGetIssueEvent_EscapesPathSegments(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.EscapedPath()
		_, _ = w.Write([]byte(`{"event":{"id":"e1","eventId":"abc"}}`))
	}))
	defer srv.Close()

	ev, err := newTestClient(srv.URL).GetIssueEvent(context.Background(), "i/1", "e1")
	if err != nil {
		t.Fatalf("GetIssueEvent: %v", err)
	}
	if gotPath != "/v1/issues/i%2F1/events/e1" {
		t.Errorf("escaped path = %q", gotPath)
	}
	if ev.EventID != "abc" {
		t.Errorf("eventId = %q", ev.EventID)
	}
}

func TestListIssueEvents_Pagination(t *testing.T) {
	var gotPath string
	var gotQuery url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.Query()
		_, _ = w.Write([]byte(`{"events":[{"id":"e1","payload":"{}"}],"pagination":{"nextPageToken":"n2"}}`))
	}))
	defer srv.Close()

	resp, err := newTestClient(srv.URL).ListIssueEvents(context.Background(), "i1", 10, "tok1")
	if err != nil {
		t.Fatalf("ListIssueEvents: %v", err)
	}
	if gotPath != "/v1/issues/i1/events" {
		t.Errorf("path = %q", gotPath)
	}
	if got := gotQuery.Get("pagination.pageSize"); got != "10" {
		t.Errorf("pageSize = %q", got)
	}
	if got := gotQuery.Get("pagination.pageToken"); got != "tok1" {
		t.Errorf("pageToken = %q", got)
	}
	if len(resp.Events) != 1 || resp.Pagination.NextPageToken != "n2" {
		t.Errorf("resp = %+v", resp)
	}
}
