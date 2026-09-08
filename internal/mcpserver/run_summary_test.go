package mcpserver_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/qase-tms/tiden-mcp-server/internal/api"
	"github.com/qase-tms/tiden-mcp-server/internal/mcpserver"
)

func TestRunSummaryProjectionTool(t *testing.T) {
	for _, tc := range []struct {
		name, view, body string
		extra            map[string]any
		wantError        bool
		calls            int
	}{
		{name: "overview", view: "overview", body: `{"view":"overview","suites":[],"caseCount":"0"}`, calls: 1},
		{name: "cases", view: "cases", extra: map[string]any{"page_size": 1, "page_token": "1"}, body: `{"view":"cases","caseOverviews":[{"identityKey":"c","comboCount":"2"}],"pagination":{"totalCount":3,"nextPageToken":"2"}}`, calls: 1},
		{name: "combos", view: "combos", extra: map[string]any{"identity_key": "case"}, body: `{"view":"combos","combos":[],"pagination":{"totalCount":0}}`, calls: 1},
		{name: "old_server", view: "overview", body: `{"suites":[],"cases":[]}`, calls: 1, wantError: true},
		{name: "missing_identity", view: "combos", wantError: true},
		{name: "flat_filter", view: "cases", extra: map[string]any{"status": "failed"}, wantError: true},
		{name: "overview_paging", view: "overview", extra: map[string]any{"page_size": 100}, wantError: true},
		{name: "unknown", view: "unknown", wantError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Method != "GET" || r.URL.Path != "/v1/products/p/runs/42/summary" || r.URL.Query().Get("view") != tc.view {
					t.Errorf("request: %s %s", r.Method, r.URL)
				}
				if tc.name == "cases" && (r.URL.Query().Get("pagination.pageSize") != "1" || r.URL.Query().Get("pagination.pageToken") != "1") {
					t.Error("paging args lost")
				}
				if tc.name == "combos" && r.URL.Query().Get("identityKey") != "case" {
					t.Error("identity lost")
				}
				_, _ = w.Write([]byte(tc.body))
			}))
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
			args := map[string]any{"product_id": "p", "run_seq": 42, "summary_view": tc.view}
			for key, value := range tc.extra {
				args[key] = value
			}
			result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "get_run_results", Arguments: args})
			if err != nil {
				t.Fatal(err)
			}
			if result.IsError != tc.wantError || calls != tc.calls {
				t.Fatalf("calls=%d result=%+v", calls, result)
			}
			if !tc.wantError {
				var body map[string]any
				if err := json.Unmarshal([]byte(result.Content[0].(*mcp.TextContent).Text), &body); err != nil {
					t.Fatal(err)
				}
				if body["view"] != tc.view {
					t.Fatal("projection marker lost")
				}
				if tc.name == "cases" && body["pagination"].(map[string]any)["nextPageToken"] != "2" {
					t.Fatal("continuation lost")
				}
			}
		})
	}
}
