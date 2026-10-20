package mcpserver_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/qase-tms/tiden-mcp-server/internal/api"
	"github.com/qase-tms/tiden-mcp-server/internal/mcpserver"
)

// callTool runs one tool through a real MCP session against handler and
// returns the decoded JSON text of the result.
func callTool(t *testing.T, handler http.HandlerFunc, name string, args map[string]any) (map[string]any, bool) {
	t.Helper()
	httpServer := httptest.NewServer(handler)
	defer httpServer.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	st, ct := mcp.NewInMemoryTransports()
	server := mcpserver.New(api.New(httpServer.URL, "token"), "workspace")
	go func() { _ = server.Run(ctx, st) }()
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "v0"}, nil)
	session, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = session.Close() }()
	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatal(err)
	}
	text := result.Content[0].(*mcp.TextContent).Text
	var body map[string]any
	if err := json.Unmarshal([]byte(text), &body); err != nil {
		if result.IsError {
			// Tool errors carry plain text, not JSON.
			return map[string]any{"error": text}, true
		}
		t.Fatalf("result is not JSON: %v: %s", err, text)
	}
	return body, result.IsError
}

const (
	featureA = "11111111-1111-4111-8111-111111111111"
	featureB = "22222222-2222-4222-8222-222222222222"
	compID1  = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	compID2  = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
)

// Two components. Feature A (f-a) has child a1; feature B (f-b) has child b1.
// The second row of component 1 is a branch copy of a1 keyed by its own id,
// reachable through canonicalId.
const (
	rowsDeployed = `[
	  {"requirementId":"` + featureA + `","display":"QA-1","coverage":"verified"},
	  {"requirementId":"a1","display":"QA-2","coverage":"not_run","parentId":"` + featureA + `"},
	  {"requirementId":"` + featureB + `","display":"QA-3","coverage":"no_test"},
	  {"requirementId":"b1","display":"QA-4","coverage":"verified","parentId":"` + featureB + `"}]`
	rowsComp2 = `[{"requirementId":"b2","display":"QA-5","coverage":"no_test","parentId":"` + featureB + `"}]`
)

// deployedSummary is the deployed server's shape: summary view supported,
// counts absent, feature and component filters ignored.
func deployedSummary() string {
	return `{"view":"summary","summary":{"components":[
	  {"componentId":"` + compID1 + `","name":"api","status":"VERDICT_STATUS_PASS","requirements":` + rowsDeployed + `},
	  {"componentId":"` + compID2 + `","name":"web","status":"VERDICT_STATUS_FAIL","requirements":` + rowsComp2 + `}]}}`
}

// newServerSummary is the upcoming server's shape: counts always present, the
// subject filter applied (feature A only).
func newServerSummary() string {
	return `{"view":"summary","summary":{"components":[
	  {"componentId":"` + compID1 + `","name":"api","status":"VERDICT_STATUS_PASS","requirements":[
	    {"requirementId":"` + featureA + `","display":"QA-1","coverage":"verified","linked":3,"passed":3,"failed":0,"stale":0},
	    {"requirementId":"a1","display":"QA-2","coverage":"not_run","parentId":"` + featureA + `","linked":3,"passed":2,"failed":1,"stale":0}]}]}}`
}

func traceabilityHandler(t *testing.T, response string, wantQuery map[string]string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		for k, v := range wantQuery {
			if q.Get(k) != v {
				t.Errorf("query %s = %q, want %q (%v)", k, q.Get(k), v, q)
			}
		}
		_, _ = w.Write([]byte(response))
	}
}

func components(t *testing.T, body map[string]any) []map[string]any {
	t.Helper()
	raw, ok := body["components"].([]any)
	if !ok {
		t.Fatalf("components missing: %v", body)
	}
	out := make([]map[string]any, len(raw))
	for i, c := range raw {
		out[i] = c.(map[string]any)
	}
	return out
}

