package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// runJSON is a full protojson-shaped TestRun payload: camelCase keys, int64
// stats as strings, explicit nulls for unset message fields.
const runJSON = `{
  "id": "0d1f7f2a-6c9b-4a1e-9b7a-6f7d2c9e1b40",
  "productId": "p1",
  "seqNum": 42,
  "title": "Nightly regression",
  "description": "",
  "status": "new",
  "environmentId": "env-1",
  "environmentSlug": "staging",
  "environmentName": "Staging",
  "branchName": "main",
  "branchId": "",
  "configurations": {"browser": "chromium"},
  "buildSha": "a1b2c3d4",
  "clientMeta": {"framework": "playwright"},
  "stats": {
    "total": "3", "passed": "2", "failed": "1", "blocked": "0",
    "skipped": "0", "invalid": "0", "muted": "0", "attempts": "3",
    "durationMs": "8420"
  },
  "startedAt": "2026-07-03T10:00:00Z",
  "completedAt": null,
  "liveDocStatus": "none",
  "liveDocOperationId": "",
  "liveDocStats": null,
  "liveDocError": "",
  "createdBy": "u1",
  "createdAt": "2026-07-03T10:00:00Z",
  "updatedAt": "2026-07-03T10:00:00Z"
}`

func TestCreateTestRun(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/products/p1/runs" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.String())
		}
		data, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(data, &gotBody); err != nil {
			t.Errorf("body invalid JSON: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"run": ` + runJSON + `}`))
	}))
	defer srv.Close()

	client := newTestClient(srv.URL)
	run, err := client.CreateTestRun(context.Background(), "p1", CreateTestRunRequest{
		Title:          "Nightly regression",
		Environment:    "staging",
		Branch:         "main",
		Configurations: map[string]string{"browser": "chromium"},
		BuildSha:       "a1b2c3d4",
	})
	if err != nil {
		t.Fatalf("CreateTestRun: %v", err)
	}
	if gotBody["title"] != "Nightly regression" || gotBody["environment"] != "staging" ||
		gotBody["branch"] != "main" || gotBody["buildSha"] != "a1b2c3d4" {
		t.Errorf("request body = %#v", gotBody)
	}
	if _, present := gotBody["description"]; present {
		t.Errorf("unset optional field serialized: %#v", gotBody)
	}
	if run.SeqNum != 42 || run.Status != "new" || run.EnvironmentSlug != "staging" {
		t.Errorf("run = %+v", run)
	}
	if run.Stats == nil || run.Stats.Total != 3 || run.Stats.DurationMs != 8420 {
		t.Errorf("int64-string stats decode failed: %+v", run.Stats)
	}
	if run.CompletedAt != nil || run.LiveDocStats != nil {
		t.Errorf("null fields should decode to nil pointers: %+v", run)
	}
}

func TestListTestRunsQueryParams(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if r.URL.Path != "/v1/products/p1/runs" {
			t.Errorf("path = %s", r.URL.Path)
		}
		if q.Get("status") != "in_progress" || q.Get("environment") != "staging" ||
			q.Get("branch") != "feat/x" || q.Get("search") != "night" ||
			q.Get("pagination.pageSize") != "10" || q.Get("pagination.pageToken") != "1" {
			t.Errorf("query = %v", q)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"runs": [` + runJSON + `], "pagination": {"nextPageToken": "2", "totalCount": 21}}`))
	}))
	defer srv.Close()

	client := newTestClient(srv.URL)
	resp, err := client.ListTestRuns(context.Background(), "p1", ListTestRunsOptions{
		Status: "in_progress", Environment: "staging", Branch: "feat/x",
		Search: "night", PageSize: 10, PageToken: "1",
	})
	if err != nil {
		t.Fatalf("ListTestRuns: %v", err)
	}
	if len(resp.Runs) != 1 || resp.Pagination.NextPageToken != "2" || resp.Pagination.TotalCount != 21 {
		t.Errorf("resp = %+v", resp)
	}
}

func TestRunLifecycleEndpoints(t *testing.T) {
	var gotMethod, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"run": ` + runJSON + `}`))
	}))
	defer srv.Close()
	client := newTestClient(srv.URL)
	ctx := context.Background()

	if _, err := client.GetTestRun(ctx, "p1", 42); err != nil {
		t.Fatalf("GetTestRun: %v", err)
	}
	if gotMethod != http.MethodGet || gotPath != "/v1/products/p1/runs/42" {
		t.Errorf("GetTestRun request = %s %s", gotMethod, gotPath)
	}

	if _, err := client.CompleteTestRun(ctx, "p1", 42); err != nil {
		t.Fatalf("CompleteTestRun: %v", err)
	}
	if gotMethod != http.MethodPost || gotPath != "/v1/products/p1/runs/42:complete" {
		t.Errorf("CompleteTestRun request = %s %s", gotMethod, gotPath)
	}

	if _, err := client.AbortTestRun(ctx, "p1", 42); err != nil {
		t.Fatalf("AbortTestRun: %v", err)
	}
	if gotMethod != http.MethodPost || gotPath != "/v1/products/p1/runs/42:abort" {
		t.Errorf("AbortTestRun request = %s %s", gotMethod, gotPath)
	}
}

