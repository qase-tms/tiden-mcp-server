package mcpserver_test

import (
	"net/http"
	"strings"
	"testing"
)

func TestRecordRiskAcceptances_StructuralRefusalSendsNothing(t *testing.T) {
	rec := &holdRecorder{response: `{}`}
	body, isErr := callTool(t, rec.handler(t), "record_risk_acceptances", map[string]any{
		"product_id":    "p1",
		"intent_branch": "intent/2026-10-29-example",
		"session_id":    "s1",
	})
	if !isErr {
		t.Fatalf("expected a tool error, got %v", body)
	}
	if got := rec.count(); got != 0 {
		t.Errorf("server saw %d requests, want 0", got)
	}
}

func TestRecordRiskAcceptances_PostsOnceAndRelaysRefusal(t *testing.T) {
	const refusal = "criterion R9 is not admissible"
	rec := &holdRecorder{
		status:   http.StatusBadRequest,
		response: `{"code":3,"message":"` + refusal + `","details":[{"reason":"QG_INVALID"}]}`,
	}
	body, isErr := callTool(t, rec.handler(t), "record_risk_acceptances", map[string]any{
		"product_id":    "p1",
		"intent_branch": "intent/2026-10-29-example",
		"session_id":    "s1",
		"acceptances": []map[string]any{{
			"spec_refs": []string{"FEED-49"},
			"criterion": "R9",
			"evidence":  "cannot run here",
			"follow_up": "none",
		}},
	})
	if !isErr {
		t.Fatalf("expected a tool error, got %v", body)
	}

	req := rec.only(t)
	if req.method != http.MethodPost || req.path != "/v1/products/p1/quality-gate:session-acceptances" {
		t.Errorf("request = %s %s, want POST /v1/products/p1/quality-gate:session-acceptances", req.method, req.path)
	}
	if text, _ := body["error"].(string); !strings.Contains(text, refusal) {
		t.Errorf("tool error %q does not carry the server message %q", text, refusal)
	}
}
