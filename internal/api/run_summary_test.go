package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRunSummaryProjection(t *testing.T) {
	for _, tc := range []struct {
		name, view, identity, token, body string
		size                              int
		invalid, wantError                bool
	}{
		{name: "overview", view: "overview", body: `{"view":"overview","suites":[],"caseCount":"0"}`},
		{name: "case_page", view: "cases", token: "1", body: `{"view":"cases","caseOverviews":[{"identityKey":"case","comboCount":"2"}],"pagination":{"totalCount":3,"nextPageToken":"2"}}`},
		{name: "combo_page", view: "combos", identity: "case/a+b", body: `{"view":"combos","combos":[{"executionKey":"exec","resultId":"r","params":{"browser":"firefox"},"durationMs":"42","attempts":3}],"pagination":{"totalCount":1}}`},
		{name: "empty_cases", view: "cases", body: `{"view":"cases","caseOverviews":[],"pagination":{"totalCount":0}}`},
		{name: "empty_combos", view: "combos", identity: "missing", body: `{"view":"combos","combos":[],"pagination":{"totalCount":0}}`},
		{name: "old_server", view: "cases", body: `{"suites":[],"cases":[]}`, wantError: true},
		{name: "missing_stats", view: "overview", body: `{"view":"overview","suites":[{"path":["Suite"]}],"caseCount":"1"}`, wantError: true},
		{name: "missing_count", view: "overview", body: `{"view":"overview","suites":[]}`, wantError: true},
		{name: "missing_cases", view: "cases", body: `{"view":"cases","pagination":{"totalCount":0}}`, wantError: true},
		{name: "missing_combos", view: "combos", identity: "case", body: `{"view":"combos","pagination":{"totalCount":0}}`, wantError: true},
		{name: "missing_pagination", view: "cases", body: `{"view":"cases","caseOverviews":[]}`, wantError: true},
		{name: "invalid_case", view: "cases", body: `{"view":"cases","caseOverviews":[{"identityKey":"case"}],"pagination":{"totalCount":1}}`, wantError: true},
		{name: "duplicate_cases", view: "cases", body: `{"view":"cases","caseOverviews":[{"identityKey":"case","comboCount":"1"},{"identityKey":"case","comboCount":"1"}],"pagination":{"totalCount":2}}`, wantError: true},
		{name: "oversized_response", view: "cases", size: 1, body: `{"view":"cases","caseOverviews":[{"identityKey":"a","comboCount":"1"},{"identityKey":"b","comboCount":"1"}],"pagination":{"totalCount":2}}`, wantError: true},
		{name: "repeated_token", view: "cases", token: "1", body: `{"view":"cases","caseOverviews":[{"identityKey":"a","comboCount":"1"}],"pagination":{"totalCount":3,"nextPageToken":"1"}}`, wantError: true},
		{name: "empty_continuation", view: "cases", body: `{"view":"cases","caseOverviews":[],"pagination":{"totalCount":1,"nextPageToken":"1"}}`, wantError: true},
		{name: "missing_params", view: "combos", identity: "case", body: `{"view":"combos","combos":[{"executionKey":"exec","resultId":"r"}],"pagination":{"totalCount":1}}`, wantError: true},
		{name: "unknown_view", view: "unknown", invalid: true, wantError: true},
		{name: "missing_identity", view: "combos", invalid: true, wantError: true},
		{name: "unexpected_identity", view: "cases", identity: "case", invalid: true, wantError: true},
		{name: "overview_page", view: "overview", size: 1, invalid: true, wantError: true},
		{name: "oversize", view: "cases", size: 201, invalid: true, wantError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Method != "GET" || r.URL.Path != "/v1/products/p/runs/42/summary" {
					t.Errorf("request: %s %s", r.Method, r.URL)
				}
				q := r.URL.Query()
				if q.Get("view") != tc.view || q.Get("identityKey") != tc.identity || q.Get("pagination.pageToken") != tc.token {
					t.Errorf("query: %v", q)
				}
				if tc.view == "overview" && q.Has("pagination.pageSize") {
					t.Error("overview was paginated")
				}
				if tc.view != "overview" && tc.size == 0 && q.Get("pagination.pageSize") != "100" {
					t.Error("wrong default page size")
				}
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()
			got, err := New(server.URL, "token").GetRunSummaryProjection(context.Background(), "p", 42, RunSummaryProjectionOptions{View: tc.view, IdentityKey: tc.identity, PageSize: tc.size, PageToken: tc.token})
			wantCalls := 1
			if tc.invalid {
				wantCalls = 0
			}
			if (err != nil) != tc.wantError || calls != wantCalls {
				t.Fatalf("calls=%d got=%+v err=%v", calls, got, err)
			}
			if tc.wantError && got != nil {
				t.Fatal("partial response on failure")
			}
			if !tc.wantError && got.View != tc.view {
				t.Fatal("view marker lost")
			}
			if tc.name == "case_page" && got.Pagination.NextPageToken != "2" {
				t.Fatal("continuation lost")
			}
			if tc.name == "combo_page" && (got.Combos[0].DurationMs != 42 || got.Combos[0].Params["browser"] != "firefox" || got.Combos[0].Attempts != 3) {
				t.Fatal("combo metadata lost")
			}
		})
	}
}

func TestRunSummaryProjection_HTTPFailure(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++; w.WriteHeader(http.StatusForbidden) }))
	defer server.Close()
	got, err := New(server.URL, "token").GetRunSummaryProjection(context.Background(), "p", 42, RunSummaryProjectionOptions{View: "overview"})
	if err == nil || got != nil || calls != 1 {
		t.Fatalf("failure fallback: %+v %v calls=%d", got, err, calls)
	}
}