func TestReportResultsPassthrough(t *testing.T) {
	var gotBody struct {
		Results []map[string]any `json:"results"`
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/products/p1/runs/42/results:report" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.String())
		}
		data, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(data, &gotBody); err != nil {
			t.Errorf("body invalid JSON: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status": true, "accepted": "1", "duplicates": "0", "errors": []}`))
	}))
	defer srv.Close()

	client := newTestClient(srv.URL)
	resp, err := client.ReportResults(context.Background(), "p1", 42, []map[string]any{
		{"id": "r1", "title": "t", "execution": map[string]any{"status": "passed"}, "someFutureField": 7},
	})
	if err != nil {
		t.Fatalf("ReportResults: %v", err)
	}
	if resp.Accepted != 1 {
		t.Errorf("accepted = %d", resp.Accepted)
	}
	if len(gotBody.Results) != 1 || gotBody.Results[0]["someFutureField"] != float64(7) {
		t.Errorf("results not passed through: %#v", gotBody.Results)
	}
}

func TestParseReportErrors(t *testing.T) {
	body := `{
	  "code": 3,
	  "message": "REPORT_VALIDATION: 2 result entries rejected (first: #0/INVALID_TITLE: title is required)",
	  "details": [
	    {"@type": "type.googleapis.com/api.v1.ReportError", "index": 0, "resultId": "", "code": "INVALID_TITLE", "message": "title is required"},
	    {"@type": "type.googleapis.com/api.v1.ReportError", "index": 1, "resultId": "abc", "code": "TEST_NOT_FOUND", "message": "testops_id 999 does not resolve to a case"},
	    {"@type": "type.googleapis.com/other.Thing", "whatever": true}
	  ]
	}`
	entries, ok := ParseReportErrors(body)
	if !ok || len(entries) != 2 {
		t.Fatalf("entries = %v ok = %v", entries, ok)
	}
	if entries[0].Index != 0 || entries[0].Code != "INVALID_TITLE" || entries[0].ResultID != "" {
		t.Errorf("entry 0 = %+v", entries[0])
	}
	if entries[1].Index != 1 || entries[1].ResultID != "abc" || entries[1].Code != "TEST_NOT_FOUND" {
		t.Errorf("entry 1 = %+v", entries[1])
	}
	if _, ok := ParseReportErrors(`{"code":5,"message":"TEST_RUN_NOT_FOUND: nope"}`); ok {
		t.Error("no-details body must return ok=false")
	}
	if _, ok := ParseReportErrors(`not json`); ok {
		t.Error("non-JSON body must return ok=false")
	}
}

func TestListRunResultsQueryParamsAndDecode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if r.URL.Path != "/v1/products/p1/runs/42/results" {
			t.Errorf("path = %s", r.URL.Path)
		}
		if q.Get("status") != "failed" || q.Get("latestOnly") != "true" ||
			q.Get("identityKey") != "t:x" || q.Get("search") != "card" ||
			q.Get("pagination.pageSize") != "50" {
			t.Errorf("query = %v", q)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results": [{
		  "id": "r1", "runId": "run-1", "testId": "", "testSeqNum": 0,
		  "eventSeq": "104", "title": "pays with a valid card",
		  "signature": "sig", "externalId": "", "identityKey": "t:x",
		  "executionKey": "t:x|9f", "status": "failed", "durationMs": "3421",
		  "startedAt": "2026-07-03T10:00:00.120Z", "endedAt": null,
		  "thread": "", "message": "402", "stacktrace": "boom",
		  "params": {"browser": "chromium"}, "paramGroups": [{"names": ["browser"]}],
		  "fields": {"muted": "false"}, "steps": [],
		  "suitePath": [{"title": "Checkout", "externalId": ""}],
		  "attachments": [], "muted": false, "defect": false,
		  "isLatestAttempt": true, "attempt": 1, "createdAt": "2026-07-03T10:00:05Z"
		}], "pagination": {"nextPageToken": "", "totalCount": 1}}`))
	}))
	defer srv.Close()

	client := newTestClient(srv.URL)
	resp, err := client.ListRunResults(context.Background(), "p1", 42, ListRunResultsOptions{
		Status: "failed", Search: "card", IdentityKey: "t:x", LatestOnly: true, PageSize: 50,
	})
	if err != nil {
		t.Fatalf("ListRunResults: %v", err)
	}
	r := resp.Results[0]
	if r.EventSeq != 104 || r.DurationMs != 3421 || r.TestID != "" || r.TestSeqNum != 0 {
		t.Errorf("result decode = %+v", r)
	}
	if r.EndedAt != nil || r.StartedAt == nil {
		t.Errorf("timestamp pointers = %+v", r)
	}
}

func TestGetRunSummary(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/products/p1/runs/42/summary":
			_, _ = w.Write([]byte(`{
			  "suites": [{"path": ["Checkout", "Cards"], "stats": {"total": "2", "passed": "1", "failed": "1", "blocked": "0", "skipped": "0", "invalid": "0", "muted": "0", "attempts": "2", "durationMs": "5921"}}],
			  "cases": [{"identityKey": "t:x", "title": "pays", "suitePath": ["Checkout", "Cards"], "testId": "", "testSeqNum": 0, "status": "failed", "durationMs": "3421", "attempts": 1, "muted": false,
			    "combos": [{"executionKey": "t:x|9f", "params": {"browser": "chromium"}, "status": "failed", "durationMs": "3421", "attempts": 1, "resultId": "r1"}]}]
			}`))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	client := newTestClient(srv.URL)
	sum, err := client.GetRunSummary(context.Background(), "p1", 42)
	if err != nil {
		t.Fatalf("GetRunSummary: %v", err)
	}
	if len(sum.Suites) != 1 || sum.Suites[0].Stats.DurationMs != 5921 {
		t.Errorf("suites = %+v", sum.Suites)
	}
	if len(sum.Cases) != 1 || sum.Cases[0].Combos[0].DurationMs != 3421 {
		t.Errorf("cases = %+v", sum.Cases)
	}
}
