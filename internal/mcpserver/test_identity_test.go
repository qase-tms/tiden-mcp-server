package mcpserver_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/qase-tms/tiden-mcp-server/internal/api"
	"github.com/qase-tms/tiden-mcp-server/internal/mcpserver"
	"github.com/qase-tms/tiden-mcp-server/internal/model"
)

func testIdentityToolCall(t *testing.T, handler http.HandlerFunc, args map[string]any) *mcp.CallToolResult {
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
	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "list_tests", Arguments: args})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func testIdentityRow() model.TestIdentity {
	seq, parent, source := 7, "suite", "source"
	return model.TestIdentity{ID: "case", ProductID: "p", Kind: "case", Title: "Case", SeqNum: &seq,
		ParentID: &parent, SourceID: &source, Tags: []string{"req:P-2"}, Signature: "pkg::Case", BranchStatus: "modified"}
}

// Registered-tool wiring must preserve bounded identity/default behavior,
// exact matching fields and explicit continuation, never invent empty bodies.
func TestListTestsIdentityTool(t *testing.T) {
	for _, phrase := range []string{"view=identity", "read-only", "page_token", "no descriptions", "get_test"} {
		if !strings.Contains(toolDescription(t, "list_tests"), phrase) {
			t.Fatalf("missing identity guidance %q", phrase)
		}
	}
	for _, tc := range []struct {
		name, view      string
		size            int
		all, empty, bad bool
	}{
		{name: "identity", view: "identity"}, {name: "small", view: "identity", size: 20},
		{name: "empty", view: "identity", empty: true}, {name: "default"}, {name: "detail", view: "detail"},
		{name: "bad view", view: "unknown", bad: true}, {name: "large", view: "identity", size: 201, bad: true},
		{name: "negative", view: "identity", size: -1, bad: true}, {name: "all identity", view: "identity", all: true, bad: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			args := map[string]any{"product_id": "p", "branch": "feature", "view": tc.view, "page_size": tc.size, "all": tc.all}
			if !tc.all {
				args["page_token"] = "start"
			}
			result := testIdentityToolCall(t, func(w http.ResponseWriter, r *http.Request) {
				calls++
				q := r.URL.Query()
				size := tc.size
				if size == 0 {
					size = 100
				}
				if q.Get("branch") != "feature" || q.Get("pagination.pageSize") != fmt.Sprint(size) || q.Get("pagination.pageToken") != "start" {
					t.Errorf("wrong request %s", r.URL)
				}
				if tc.view != "identity" {
					_, _ = w.Write([]byte(`{"tests":[{"id":"case","kind":"case","description":"full body"}],"pagination":{"totalCount":1}}`))
					return
				}
				if q.Get("view") != "identity" {
					t.Error("identity flag not sent")
				}
				rows, total, token := []model.TestIdentity{testIdentityRow()}, 2, "next"
				if tc.empty {
					rows, total, token = []model.TestIdentity{}, 0, ""
				}
				_ = json.NewEncoder(w).Encode(api.ListTestIdentitiesResponse{View: "identity", Identities: rows, Pagination: &model.Pagination{TotalCount: total, NextPageToken: token}})
			}, args)
			if result.IsError != tc.bad || (tc.bad && calls != 0) || (!tc.bad && calls != 1) {
				t.Fatalf("calls %d, result %+v", calls, result)
			}
			if tc.bad {
				return
			}
			var body map[string]any
			if err := json.Unmarshal([]byte(result.Content[0].(*mcp.TextContent).Text), &body); err != nil {
				t.Fatal(err)
			}
			if tc.view != "identity" {
				if body["tests"].([]any)[0].(map[string]any)["description"] != "full body" {
					t.Fatal("default detail lost")
				}
				return
			}
			if body["view"] != "identity" {
				t.Fatal("missing identity marker")
			}
			if tc.empty {
				if len(body["identities"].([]any)) != 0 {
					t.Fatal("empty identity lost")
				}
				return
			}
			row := body["identities"].([]any)[0].(map[string]any)
			if row["signature"] != "pkg::Case" || row["parentId"] != "suite" || row["sourceId"] != "source" || row["tags"].([]any)[0] != "req:P-2" || row["seqNum"] != float64(7) {
				t.Fatalf("identity fields lost: %v", row)
			}
			if _, exists := row["description"]; exists {
				t.Fatal("identity turned into partial writable test")
			}
			if body["pagination"].(map[string]any)["nextPageToken"] != "next" {
				t.Fatal("continuation lost")
			}
		})
	}
}

// An old, malformed, foreign or failed response must not masquerade as a
// successful empty identity inventory in an agent's context.
func TestListTestsIdentityValidation(t *testing.T) {
	valid, _ := json.Marshal(testIdentityRow())
	var fields map[string]any
	if err := json.Unmarshal(valid, &fields); err != nil {
		t.Fatal(err)
	}
	cases := map[string]string{
		"old server":         `{"tests":[],"pagination":{}}`,
		"missing rows":       `{"view":"identity","pagination":{}}`,
		"missing pagination": `{"view":"identity","identities":[]}`,
		"oversize":           fmt.Sprintf(`{"view":"identity","identities":[%s,%s],"pagination":{"totalCount":2}}`, valid, valid),
		"duplicate":          fmt.Sprintf(`{"view":"identity","identities":[%s,%s],"pagination":{"totalCount":2}}`, valid, valid),
	}
	for _, key := range []string{"id", "productId", "kind", "title", "tags", "signature", "seqNum", "framework", "filePath", "layer", "isAutomated", "branchStatus"} {
		copy := map[string]any{}
		for k, v := range fields {
			copy[k] = v
		}
		delete(copy, key)
		body, _ := json.Marshal(map[string]any{"view": "identity", "identities": []any{copy}, "pagination": model.Pagination{TotalCount: 1}})
		cases["missing "+key] = string(body)
	}
	for name, change := range map[string]map[string]any{"foreign": {"productId": "other"}, "bad kind": {"kind": "unknown"}, "suite sequence": {"kind": "suite"}, "bad tags": {"tags": 42}, "null tags": {"tags": nil}} {
		copy := map[string]any{}
		for k, v := range fields {
			copy[k] = v
		}
		for k, v := range change {
			copy[k] = v
		}
		body, _ := json.Marshal(map[string]any{"view": "identity", "identities": []any{copy}, "pagination": model.Pagination{TotalCount: 1}})
		cases[name] = string(body)
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			calls := 0
			pageSize := 1
			if name == "duplicate" {
				pageSize = 2
			}
			result := testIdentityToolCall(t, func(w http.ResponseWriter, _ *http.Request) { calls++; _, _ = w.Write([]byte(body)) }, map[string]any{"product_id": "p", "view": "identity", "page_size": pageSize})
			if !result.IsError || calls != 1 {
				t.Fatalf("malformed identity accepted: %+v, calls %d", result, calls)
			}
		})
	}
	for _, status := range []int{401, 403} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			calls := 0
			result := testIdentityToolCall(t, func(w http.ResponseWriter, _ *http.Request) { calls++; w.WriteHeader(status) }, map[string]any{"product_id": "p", "view": "identity"})
			if !result.IsError || calls != 1 {
				t.Fatalf("auth failure expanded to fallback: %+v", result)
			}
		})
	}
}