func TestGetTraceabilityDefaultIsRollup(t *testing.T) {
	body, isErr := callTool(t, traceabilityHandler(t, deployedSummary(), map[string]string{"view": "summary", "subjectId": ""}),
		"get_traceability", withProduct(map[string]any{}))
	if isErr {
		t.Fatalf("tool error: %v", body)
	}
	if body["view"] != "rollup" {
		t.Errorf("view = %v", body["view"])
	}
	comps := components(t, body)
	if len(comps) != 2 {
		t.Fatalf("components = %v", comps)
	}
	api1 := comps[0]
	if api1["name"] != "api" || api1["specCount"] != float64(4) {
		t.Errorf("api rollup = %v", api1)
	}
	cov := api1["coverage"].(map[string]any)
	if cov["verified"] != float64(2) || cov["not_run"] != float64(1) || cov["no_test"] != float64(1) {
		t.Errorf("coverage = %v", cov)
	}
	nv := api1["notVerified"].([]any)
	if len(nv) != 2 || nv[0].(map[string]any)["display"] != "QA-2" || nv[0].(map[string]any)["requirementId"] != "a1" {
		t.Errorf("notVerified = %v", nv)
	}
	if _, ok := api1["requirements"]; ok {
		t.Errorf("rollup must not list every spec: %v", api1)
	}
}

func TestGetTraceabilityRollupCapsNotVerified(t *testing.T) {
	var rows []string
	for i := 0; i < 60; i++ {
		rows = append(rows, fmt.Sprintf(`{"requirementId":"r%d","display":"QA-%d","coverage":"no_test"}`, i, i))
	}
	resp := `{"view":"summary","summary":{"components":[{"componentId":"` + compID1 + `","name":"api","status":"VERDICT_STATUS_PASS","requirements":[` + strings.Join(rows, ",") + `]}]}}`
	body, isErr := callTool(t, traceabilityHandler(t, resp, nil), "get_traceability", withProduct(map[string]any{}))
	if isErr {
		t.Fatalf("tool error: %v", body)
	}
	c := components(t, body)[0]
	if len(c["notVerified"].([]any)) != 50 || c["notVerifiedMore"] != float64(10) {
		t.Errorf("cap not applied: %d listed, more = %v", len(c["notVerified"].([]any)), c["notVerifiedMore"])
	}
}

func TestGetTraceabilityRollupFromFullMatrix(t *testing.T) {
	// A server that ignores view=summary answers with the complete matrix.
	resp := `{"matrix":{"components":[{"componentId":"` + compID1 + `","name":"api","status":"VERDICT_STATUS_PASS","requirements":[
	  {"requirementId":"r1","display":"QA-1","coverage":"no_test","cells":[{"display":"TC-1","testCase":"t","status":"absent"}]}]}]}}`
	body, isErr := callTool(t, traceabilityHandler(t, resp, nil), "get_traceability", withProduct(map[string]any{}))
	if isErr || body["view"] != "rollup" {
		t.Fatalf("body = %v, error = %v", body, isErr)
	}
	if components(t, body)[0]["specCount"] != float64(1) {
		t.Errorf("body = %v", body)
	}
}

func TestGetTraceabilityFeatureSummaryServerIgnoresFilter(t *testing.T) {
	// Deployed server: view=summary works, subjectId is ignored and the whole
	// product comes back with no counts.
	body, isErr := callTool(t, traceabilityHandler(t, deployedSummary(), map[string]string{"view": "summary", "subjectType": "feature", "subjectId": featureA}),
		"get_traceability", withProduct(map[string]any{"feature_id": featureA}))
	if isErr {
		t.Fatalf("tool error: %v", body)
	}
	if body["view"] != "summary" || body["featureFilter"] != "client" || body["note"] == "" {
		t.Errorf("filter not marked: %v", body)
	}
	comps := components(t, body)
	if len(comps) != 1 || comps[0]["name"] != "api" {
		t.Fatalf("empty component not dropped: %v", comps)
	}
	reqs := comps[0]["requirements"].([]any)
	if len(reqs) != 2 || reqs[0].(map[string]any)["display"] != "QA-1" || reqs[1].(map[string]any)["display"] != "QA-2" {
		t.Errorf("subtree = %v", reqs)
	}
	if _, ok := reqs[0].(map[string]any)["linked"]; ok {
		t.Errorf("counts must be absent on the deployed shape: %v", reqs[0])
	}
}

