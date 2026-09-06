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

// Exercise the registered tool, including JSON argument decoding and MCP output.
func TestListTestsPagingTool(t *testing.T) {
	for _, tc := range []struct {
		name              string
		args              map[string]any
		requests, tests   int
		size, token, next string
		wantError         bool
	}{
		{name: "default single page", args: map[string]any{}, requests: 1, tests: 1, size: "100", next: "next"},
		{name: "explicit next page", args: map[string]any{"page_size": 50, "page_token": "next"}, requests: 1, tests: 1, size: "50", token: "next"},
		{name: "explicit all", args: map[string]any{"all": true}, requests: 2, tests: 2, size: "200"},
		{name: "invalid size", args: map[string]any{"page_size": -1}, wantError: true},
		{name: "oversize", args: map[string]any{"page_size": 201}, wantError: true},
		{name: "all with size", args: map[string]any{"all": true, "page_size": 100}, wantError: true},
		{name: "all with token", args: map[string]any{"all": true, "page_token": "next"}, wantError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			requests := 0
			httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests++
				q := r.URL.Query()
				if q.Get("branch") != "feature/x" || q.Get("pagination.pageSize") != tc.size {
					t.Errorf("query = %v", q)
				}
				if requests == 1 && q.Get("pagination.pageToken") != tc.token {
					t.Errorf("initial token = %q", q.Get("pagination.pageToken"))
				}
				w.Header().Set("Content-Type", "application/json")
				if q.Get("pagination.pageToken") == "" {
					_, _ = w.Write([]byte(`{"tests":[{"id":"t1"}],"pagination":{"nextPageToken":"next","totalCount":2}}`))
				} else {
					_, _ = w.Write([]byte(`{"tests":[{"id":"t2"}],"pagination":{"nextPageToken":"","totalCount":2}}`))
				}
			}))
			defer httpServer.Close()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			serverTransport, clientTransport := mcp.NewInMemoryTransports()
			server := mcpserver.New(api.New(httpServer.URL, "test-token"), "workspace")
			go func() { _ = server.Run(ctx, serverTransport) }()
			client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "v0"}, nil)
			session, err := client.Connect(ctx, clientTransport, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = session.Close() }()
			tc.args["product_id"] = "p1"
			tc.args["branch"] = "feature/x"
			result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "list_tests", Arguments: tc.args})
			if err != nil {
				t.Fatal(err)
			}
			if requests != tc.requests || result.IsError != tc.wantError {
				t.Fatalf("requests = %d, result = %+v", requests, result)
			}
			if tc.wantError {
				return
			}
			if len(result.Content) != 1 {
				t.Fatalf("content = %#v", result.Content)
			}
			content, ok := result.Content[0].(*mcp.TextContent)
			if !ok {
				t.Fatalf("content type = %T", result.Content[0])
			}
			var response api.ListTestsResponse
			if err := json.Unmarshal([]byte(content.Text), &response); err != nil {
				t.Fatal(err)
			}
			if len(response.Tests) != tc.tests || response.Pagination.TotalCount != 2 || response.Pagination.NextPageToken != tc.next {
				t.Fatalf("response = %+v", response)
			}
		})
	}
}
