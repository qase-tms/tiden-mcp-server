package mcpserver_test

import (
	"net/http"
	"testing"
)

func TestRunLifecycleToolsHitTheirEndpoints(t *testing.T) {
	const runBody = `{"run":{"id":"r1","productId":"p1","seqNum":42,"status":"new"}}`
	for _, tc := range []struct {
		tool, response string
		args           map[string]any
		method, path   string
	}{
		{"create_test_run", runBody, map[string]any{"product_id": "p1", "title": "Nightly"}, http.MethodPost, "/v1/products/p1/runs"},
		{"complete_test_run", runBody, map[string]any{"product_id": "p1", "run_seq": 42}, http.MethodPost, "/v1/products/p1/runs/42:complete"},
		{"abort_test_run", runBody, map[string]any{"product_id": "p1", "run_seq": 42}, http.MethodPost, "/v1/products/p1/runs/42:abort"},
		{"list_test_runs", `{"runs":[],"pagination":{}}`, map[string]any{"product_id": "p1"}, http.MethodGet, "/v1/products/p1/runs"},
		{"get_test_run", runBody, map[string]any{"product_id": "p1", "run_seq": 42}, http.MethodGet, "/v1/products/p1/runs/42"},
	} {
		t.Run(tc.tool, func(t *testing.T) {
			rec := &holdRecorder{response: tc.response}
			body, isErr := callTool(t, rec.handler(t), tc.tool, tc.args)
			if isErr {
				t.Fatalf("tool error: %v", body)
			}
			req := rec.only(t)
			if req.method != tc.method || req.path != tc.path {
				t.Errorf("request = %s %s, want %s %s", req.method, req.path, tc.method, tc.path)
			}
		})
	}
}
