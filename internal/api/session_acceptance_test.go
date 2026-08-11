package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRecordSessionRiskAcceptances(t *testing.T) {
	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/products/p1/quality-gate:session-acceptances" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"acceptancesRecorded":1,"deferredRequirements":2,"replacedRows":3}`))
	}))
	defer server.Close()

	client := New(server.URL, "token")
	got, err := client.RecordSessionRiskAcceptances(context.Background(), "p1", RecordSessionRiskAcceptancesRequest{
		RequirementID: "draft-1", IntentBranch: "intent/x", SessionID: "session-1",
		Acceptances:                 []SessionRiskAcceptance{{RequirementRefs: []string{"QA-1"}, Criterion: "R4", Evidence: "provider sandbox covers the contract", FollowUp: "issue:QA-42"}},
		ProposedTestRequirementRefs: []string{"QA-2"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.AcceptancesRecorded != 1 || got.DeferredRequirements != 2 || got.ReplacedRows != 3 {
		t.Fatalf("response = %+v", got)
	}
	if body["requirementId"] != "draft-1" || body["intentBranch"] != "intent/x" {
		t.Fatalf("body = %+v", body)
	}
	acceptances, ok := body["acceptances"].([]any)
	if !ok || len(acceptances) != 1 {
		t.Fatalf("acceptances = %#v", body["acceptances"])
	}
	first, _ := acceptances[0].(map[string]any)
	if refs, ok := first["requirementRefs"].([]any); !ok || len(refs) != 1 || refs[0] != "QA-1" {
		t.Fatalf("requirementRefs = %#v", first["requirementRefs"])
	}
	if refs, ok := body["proposedTestRequirementRefs"].([]any); !ok || len(refs) != 1 || refs[0] != "QA-2" {
		t.Fatalf("proposedTestRequirementRefs = %#v", body["proposedTestRequirementRefs"])
	}
}
