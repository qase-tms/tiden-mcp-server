package mcpserver_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/qase-tms/tiden-mcp-server/internal/api"
	"github.com/qase-tms/tiden-mcp-server/internal/mcpserver"
)

// TestListProductsTool_DefaultsToResolvedWorkspace proves the workspace_id
// fallback wired in registerListProducts (internal/mcpserver/tools.go): when
// the tool argument is omitted, the request goes to the server's resolved
// default workspace (the value main.go passes to mcpserver.New from the
// TIDEN-68 resolution ladder); an explicit workspace_id still overrides it.
func TestListProductsTool_DefaultsToResolvedWorkspace(t *testing.T) {
	for _, tc := range []struct {
		name        string
		args        map[string]any
		defaultWSID string
		wantPath    string
	}{
		{
			name:        "omitted workspace_id uses the resolved default",
			args:        map[string]any{},
			defaultWSID: "ws-resolved-default",
			wantPath:    "/v1/workspaces/ws-resolved-default/products",
		},
		{
			name:        "explicit workspace_id overrides the default",
			args:        map[string]any{"workspace_id": "ws-explicit"},
			defaultWSID: "ws-resolved-default",
			wantPath:    "/v1/workspaces/ws-explicit/products",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var gotPath string
			httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotPath = r.URL.Path
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"products":[],"pagination":{"nextPageToken":"","totalCount":0}}`))
			}))
			defer httpServer.Close()

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			serverTransport, clientTransport := mcp.NewInMemoryTransports()
			server := mcpserver.New(api.New(httpServer.URL, "test-token"), tc.defaultWSID)
			go func() { _ = server.Run(ctx, serverTransport) }()

			client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "v0"}, nil)
			session, err := client.Connect(ctx, clientTransport, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = session.Close() }()

			result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "list_products", Arguments: tc.args})
			if err != nil {
				t.Fatal(err)
			}
			if result.IsError {
				t.Fatalf("unexpected tool error: %+v", result)
			}
			if gotPath != tc.wantPath {
				t.Errorf("request path = %q, want %q", gotPath, tc.wantPath)
			}
		})
	}
}
