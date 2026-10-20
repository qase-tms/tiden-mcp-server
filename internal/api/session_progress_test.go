package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestGetSessionProgress pins the exact wire contract: POST path and body
// (intentBranch present), and full response decode.
func TestGetSessionProgress(t *testing.T) {
	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/products/p1/quality-gate:session-progress" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.String())
		}
		data, _ := io.ReadAll(r.Body)
		gotBody = string(data)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
		  "requirements": [{
		    "requirementId": "r1", "display": "QA-57", "title": "Session progress",
		    "coverage": "failing", "proposedOnly": true, "movedThisSession": true,
		    "tests": [{
		      "testId": "t1", "display": "QA-3", "title": "reports progress",
		      "status": "failed", "fromSession": true, "runSeq": 42, "proposed": true, "signature": "go/pkg::TestX"
		    }],
		    "canonicalId": "m1", "testCount": 1
		  }],
		  "intentBranchStatus": "open", "freshnessApplied": true,
		  "summary": {"total": 3, "verified": 1, "failing": 1, "notRun": 0, "noTest": 1},
		  "ready": false,
		  "nextActions": ["fix the failing test and re-run"]
		}`))
	}))
	defer srv.Close()

	client := newTestClient(srv.URL)
	resp, err := client.GetSessionProgress(context.Background(), "p1", SessionProgressQuery{
		SessionID:      "3f0e8c1a-2b4d-4e6f-8a9b-0c1d2e3f4a5b",
		RequirementIDs: []string{"r1", "r2"},
		IntentBranch:   "intent/2026-08-05-x",
	})
	if err != nil {
		t.Fatalf("GetSessionProgress: %v", err)
	}

	wantBody := `{"sessionId":"3f0e8c1a-2b4d-4e6f-8a9b-0c1d2e3f4a5b","requirementIds":["r1","r2"],"intentBranch":"intent/2026-08-05-x"}`
	if gotBody != wantBody {
		t.Errorf("request body = %s, want %s", gotBody, wantBody)
	}

	// The response is passed through as sent, including fields this client
	// has no struct for.
	var got struct {
		Requirements []struct {
			RequirementID string `json:"requirementId"`
			CanonicalID   string `json:"canonicalId"`
			Coverage      string `json:"coverage"`
			Tests         []struct {
				Signature string `json:"signature"`
				RunSeq    int    `json:"runSeq"`
			} `json:"tests"`
		} `json:"requirements"`
		IntentBranchStatus string `json:"intentBranchStatus"`
		FreshnessApplied   bool   `json:"freshnessApplied"`
		Ready              bool   `json:"ready"`
	}
	if err := json.Unmarshal(resp, &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got.Requirements) != 1 {
		t.Fatalf("requirements = %s", resp)
	}
	req := got.Requirements[0]
	if req.RequirementID != "r1" || req.CanonicalID != "m1" || req.Coverage != "failing" ||
		len(req.Tests) != 1 || req.Tests[0].Signature != "go/pkg::TestX" || req.Tests[0].RunSeq != 42 {
		t.Errorf("requirement decode = %+v", req)
	}
	if got.IntentBranchStatus != "open" || !got.FreshnessApplied || got.Ready {
		t.Errorf("top-level fields = %s", resp)
	}
}

// TestGetSessionProgressOmitsEmptyIntentBranch pins that an empty intent
// branch is omitted from the body, not sent as "".
func TestGetSessionProgressOmitsEmptyIntentBranch(t *testing.T) {
	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		gotBody = string(data)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"summary": {"total": 0, "verified": 0, "failing": 0, "notRun": 0, "noTest": 0}}`))
	}))
	defer srv.Close()

	client := newTestClient(srv.URL)
	if _, err := client.GetSessionProgress(context.Background(), "p1", SessionProgressQuery{
		SessionID:      "s1",
		RequirementIDs: []string{"r1"},
	}); err != nil {
		t.Fatalf("GetSessionProgress: %v", err)
	}
	wantBody := `{"sessionId":"s1","requirementIds":["r1"]}`
	if gotBody != wantBody {
		t.Errorf("request body = %s, want %s", gotBody, wantBody)
	}
}

// TestGetSessionProgressSendsCompactTests pins the compactTests field name.
func TestGetSessionProgressSendsCompactTests(t *testing.T) {
	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		gotBody = string(data)
		_, _ = w.Write([]byte(`{"summary": {}}`))
	}))
	defer srv.Close()
	if _, err := newTestClient(srv.URL).GetSessionProgress(context.Background(), "p1", SessionProgressQuery{
		SessionID: "s1", RequirementIDs: []string{"r1"}, CompactTests: true,
	}); err != nil {
		t.Fatal(err)
	}
	if want := `{"sessionId":"s1","requirementIds":["r1"],"compactTests":true}`; gotBody != want {
		t.Errorf("request body = %s, want %s", gotBody, want)
	}
}
