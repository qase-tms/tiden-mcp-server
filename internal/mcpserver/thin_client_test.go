package mcpserver_test

import (
	"context"
	"encoding/json"
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
	var body map[string]any
	_ = json.Unmarshal([]byte(result.Content[0].(*mcp.TextContent).Text), &body)
	return body, result.IsError
}

func TestGetTraceabilityThinClient(t *testing.T) {
	const summaryJSON = `{"summary":{"components":[{"componentId":"c1","name":"api","status":"VERDICT_STATUS_PASS","requirements":[{"requirementId":"r1","display":"QA-1","coverage":"verified","linked":3,"passed":2,"failed":1,"stale":0}]}]},"view":"summary"}`
	const detailJSON = `{"matrix":{"components":[{"componentId":"c1","name":"api","status":"VERDICT_STATUS_PASS","requirements":[{"requirementId":"r1","display":"QA-1","coverage":"verified","cells":[{"display":"TC-1","testCase":"t","status":"passed"}]}]}]}}`
	for _, tc := range []struct {
		name         string
		args         map[string]any
		response     string
		wantView     string
		wantSubject  string
		wantCells    bool
		wantCountsOK bool
	}{
		{name: "H1 default is summary", args: map[string]any{}, response: summaryJSON, wantView: "summary", wantCountsOK: true},
		{name: "H2 feature with tests", args: map[string]any{"feature_id": "f1", "include_tests": true}, response: detailJSON, wantSubject: "f1", wantCells: true},
		{name: "feature summary", args: map[string]any{"feature_id": "f1"}, response: summaryJSON, wantView: "summary", wantSubject: "f1", wantCountsOK: true},
		{name: "N1 old server ignores view", args: map[string]any{}, response: detailJSON, wantView: "summary", wantCells: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body, isErr := callTool(t, func(w http.ResponseWriter, r *http.Request) {
				q := r.URL.Query()
				if q.Get("view") != tc.wantView || q.Get("subjectId") != tc.wantSubject {
					t.Errorf("query = %v", q)
				}
				if tc.wantSubject != "" && q.Get("subjectType") != "feature" {
					t.Errorf("subjectType = %q", q.Get("subjectType"))
				}
				_, _ = w.Write([]byte(tc.response))
			}, "get_traceability", withProduct(tc.args))
			if isErr {
				t.Fatalf("tool error: %v", body)
			}
			req := body["components"].([]any)[0].(map[string]any)["requirements"].([]any)[0].(map[string]any)
			if _, ok := req["cells"]; ok != tc.wantCells {
				t.Errorf("cells present = %v, want %v: %v", ok, tc.wantCells, req)
			}
			if tc.wantCountsOK && (req["linked"] != float64(3) || req["failed"] != float64(1) || req["stale"] != float64(0)) {
				t.Errorf("counts not passed through: %v", req)
			}
			if !tc.wantCountsOK {
				if _, ok := req["linked"]; ok {
					t.Errorf("unexpected counts: %v", req)
				}
			}
		})
	}
}

func TestSessionProgressSendsCompactTests(t *testing.T) {
	var got map[string]any
	body, isErr := callTool(t, func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(data, &got)
		_, _ = w.Write([]byte(`{"requirements":[{"requirementId":"r1","coverage":"verified","testCount":5}],"summary":{"total":1,"verified":1}}`))
	}, "session_progress", map[string]any{"product_id": "p1", "session_id": "s1", "spec_ids": []string{"r1"}})
	if isErr || got["compactTests"] != true {
		t.Fatalf("body = %v, error = %v", got, isErr)
	}
	if body["requirements"].([]any)[0].(map[string]any)["testCount"] != float64(5) {
		t.Errorf("testCount lost: %v", body)
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
	for _, phrase := range []string{"one page", "page_token", "all=true", "100 pages", "incomplete"} {
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
	}{
		{name: "default single page", args: map[string]any{}, requests: 1, size: "100", wantCount: 1},
		{name: "explicit page", args: map[string]any{"page_size": 50, "page_token": "next"}, requests: 1, size: "50", wantCount: 1},
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
				if r.URL.Query().Get("pagination.pageToken") == "" && requests == 1 && tc.name != "explicit page" {
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
