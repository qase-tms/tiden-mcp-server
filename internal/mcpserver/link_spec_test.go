package mcpserver_test

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

const (
	linkTestID = "33333333-3333-4333-8333-333333333333"
	linkReqID  = "44444444-4444-4444-8444-444444444444"
)

func linkProposalResponse(status string) string {
	return `{"branch":"intent/x","link":null,"proposal":{"id":"p-1","testId":"` + linkTestID +
		`","requirementId":"` + linkReqID + `","testSeqNum":412,"requirementSeqNum":88,"status":"` + status + `"}}`
}

func jsonHandler(status int, response string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(response))
	}
}

func linkArgs(extra map[string]any) map[string]any {
	args := map[string]any{"test_id": linkTestID, "spec_id": linkReqID}
	for k, v := range extra {
		args[k] = v
	}
	return args
}

func TestLinkSpec_ReturnsBranchProposal(t *testing.T) {
	body, isErr := callTool(t, jsonHandler(http.StatusOK, linkProposalResponse("proposed")), "link_spec", linkArgs(map[string]any{"branch": "intent/x"}))
	if isErr {
		t.Fatalf("tool error: %v", body)
	}
	if body["status"] != "proposed" || body["branch"] != "intent/x" || body["testId"] != linkTestID || body["requirementId"] != linkReqID {
		t.Errorf("result = %v", body)
	}
	if len(body) != 4 {
		t.Errorf("result must be flat with four keys, got %v", body)
	}
}

func TestLinkSpec_ReturnsMainLink(t *testing.T) {
	resp := `{"branch":"main","link":{"id":"l-1","testId":"` + linkTestID + `","requirementId":"` + linkReqID + `","createdAt":"2026-10-28T09:00:00Z"},"proposal":null}`
	body, isErr := callTool(t, jsonHandler(http.StatusOK, resp), "link_spec", linkArgs(nil))
	if isErr {
		t.Fatalf("tool error: %v", body)
	}
	if body["status"] != "linked" || body["branch"] != "main" {
		t.Errorf("result = %v", body)
	}
}

func TestLinkSpec_RejectedProposalStatus(t *testing.T) {
	body, isErr := callTool(t, jsonHandler(http.StatusOK, linkProposalResponse("rejected")), "link_spec", linkArgs(map[string]any{"branch": "intent/x"}))
	if isErr {
		t.Fatalf("tool error: %v", body)
	}
	if body["status"] != "rejected" {
		t.Errorf("status = %v, want rejected", body["status"])
	}
}

func TestLinkSpec_OlderServerFallsBackToLinked(t *testing.T) {
	body, isErr := callTool(t, jsonHandler(http.StatusOK, `{}`), "link_spec", linkArgs(map[string]any{"branch": "intent/x"}))
	if isErr {
		t.Fatalf("tool error: %v", body)
	}
	if body["status"] != "linked" || body["testId"] != linkTestID || body["requirementId"] != linkReqID {
		t.Errorf("result = %v", body)
	}
	if _, has := body["branch"]; has {
		t.Errorf("branch must be absent when the server reports none: %v", body)
	}
}

func TestLinkSpec_UnknownBranchIsToolError(t *testing.T) {
	msg := `{"code":5,"message":"BRANCH_NOT_FOUND: branch \"intent/typo\" not found; to create it, set create_branch"}`
	body, isErr := callTool(t, jsonHandler(http.StatusNotFound, msg), "link_spec", linkArgs(map[string]any{"branch": "intent/typo"}))
	if !isErr {
		t.Fatalf("expected tool error, got %v", body)
	}
	text, _ := body["error"].(string)
	if !strings.Contains(text, "BRANCH_NOT_FOUND") {
		t.Errorf("error text = %q, want the server's BRANCH_NOT_FOUND text", text)
	}
}

func TestLinkSpec_CreateBranchArgSent(t *testing.T) {
	var sent []byte
	handler := func(w http.ResponseWriter, r *http.Request) {
		sent, _ = io.ReadAll(r.Body)
		jsonHandler(http.StatusOK, linkProposalResponse("proposed"))(w, r)
	}
	if _, isErr := callTool(t, handler, "link_spec", linkArgs(map[string]any{"branch": "intent/new", "create_branch": true})); isErr {
		t.Fatal("tool error")
	}
	if !strings.Contains(string(sent), `"createBranch":true`) {
		t.Errorf("request body = %s, want createBranch true", sent)
	}

	sent = nil
	if _, isErr := callTool(t, handler, "link_spec", linkArgs(map[string]any{"branch": "intent/new"})); isErr {
		t.Fatal("tool error")
	}
	if strings.Contains(string(sent), "createBranch") {
		t.Errorf("createBranch must be omitted when false: %s", sent)
	}
}

func TestGetMergePreview_CarriesLinkProposals(t *testing.T) {
	resp := `{"additions":[],"modifications":[],"deletions":[],"stats":{"additions":0,"modifications":0,"deletions":0,"conflicts":0},` +
		`"linkProposals":[{"id":"p-1","testId":"` + linkTestID + `","requirementId":"` + linkReqID + `","testSeqNum":412,"requirementSeqNum":88,"status":"proposed"},` +
		`{"id":"p-2","testId":"` + linkTestID + `","requirementId":"` + linkReqID + `","testSeqNum":413,"requirementSeqNum":89,"status":"accepted"}]}`
	body, isErr := callTool(t, jsonHandler(http.StatusOK, resp), "get_merge_preview", map[string]any{"branch_id": "branch-1"})
	if isErr {
		t.Fatalf("tool error: %v", body)
	}
	rows, _ := body["linkProposals"].([]any)
	if len(rows) != 2 {
		t.Fatalf("linkProposals = %v, want 2 rows", body["linkProposals"])
	}
	first, _ := rows[0].(map[string]any)
	if first["testSeqNum"] != float64(412) || first["requirementSeqNum"] != float64(88) || first["status"] != "proposed" {
		t.Errorf("first row = %v", first)
	}
	for _, row := range rows {
		m, _ := row.(map[string]any)
		if len(m) != 3 {
			t.Errorf("row must carry only three keys: %v", m)
		}
		for _, k := range []string{"id", "testId", "requirementId"} {
			if _, has := m[k]; has {
				t.Errorf("row carries %s: %v", k, m)
			}
		}
	}
}
