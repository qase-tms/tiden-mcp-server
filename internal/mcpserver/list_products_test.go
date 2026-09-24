package mcpserver_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
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

// TestListProductsTool_NoWorkspaceIDAndNoDefault_ReturnsMissingWorkspaceIDError
// is the F6m companion to the ladder amendment in internal/resolve: an
// override token that resolves to no workspace now reaches this tool with
// defaultWorkspaceID == "" instead of the server refusing to start. The
// pre-existing errMissingField("workspace_id") guard in registerListProducts
// (tools.go) is what makes that safe - it must still fire, as a normal tool
// error, and it must never call the API with an empty workspace id.
func TestListProductsTool_NoWorkspaceIDAndNoDefault_ReturnsMissingWorkspaceIDError(t *testing.T) {
	httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("unexpected request: %s %s (workspace_id is missing; the API must never be called)", r.Method, r.URL.Path)
	}))
	defer httpServer.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	server := mcpserver.New(api.New(httpServer.URL, "test-token"), "")
	go func() { _ = server.Run(ctx, serverTransport) }()

	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "v0"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = session.Close() }()

	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "list_products", Arguments: map[string]any{}})
	if err != nil {
		t.Fatal(err)
	}
	if !result.IsError {
		t.Fatalf("result.IsError = false, want a tool error (no workspace_id argument and no default)")
	}
	content, ok := result.Content[0].(*mcp.TextContent)
	if !ok || !strings.Contains(content.Text, "workspace_id") {
		t.Errorf("content = %+v, want the error to name workspace_id", result.Content)
	}
}
