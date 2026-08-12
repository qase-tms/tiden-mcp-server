package api

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestRecordSessionRiskAcceptances pins the exact wire contract: POST path
// and body shape (mirroring tiden-cli's internal/api.SessionRiskAcceptance so
// the two clients cannot drift), and full response decode.
func TestRecordSessionRiskAcceptances(t *testing.T) {
	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/products/p1/quality-gate:session-acceptances" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.String())
		}
		data, _ := io.ReadAll(r.Body)
		gotBody = string(data)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"acceptancesRecorded": 1, "deferredRequirements": 2, "replacedRows": 3}`))
	}))
	defer srv.Close()

	client := newTestClient(srv.URL)
	resp, err := client.RecordSessionRiskAcceptances(context.Background(), "p1", RecordSessionRiskAcceptancesRequest{
		RequirementID: "draft-1",
		IntentBranch:  "intent/2026-08-05-x",
		SessionID:     "3f0e8c1a-2b4d-4e6f-8a9b-0c1d2e3f4a5b",
		Acceptances: []SessionRiskAcceptance{{
			RequirementRefs: []string{"FEED-49"},
			Criterion:       "R1",
			Evidence:        "no sandbox to run this against",
			FollowUp:        "none",
		}},
		ProposedTestRequirementRefs: []string{"FEED-34", "FEED-36"},
	})
	if err != nil {
		t.Fatalf("RecordSessionRiskAcceptances: %v", err)
	}

	wantBody := `{"requirementId":"draft-1","intentBranch":"intent/2026-08-05-x","sessionId":"3f0e8c1a-2b4d-4e6f-8a9b-0c1d2e3f4a5b","acceptances":[{"requirementRefs":["FEED-49"],"criterion":"R1","evidence":"no sandbox to run this against","followUp":"none"}],"proposedTestRequirementRefs":["FEED-34","FEED-36"]}`
	if gotBody != wantBody {
		t.Errorf("request body = %s, want %s", gotBody, wantBody)
	}

	if resp.AcceptancesRecorded != 1 || resp.DeferredRequirements != 2 || resp.ReplacedRows != 3 {
		t.Errorf("response decode = %+v", resp)
	}
}

// TestRecordSessionRiskAcceptancesOmitsEmptyCollections pins that a
// deferral-only call (no acceptances) omits the acceptances key rather than
// sending an empty array, and vice versa.
func TestRecordSessionRiskAcceptancesOmitsEmptyCollections(t *testing.T) {
	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		gotBody = string(data)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"acceptancesRecorded": 0, "deferredRequirements": 1, "replacedRows": 0}`))
	}))
	defer srv.Close()

	client := newTestClient(srv.URL)
	if _, err := client.RecordSessionRiskAcceptances(context.Background(), "p1", RecordSessionRiskAcceptancesRequest{
		RequirementID:               "draft-1",
		IntentBranch:                "intent/2026-08-05-x",
		SessionID:                   "s1",
		ProposedTestRequirementRefs: []string{"FEED-34"},
	}); err != nil {
		t.Fatalf("RecordSessionRiskAcceptances: %v", err)
	}
	wantBody := `{"requirementId":"draft-1","intentBranch":"intent/2026-08-05-x","sessionId":"s1","proposedTestRequirementRefs":["FEED-34"]}`
	if gotBody != wantBody {
		t.Errorf("request body = %s, want %s", gotBody, wantBody)
	}
}

// TestRecordSessionRiskAcceptances_ServerRefusalSurfaced pins that a
// structural refusal (e.g. unknown criterion, missing requirement_id) comes
// back as an *APIError whose Message is the server's own text — the refusal
// text is what teaches the agent, so it must not be swallowed or reworded.
func TestRecordSessionRiskAcceptances_ServerRefusalSurfaced(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"code":3,"message":"QG_INVALID: criterion must be one of R1..R5"}`))
	}))
	defer srv.Close()

	client := newTestClient(srv.URL)
	_, err := client.RecordSessionRiskAcceptances(context.Background(), "p1", RecordSessionRiskAcceptancesRequest{
		RequirementID: "draft-1",
		IntentBranch:  "intent/2026-08-05-x",
		SessionID:     "s1",
		Acceptances: []SessionRiskAcceptance{{
			RequirementRefs: []string{"FEED-49"},
			Criterion:       "R9",
			Evidence:        "x",
			FollowUp:        "none",
		}},
	})
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("err = %v, want *APIError", err)
	}
	if apiErr.HTTPStatus != http.StatusBadRequest || apiErr.Message != "QG_INVALID: criterion must be one of R1..R5" {
		t.Errorf("apiErr = %+v", apiErr)
	}
}
