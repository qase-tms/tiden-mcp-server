package mcpserver_test

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
)

const (
	issueA = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	issueB = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
	specA  = "11111111-1111-4111-8111-111111111111"
	specB  = "22222222-2222-4222-8222-222222222222"
)

// linkRecorder answers the resolve and batchUpdate endpoints and records each
// request body, so a test can check how many calls a tool made and what it sent.
type linkRecorder struct {
	mu       sync.Mutex
	resolves []map[string]any
	updates  []map[string]any
	gets     []string
	resolve  string // response body for requirements:resolve
	update   string // response body for batchUpdate
	status   int    // status for batchUpdate; 0 means 200
}

func (l *linkRecorder) handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		l.mu.Lock()
		defer l.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		body, _ := io.ReadAll(r.Body)
		var parsed map[string]any
		_ = json.Unmarshal(body, &parsed)
		switch {
		case strings.HasSuffix(r.URL.Path, "/requirements:resolve"):
			l.resolves = append(l.resolves, parsed)
			_, _ = w.Write([]byte(l.resolve))
		case strings.HasSuffix(r.URL.Path, "/issue-requirement-links:batchUpdate"):
			l.updates = append(l.updates, parsed)
			if l.status != 0 {
				w.WriteHeader(l.status)
			}
			_, _ = w.Write([]byte(l.update))
		case strings.HasSuffix(r.URL.Path, "/issues") && strings.Contains(r.URL.Path, "/v1/requirements/"):
			l.gets = append(l.gets, r.URL.String())
			_, _ = w.Write([]byte(`{"issues":[{"id":"` + issueA + `","title":"boom","status":"unresolved"}],"totalCount":1}`))
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"code":5,"message":"unexpected ` + r.URL.Path + `"}`))
		}
	}
}

func pairs(t *testing.T, body map[string]any, key string) []map[string]any {
	t.Helper()
	raw, _ := body[key].([]any)
	out := make([]map[string]any, len(raw))
	for i, p := range raw {
		out[i], _ = p.(map[string]any)
	}
	return out
}

func TestLinkIssueSpecs_ResolvesKeysOnceThenWritesAllPairs(t *testing.T) {
	rec := &linkRecorder{
		resolve: `{"requirements":[{"ref":"AUTH-3","status":"ok","viewId":"` + specA + `","canonicalId":"` + specA + `","seqNum":3},` +
			`{"ref":"AUTH-4","status":"ok","viewId":"` + specB + `","canonicalId":"` + specB + `","seqNum":4}],"branchExists":true}`,
		update: `{"results":[{"issueId":"` + issueA + `","requirementId":"` + specA + `","outcome":"linked"}]}`,
	}
	args := map[string]any{"product_id": "p1", "links": []any{
		map[string]any{"issue_id": issueA, "spec_ref": "AUTH-3"},
		map[string]any{"issue_id": issueA, "spec_ref": "AUTH-4"},
		map[string]any{"issue_id": issueB, "spec_ref": "AUTH-3"},
	}}
	body, isErr := callTool(t, rec.handler(), "link_issue_specs", args)
	if isErr {
		t.Fatalf("tool error: %v", body)
	}
	if len(rec.resolves) != 1 || len(rec.updates) != 1 {
		t.Fatalf("calls: %d resolve, %d update; want 1 and 1", len(rec.resolves), len(rec.updates))
	}
	if refs, _ := rec.resolves[0]["refs"].([]any); len(refs) != 2 {
		t.Errorf("resolve refs = %v, want the two distinct keys", rec.resolves[0]["refs"])
	}
	add := pairs(t, rec.updates[0], "add")
	if len(add) != 3 || add[0]["requirementId"] != specA || add[1]["requirementId"] != specB || add[2]["issueId"] != issueB {
		t.Errorf("add = %v", add)
	}
	if _, has := rec.updates[0]["remove"]; has {
		t.Errorf("link must not send remove: %v", rec.updates[0])
	}
}

func TestLinkIssueSpecs_UUIDRefsSkipResolve(t *testing.T) {
	rec := &linkRecorder{update: `{"results":[]}`}
	args := map[string]any{"product_id": "p1", "links": []any{map[string]any{"issue_id": issueA, "spec_ref": specA}}}
	if body, isErr := callTool(t, rec.handler(), "link_issue_specs", args); isErr {
		t.Fatalf("tool error: %v", body)
	}
	if len(rec.resolves) != 0 || len(rec.updates) != 1 {
		t.Fatalf("calls: %d resolve, %d update; want 0 and 1", len(rec.resolves), len(rec.updates))
	}
}

func TestLinkIssueSpecs_UnresolvedRefWritesNothing(t *testing.T) {
	rec := &linkRecorder{resolve: `{"requirements":[{"ref":"AUTH-99","status":"unresolved"}],"branchExists":true}`}
	args := map[string]any{"product_id": "p1", "links": []any{map[string]any{"issue_id": issueA, "spec_ref": "AUTH-99"}}}
	body, isErr := callTool(t, rec.handler(), "link_issue_specs", args)
	if !isErr {
		t.Fatalf("expected tool error, got %v", body)
	}
	if text, _ := body["error"].(string); !strings.Contains(text, "AUTH-99") {
		t.Errorf("error must name the ref: %v", body)
	}
	if len(rec.updates) != 0 {
		t.Errorf("no write may happen for an unresolved ref")
	}
}

func TestLinkIssueSpecs_BadArgsFailBeforeAnyCall(t *testing.T) {
	tooMany := make([]any, 201)
	for i := range tooMany {
		tooMany[i] = map[string]any{"issue_id": issueA, "spec_ref": specA}
	}
	for name, links := range map[string]any{
		"empty":       []any{},
		"over limit":  tooMany,
		"no spec ref": []any{map[string]any{"issue_id": issueA}},
	} {
		t.Run(name, func(t *testing.T) {
			rec := &linkRecorder{}
			body, isErr := callTool(t, rec.handler(), "link_issue_specs", map[string]any{"product_id": "p1", "links": links})
			if !isErr {
				t.Fatalf("expected tool error, got %v", body)
			}
			if len(rec.resolves)+len(rec.updates) != 0 {
				t.Errorf("no API call may happen: %d resolve, %d update", len(rec.resolves), len(rec.updates))
			}
		})
	}
}

func TestLinkIssueSpecs_ServerErrorIsToolError(t *testing.T) {
	rec := &linkRecorder{
		status: http.StatusNotFound,
		update: `{"code":5,"message":"NOT_FOUND: add[0]: issue not found in this product"}`,
	}
	args := map[string]any{"product_id": "p1", "links": []any{map[string]any{"issue_id": issueA, "spec_ref": specA}}}
	body, isErr := callTool(t, rec.handler(), "link_issue_specs", args)
	if !isErr {
		t.Fatalf("expected tool error, got %v", body)
	}
	if text, _ := body["error"].(string); !strings.Contains(text, "add[0]") {
		t.Errorf("tool error must carry the server message: %v", body)
	}
}

func TestUnlinkIssueSpecs_SendsRemove(t *testing.T) {
	rec := &linkRecorder{update: `{"results":[{"issueId":"` + issueA + `","requirementId":"` + specA + `","outcome":"unlinked"}]}`}
	args := map[string]any{"product_id": "p1", "links": []any{map[string]any{"issue_id": issueA, "spec_ref": specA}}}
	body, isErr := callTool(t, rec.handler(), "unlink_issue_specs", args)
	if isErr {
		t.Fatalf("tool error: %v", body)
	}
	if len(rec.updates) != 1 || len(pairs(t, rec.updates[0], "remove")) != 1 {
		t.Fatalf("updates = %v", rec.updates)
	}
	if _, has := rec.updates[0]["add"]; has {
		t.Errorf("unlink must not send add: %v", rec.updates[0])
	}
}

func TestListSpecIssues_ResolvesTheRefAndReadsItsIssues(t *testing.T) {
	rec := &linkRecorder{resolve: `{"requirements":[{"ref":"AUTH-3","status":"ok","viewId":"` + specA + `","canonicalId":"` + specA + `"}],"branchExists":true}`}
	body, isErr := callTool(t, rec.handler(), "list_spec_issues", map[string]any{"product_id": "p1", "spec_ref": "AUTH-3", "limit": 10})
	if isErr {
		t.Fatalf("tool error: %v", body)
	}
	if len(rec.gets) != 1 || !strings.Contains(rec.gets[0], "/v1/requirements/"+specA+"/issues") || !strings.Contains(rec.gets[0], "limit=10") {
		t.Fatalf("gets = %v", rec.gets)
	}
	if body["totalCount"] != float64(1) {
		t.Errorf("result = %v", body)
	}
}

func TestGetIssue_PassesLinkedRequirements(t *testing.T) {
	resp := `{"issue":{"id":"` + issueA + `","title":"boom","status":"unresolved"},` +
		`"linkedRequirements":[{"requirementId":"` + specA + `","seqNum":3,"title":"Login","status":"Active","branchName":"intent/x","branchClosed":true}]}`
	body, isErr := callTool(t, jsonHandler(http.StatusOK, resp), "get_issue", map[string]any{"id": issueA})
	if isErr {
		t.Fatalf("tool error: %v", body)
	}
	linked, _ := body["linkedRequirements"].([]any)
	if len(linked) != 1 {
		t.Fatalf("linkedRequirements = %v", body["linkedRequirements"])
	}
	first, _ := linked[0].(map[string]any)
	if first["requirementId"] != specA || first["branchName"] != "intent/x" || first["branchClosed"] != true {
		t.Errorf("linked spec = %v", first)
	}
}

func TestGetIssueFixContext_PassesLinksAndNextStep(t *testing.T) {
	resp := `{"context":{"issue":{"id":"` + issueA + `"},` +
		`"suspectRequirements":[{"id":"` + specA + `","seqNum":3,"title":"Login","coverage":"verified","linked":true}],` +
		`"linkedRequirements":[{"requirement":{"requirementId":"` + specA + `","seqNum":3,"title":"Login"},"coverage":"verified"}],` +
		`"nextStep":"link the spec"}}`
	body, isErr := callTool(t, jsonHandler(http.StatusOK, resp), "get_issue_fix_context", map[string]any{"product_id": "p1", "issue_id": issueA})
	if isErr {
		t.Fatalf("tool error: %v", body)
	}
	raw, _ := json.Marshal(body)
	for _, want := range []string{`"linked":true`, `"linkedRequirements"`, `"nextStep":"link the spec"`} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("fix context result lacks %s: %s", want, raw)
		}
	}
}