func TestGetTraceabilityFeatureSummaryServerAppliesFilter(t *testing.T) {
	// Upcoming server: filter applied, counts always present.
	body, isErr := callTool(t, traceabilityHandler(t, newServerSummary(), map[string]string{"view": "summary", "subjectType": "feature", "subjectId": featureA}),
		"get_traceability", withProduct(map[string]any{"feature_id": featureA}))
	if isErr {
		t.Fatalf("tool error: %v", body)
	}
	if _, marked := body["featureFilter"]; marked {
		t.Errorf("server applied the filter, must not be marked: %v", body)
	}
	reqs := components(t, body)[0]["requirements"].([]any)
	row := reqs[1].(map[string]any)
	if len(reqs) != 2 || row["linked"] != float64(3) || row["failed"] != float64(1) || row["stale"] != float64(0) {
		t.Errorf("counts not passed through: %v", reqs)
	}
}

func TestGetTraceabilityFeatureFilterMatchesCanonicalID(t *testing.T) {
	// Branch rows carry a branch-local id; parentId names the main id.
	resp := `{"view":"summary","summary":{"components":[{"componentId":"` + compID1 + `","name":"api","status":"VERDICT_STATUS_PASS","requirements":[
	  {"requirementId":"branch-a","canonicalId":"` + featureA + `","display":"QA-1","coverage":"verified"},
	  {"requirementId":"branch-a1","display":"QA-2","coverage":"no_test","parentId":"` + featureA + `"},
	  {"requirementId":"other","display":"QA-9","coverage":"no_test"}]}]}}`
	body, isErr := callTool(t, traceabilityHandler(t, resp, nil), "get_traceability", withProduct(map[string]any{"feature_id": featureA}))
	if isErr {
		t.Fatalf("tool error: %v", body)
	}
	reqs := components(t, body)[0]["requirements"].([]any)
	if len(reqs) != 2 || body["featureFilter"] != "client" {
		t.Errorf("reqs = %v, body = %v", reqs, body)
	}
}

func TestGetTraceabilityComponentScope(t *testing.T) {
	body, isErr := callTool(t, traceabilityHandler(t, deployedSummary(), map[string]string{"view": "summary", "subjectType": "component", "subjectId": compID2}),
		"get_traceability", withProduct(map[string]any{"component_id": compID2}))
	if isErr {
		t.Fatalf("tool error: %v", body)
	}
	comps := components(t, body)
	if len(comps) != 1 || comps[0]["name"] != "web" || body["componentFilter"] != "client" {
		t.Errorf("body = %v", body)
	}
}

func TestGetTraceabilityIncludeTestsReturnsDetailForScope(t *testing.T) {
	const detail = `{"matrix":{"components":[
	  {"componentId":"` + compID1 + `","name":"api","status":"VERDICT_STATUS_PASS","requirements":[
	    {"requirementId":"` + featureA + `","display":"QA-1","coverage":"verified","cells":[{"display":"TC-1","testCase":"t","status":"passed"}]}]}]}}`
	body, isErr := callTool(t, traceabilityHandler(t, detail, map[string]string{"view": "", "subjectType": "feature", "subjectId": featureA}),
		"get_traceability", withProduct(map[string]any{"feature_id": featureA, "include_tests": true}))
	if isErr {
		t.Fatalf("tool error: %v", body)
	}
	req := components(t, body)[0]["requirements"].([]any)[0].(map[string]any)
	if body["view"] != "detail" || req["cells"] == nil {
		t.Errorf("body = %v", body)
	}
}

func TestGetTraceabilityInputValidation(t *testing.T) {
	for _, tc := range []struct {
		name string
		args map[string]any
		want string
	}{
		{"display ref as feature_id", map[string]any{"feature_id": "QA-12"}, "get_spec or list_specs"},
		{"display ref as component_id", map[string]any{"component_id": "web"}, "component UUID"},
		{"both scopes", map[string]any{"feature_id": featureA, "component_id": compID1}, "not both"},
		{"include_tests without scope", map[string]any{"include_tests": true}, "requires feature_id or component_id"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body, isErr := callTool(t, func(http.ResponseWriter, *http.Request) {
				t.Error("request must not reach the server")
			}, "get_traceability", withProduct(tc.args))
			if !isErr || !strings.Contains(fmt.Sprint(body["error"]), tc.want) {
				t.Errorf("error = %v (isErr %v), want %q", body, isErr, tc.want)
			}
		})
	}
}

