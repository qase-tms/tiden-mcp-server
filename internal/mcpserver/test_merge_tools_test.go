package mcpserver_test

import (
	"net/http"
	"strings"
	"testing"
)

func TestMergeTestCasesCallsTheMergeEndpoint(t *testing.T) {
	rec := &holdRecorder{response: `{"mergeId":"m-1","survivorSeq":57,"mergedSeq":61,"queued":true}`}
	body, isErr := callTool(t, rec.handler(t), "merge_test_cases", map[string]any{
		"product_id": "p1", "ref": " 61 ", "into_ref": "57", "reason": " renamed in #812 ",
	})
	if isErr {
		t.Fatalf("tool error: %v", body)
	}
	req := rec.only(t)
	if req.method != http.MethodPost || req.path != "/v1/products/p1/tests/61:merge" {
		t.Errorf("request = %s %s, want POST /v1/products/p1/tests/61:merge", req.method, req.path)
	}
	if req.auth != "Bearer token" {
		t.Errorf("auth = %q", req.auth)
	}
	if req.body["intoRef"] != "57" || req.body["reason"] != "renamed in #812" {
		t.Errorf("body = %v, want intoRef=57 and the trimmed reason", req.body)
	}
	if body["mergeId"] != "m-1" || body["survivorSeq"] != float64(57) || body["mergedSeq"] != float64(61) || body["queued"] != true {
		t.Errorf("result = %v", body)
	}
}

func TestMergeTestCasesAcceptsUUIDsAndOmitsAnEmptyReason(t *testing.T) {
	rec := &holdRecorder{response: `{"mergeId":"m-1","survivorSeq":57,"mergedSeq":61,"queued":false}`}
	_, isErr := callTool(t, rec.handler(t), "merge_test_cases", map[string]any{
		"product_id": "p1", "ref": "7f1c1c52-0000-4000-8000-000000000001", "into_ref": "7f1c1c52-0000-4000-8000-000000000002",
	})
	if isErr {
		t.Fatal("tool error")
	}
	req := rec.only(t)
	if req.path != "/v1/products/p1/tests/7f1c1c52-0000-4000-8000-000000000001:merge" {
		t.Errorf("path = %s", req.path)
	}
	if _, set := req.body["reason"]; set {
		t.Errorf("an empty reason must not be sent: %v", req.body)
	}
}

func TestMergeTestCasesValidatesBeforeSending(t *testing.T) {
	for name, args := range map[string]map[string]any{
		"no product": {"ref": "61", "into_ref": "57"},
		"no ref":     {"product_id": "p1", "ref": "  ", "into_ref": "57"},
		"no into":    {"product_id": "p1", "ref": "61"},
	} {
		t.Run(name, func(t *testing.T) {
			rec := &holdRecorder{response: `{}`}
			body, isErr := callTool(t, rec.handler(t), "merge_test_cases", args)
			if !isErr {
				t.Fatalf("expected a tool error, got %v", body)
			}
			if rec.count() != 0 {
				t.Errorf("a request was sent for an invalid call")
			}
		})
	}
}

func TestMergeTestCasesRefusalIsAToolErrorWithTheServerMessage(t *testing.T) {
	rec := &holdRecorder{status: http.StatusConflict, response: `{"code":9,"message":"merge refused: the pair was split by merge m-9"}`}
	body, isErr := callTool(t, rec.handler(t), "merge_test_cases", map[string]any{"product_id": "p1", "ref": "61", "into_ref": "57"})
	if !isErr {
		t.Fatalf("a refused merge must be a tool error, got %v", body)
	}
	if text, _ := body["error"].(string); !strings.Contains(text, "split by merge m-9") {
		t.Errorf("error = %v, want the server's reason", body)
	}
}

func TestUnmergeTestCasesCallsTheUndoEndpointAndReportsBlocked(t *testing.T) {
	rec := &holdRecorder{response: `{"restoredSeq":61,"ambiguousAfterMerge":2}`}
	body, isErr := callTool(t, rec.handler(t), "unmerge_test_cases", map[string]any{"product_id": "p1", "merge_id": "m-1"})
	if isErr {
		t.Fatalf("tool error: %v", body)
	}
	req := rec.only(t)
	if req.method != http.MethodPost || req.path != "/v1/products/p1/test-merges/m-1:undo" {
		t.Errorf("request = %s %s", req.method, req.path)
	}
	if body["restoredSeq"] != float64(61) || body["ambiguousAfterMerge"] != float64(2) || body["blocked"] != true {
		t.Errorf("result = %v", body)
	}
}

func TestUnmergeTestCasesRefusalNamesTheNewerMerge(t *testing.T) {
	rec := &holdRecorder{status: http.StatusConflict, response: `{"code":9,"message":"undo merge m-2 first: it is newer in this chain"}`}
	body, isErr := callTool(t, rec.handler(t), "unmerge_test_cases", map[string]any{"product_id": "p1", "merge_id": "m-1"})
	if !isErr {
		t.Fatalf("expected a tool error, got %v", body)
	}
	if text, _ := body["error"].(string); !strings.Contains(text, "undo merge m-2 first") {
		t.Errorf("error = %v", body)
	}
	if _, isErr := callTool(t, (&holdRecorder{response: `{}`}).handler(t), "unmerge_test_cases", map[string]any{"product_id": "p1", "merge_id": " "}); !isErr {
		t.Errorf("an empty merge_id must be refused")
	}
}

