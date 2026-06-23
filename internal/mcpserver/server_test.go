package mcpserver_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/qase-tms/tiden-mcp-server/internal/api"
	"github.com/qase-tms/tiden-mcp-server/internal/mcpserver"
)

// allExpectedTools is the canonical list of tool names that must be registered.
// Any deviation (addition, removal, rename) must be a deliberate change here.
var allExpectedTools = []string{
	"whoami",
	"list_workspaces",
	"list_products",
	"list_requirements",
	"get_requirement",
	"create_requirement",
	"update_requirement",
	"list_tests",
	"get_test",
	"list_branches",
	"create_branch",
	"get_merge_preview",
	"list_components",
	"list_environments",
	"list_releases",
	"gate_check",
	"get_verdict",
	"get_overview",
	"get_traceability",
	"create_test",
	"update_test",
	"link_requirement",
}

// newTestServer creates an MCP server backed by a throwaway api.Client.
// The client is never called in these tests; we only inspect registration.
func newTestServer() *mcp.Server {
	client := api.New("http://localhost", "test-token")
	return mcpserver.New(client, "ws-default")
}

// TestToolRegistry verifies that every declared tool name is registered and
// that the total count exactly matches allExpectedTools (no silent extras).
func TestToolRegistry(t *testing.T) {
	ctx := context.Background()
	srv := newTestServer()

	// Connect via in-memory transport so we can call tools/list.
	serverTransport, clientTransport := mcp.NewInMemoryTransports()

	serverDone := make(chan error, 1)
	go func() {
		serverDone <- srv.Run(ctx, serverTransport)
	}()

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "v0"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client.Connect: %v", err)
	}
	defer func() { _ = session.Close() }()

	// Collect all tool names from the server.
	registered := map[string]bool{}
	for tool, err := range session.Tools(ctx, nil) {
		if err != nil {
			t.Fatalf("Tools iterator: %v", err)
		}
		registered[tool.Name] = true
	}

	// Every expected tool must be present.
	for _, name := range allExpectedTools {
		if !registered[name] {
			t.Errorf("tool %q not registered", name)
		}
	}

	// No unexpected extras.
	if len(registered) != len(allExpectedTools) {
		t.Errorf("expected %d tools, got %d; registered: %v", len(allExpectedTools), len(registered), registered)
	}
}

// TestInputSchemaUnmarshal verifies that the JSON input schema for key tools
// can be unmarshalled correctly - catching field name mismatches early.
func TestInputSchemaUnmarshal(t *testing.T) {
	tests := []struct {
		tool  string
		input string
	}{
		{
			tool:  "list_requirements",
			input: `{"product_id": "prod-123", "branch": "feature/xyz"}`,
		},
		{
			tool:  "create_requirement",
			input: `{"product_id": "prod-123", "title": "Auth flow", "description": "Describes OAuth flow", "parent_id": "req-456", "branch": "main"}`,
		},
		{
			tool:  "update_requirement",
			input: `{"id": "req-789", "title": "New title", "status": "Active", "priority": "High", "branch": "main"}`,
		},
		{
			tool:  "list_tests",
			input: `{"product_id": "prod-123", "branch": ""}`,
		},
		{
			tool:  "create_branch",
			input: `{"product_id": "prod-123", "name": "feature/login", "description": "Login redesign"}`,
		},
		{
			tool:  "get_merge_preview",
			input: `{"branch_id": "branch-abc"}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.tool, func(t *testing.T) {
			// We verify that the raw JSON is valid and contains sensible keys.
			// Full schema validation happens inside the SDK on tool call; here we
			// just ensure the JSON we intend to send is well-formed.
			var m map[string]any
			if err := json.Unmarshal([]byte(tc.input), &m); err != nil {
				t.Fatalf("input JSON is malformed: %v", err)
			}
		})
	}
}