func TestSessionProgressCompactTestsFlag(t *testing.T) {
	for _, tc := range []struct {
		name string
		args map[string]any
		want bool
	}{
		{"default is compact", map[string]any{}, true},
		{"all_tests lists everything", map[string]any{"all_tests": true}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var got map[string]any
			args := map[string]any{"product_id": "p1", "session_id": "s1", "spec_ids": []string{"r1"}}
			for k, v := range tc.args {
				args[k] = v
			}
			body, isErr := callTool(t, func(w http.ResponseWriter, r *http.Request) {
				data, _ := io.ReadAll(r.Body)
				if err := json.Unmarshal(data, &got); err != nil {
					t.Errorf("request body: %v", err)
				}
				_, _ = w.Write([]byte(`{"requirements":[{"requirementId":"r1","canonicalId":"m1","coverage":"verified","testCount":5,"tests":[{"signature":"pkg::TestA"}]}],"summary":{"total":1,"verified":1},"intentBranchStatus":"open","freshnessApplied":true}`))
			}, "session_progress", args)
			if isErr {
				t.Fatalf("tool error: %v", body)
			}
			compact, _ := got["compactTests"].(bool)
			if compact != tc.want {
				t.Errorf("compactTests = %v, want %v (%v)", got["compactTests"], tc.want, got)
			}
			row := body["requirements"].([]any)[0].(map[string]any)
			if row["testCount"] != float64(5) || row["canonicalId"] != "m1" || body["intentBranchStatus"] != "open" || body["freshnessApplied"] != true {
				t.Errorf("response fields lost: %v", body)
			}
			if row["tests"].([]any)[0].(map[string]any)["signature"] != "pkg::TestA" {
				t.Errorf("test signature lost: %v", row)
			}
		})
	}
}

func TestGetSpecPassesBranch(t *testing.T) {
	for _, branch := range []string{"intent/x", ""} {
		_, isErr := callTool(t, func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/v1/requirements/r1" || r.URL.Query().Get("branch") != branch {
				t.Errorf("request = %s", r.URL)
			}
			_, _ = w.Write([]byte(`{"requirement":{"id":"r1"}}`))
		}, "get_spec", map[string]any{"id": "r1", "branch": branch})
		if isErr {
			t.Fatal("tool error")
		}
	}
}

func TestListSpecsPagingTool(t *testing.T) {
	description := toolDescription(t, "list_specs")
	for _, phrase := range []string{"one page", "page_token", "all=true", "100 pages", "incomplete", "same", "identity"} {
		if !strings.Contains(description, phrase) {
			t.Fatalf("missing pagination guidance %q: %s", phrase, description)
		}
	}
	for _, tc := range []struct {
		name      string
		args      map[string]any
		requests  int
		size      string
		wantCount int
		wantError bool
		// tokenGiven marks a call that continues a traversal; the server then
		// answers with the last page.
		tokenGiven bool
	}{
		{name: "default single page", args: map[string]any{}, requests: 1, size: "25", wantCount: 1},
		{name: "explicit page", args: map[string]any{"page_size": 50, "page_token": "next"}, requests: 1, size: "50", wantCount: 1, tokenGiven: true},
		{name: "explicit all", args: map[string]any{"all": true}, requests: 2, size: "200", wantCount: 2},
		{name: "all with size", args: map[string]any{"all": true, "page_size": 10}, wantError: true},
		{name: "identity with all", args: map[string]any{"all": true, "view": "identity"}, wantError: true},
		{name: "oversize", args: map[string]any{"page_size": 201}, wantError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			requests := 0
			body, isErr := callTool(t, func(w http.ResponseWriter, r *http.Request) {
				requests++
				if r.URL.Query().Get("pagination.pageSize") != tc.size {
					t.Errorf("query = %v", r.URL.Query())
				}
				if r.URL.Query().Get("pagination.pageToken") == "" && requests == 1 && !tc.tokenGiven {
					_, _ = w.Write([]byte(`{"requirements":[{"id":"r1"}],"pagination":{"nextPageToken":"next","totalCount":2}}`))
					return
				}
				_, _ = w.Write([]byte(`{"requirements":[{"id":"r2"}],"pagination":{"totalCount":2}}`))
			}, "list_specs", withProduct(tc.args))
			if isErr != tc.wantError || requests != tc.requests {
				t.Fatalf("requests = %d, error = %v, body = %v", requests, isErr, body)
			}
			if !tc.wantError && len(body["requirements"].([]any)) != tc.wantCount {
				t.Errorf("body = %v", body)
			}
		})
	}
}

func withProduct(args map[string]any) map[string]any {
	args["product_id"] = "p1"
	return args
}
