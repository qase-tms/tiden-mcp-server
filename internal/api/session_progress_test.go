package api

import (
	"context"
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
		      "status": "failed", "fromSession": true, "runSeq": 42, "proposed": true
		    }]
		  }],
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

	if len(resp.Requirements) != 1 {
		t.Fatalf("requirements = %+v", resp.Requirements)
	}
	req := resp.Requirements[0]
	if req.RequirementID != "r1" || req.Display != "QA-57" || req.Coverage != "failing" ||
		!req.ProposedOnly || !req.MovedThisSession {
		t.Errorf("requirement decode = %+v", req)
	}
	if len(req.Tests) != 1 {
		t.Fatalf("tests = %+v", req.Tests)
	}
	tc := req.Tests[0]
	if tc.TestID != "t1" || tc.Display != "QA-3" || tc.Status != "failed" ||
		!tc.FromSession || tc.RunSeq != 42 || !tc.Proposed {
		t.Errorf("test decode = %+v", tc)
	}
	if resp.Summary.Total != 3 || resp.Summary.Verified != 1 || resp.Summary.Failing != 1 ||
		resp.Summary.NotRun != 0 || resp.Summary.NoTest != 1 {
		t.Errorf("summary = %+v", resp.Summary)
	}
	if resp.Ready {
		t.Error("ready must decode false")
	}
	if len(resp.NextActions) != 1 || resp.NextActions[0] != "fix the failing test and re-run" {
		t.Errorf("nextActions = %v", resp.NextActions)
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
