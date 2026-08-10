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
		_, _ = w.Write([]byte(`{"recordedCount":1,"deduplicatedCount":2}`))
	}))
	defer server.Close()

	client := New(server.URL, "token")
	got, err := client.RecordSessionRiskAcceptances(context.Background(), "p1", RecordSessionRiskAcceptancesRequest{
		RequirementID: "draft-1", IntentBranch: "intent/x", SessionID: "session-1",
		Acceptances: []SessionRiskAcceptance{{RequirementIDs: []string{"req-1"}, Criterion: "R4", Evidence: "provider sandbox covers the contract", FollowUp: "issue:QA-42"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.RecordedCount != 1 || got.DeduplicatedCount != 2 {
		t.Fatalf("response = %+v", got)
	}
	if body["requirementId"] != "draft-1" || body["intentBranch"] != "intent/x" {
		t.Fatalf("body = %+v", body)
	}
}