func TestListTestMergesFiltersByCase(t *testing.T) {
	rec := &holdRecorder{response: `{"merges":[{"mergeId":"m-2","survivorId":"s","survivorSeq":57,"mergedId":"l","mergedSeq":61,"status":"merged","reason":"twin","createdAt":"2026-10-23T10:00:00Z"}]}`}
	body, isErr := callTool(t, rec.handler(t), "list_test_merges", map[string]any{"product_id": "p1", "test_ref": "57"})
	if isErr {
		t.Fatalf("tool error: %v", body)
	}
	req := rec.only(t)
	if req.method != http.MethodGet || req.path != "/v1/products/p1/test-merges" {
		t.Errorf("request = %s %s", req.method, req.path)
	}
	merges, _ := body["merges"].([]any)
	if len(merges) != 1 || merges[0].(map[string]any)["status"] != "merged" {
		t.Errorf("result = %v", body)
	}
}

func TestListTestMergesQueryCarriesTheCaseRef(t *testing.T) {
	var gotQuery string
	_, isErr := callTool(t, func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		_, _ = w.Write([]byte(`{}`))
	}, "list_test_merges", map[string]any{"product_id": "p1", "test_ref": "57"})
	if isErr {
		t.Fatal("tool error")
	}
	if gotQuery != "testRef=57" {
		t.Errorf("query = %q, want testRef=57", gotQuery)
	}
	body, isErr := callTool(t, func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		_, _ = w.Write([]byte(`{}`))
	}, "list_test_merges", map[string]any{"product_id": "p1"})
	if isErr || gotQuery != "" {
		t.Errorf("no test_ref: query = %q, err = %v", gotQuery, isErr)
	}
	if merges, ok := body["merges"].([]any); !ok || len(merges) != 0 {
		t.Errorf("an empty list must be an empty array, got %v", body)
	}
}

// get_test hands the identity fields through the model untouched.
func TestGetTestPassesIdentityFieldsThrough(t *testing.T) {
	rec := &holdRecorder{response: `{"test":{"id":"t1","productId":"p1","kind":"case","title":"logs in","repository":"github.com/acme/shop",
	  "mergedIntoId":"t0","redirectedFrom":"61","keys":[{"key":"v2n:abc","createdAt":"2026-10-23T10:00:00Z"},{"key":"s:src/a.test.ts::logs_in"}]}}`}
	body, isErr := callTool(t, rec.handler(t), "get_test", map[string]any{"id": "t1"})
	if isErr {
		t.Fatalf("tool error: %v", body)
	}
	if body["repository"] != "github.com/acme/shop" || body["mergedIntoId"] != "t0" || body["redirectedFrom"] != "61" {
		t.Errorf("identity fields lost: %v", body)
	}
	keys, _ := body["keys"].([]any)
	if len(keys) != 2 || keys[0].(map[string]any)["key"] != "v2n:abc" || keys[0].(map[string]any)["createdAt"] != "2026-10-23T10:00:00Z" {
		t.Errorf("keys = %v", body["keys"])
	}
}

// report_test_results passes an identity object to the server verbatim and says
// in its docs that rows may carry one.
func TestReportTestResultsPassesIdentityThrough(t *testing.T) {
	for _, phrase := range []string{"identity", "merge_test_cases"} {
		if !strings.Contains(toolDescription(t, "report_test_results"), phrase) {
			t.Errorf("report_test_results description lacks %q", phrase)
		}
	}
	rec := &holdRecorder{response: `{"status":true,"accepted":"1","duplicates":"0","errors":[]}`}
	body, isErr := callTool(t, rec.handler(t), "report_test_results", map[string]any{
		"product_id": "p1", "run_seq": 42,
		"results": []map[string]any{{
			"id": "00000000-0000-4000-8000-000000000001", "title": "logs in", "signature": "src/a.test.ts::logs_in",
			"identity":  map[string]any{"language": "js", "repository": "github.com/acme/shop", "scope": []string{"src/a.test.ts"}, "name": []string{"logs in"}},
			"execution": map[string]any{"status": "passed"},
		}},
	})
	if isErr {
		t.Fatalf("tool error: %v", body)
	}
	req := rec.only(t)
	rows, _ := req.body["results"].([]any)
	if len(rows) != 1 {
		t.Fatalf("body = %v", req.body)
	}
	identity, _ := rows[0].(map[string]any)["identity"].(map[string]any)
	if identity["language"] != "js" || identity["repository"] != "github.com/acme/shop" {
		t.Errorf("identity did not reach the server: %v", rows[0])
	}
}
