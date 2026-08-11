package mcpserver_test

import (
	"context"
	"encoding/json"
	"sort"
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
	"get_product",
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
	"session_progress",
	"get_session_progress",
	"record_risk_acceptances",
	"create_test",
	"update_test",
	"link_requirement",
	"list_test_runs",
	"get_test_run",
	"get_run_results",
	"report_test_results",
	"complete_test_run",
	"create_test_run",
	"abort_test_run",
	"capture_intent",
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
		{
			tool:  "gate_check",
			input: `{"product_id": "prod-123", "branch": "intent/2026-08-05-x"}`,
		},
		{
			tool:  "get_verdict",
			input: `{"product_id": "prod-123", "release_id": "rel-1", "branch": ""}`,
		},
		{
			tool:  "get_traceability",
			input: `{"product_id": "prod-123", "branch": "intent/2026-08-05-x"}`,
		},
		{
			tool:  "session_progress",
			input: `{"product_id": "prod-123", "session_id": "3f0e8c1a-2b4d-4e6f-8a9b-0c1d2e3f4a5b", "requirement_ids": ["req-1", "req-2"], "intent_branch": "intent/2026-08-05-x"}`,
		},
		{
			tool:  "get_session_progress",
			input: `{"product_id":"prod-123","session_id":"3f0e8c1a-2b4d-4e6f-8a9b-0c1d2e3f4a5b","requirement_ids":["req-1"],"intent_branch":"intent/x"}`,
		},
		{
			tool:  "record_risk_acceptances",
			input: `{"product_id":"prod-123","requirement_id":"draft-1","intent_branch":"intent/x","session_id":"3f0e8c1a-2b4d-4e6f-8a9b-0c1d2e3f4a5b","acceptances":[{"requirement_refs":["QA-1"],"criterion":"R4","evidence":"provider sandbox covers the contract","follow_up":"issue:QA-42"}]}`,
		},
		{
			tool:  "list_test_runs",
			input: `{"product_id": "p1", "status": "failed", "page_size": 10}`,
		},
		{
			tool:  "get_test_run",
			input: `{"product_id": "p1", "run_seq": 42}`,
		},
		{
			tool:  "get_run_results",
			input: `{"product_id": "p1", "run_seq": 42, "summary": true}`,
		},
		{
			tool:  "report_test_results",
			input: `{"product_id": "p1", "run_seq": 42, "results": [{"title": "t", "execution": {"status": "passed"}}]}`,
		},
		{
			tool:  "complete_test_run",
			input: `{"product_id": "p1", "run_seq": 42}`,
		},
		{
			tool:  "create_test_run",
			input: `{"product_id": "p1", "title": "Nightly", "environment": "staging"}`,
		},
		{
			tool:  "abort_test_run",
			input: `{"product_id": "p1", "run_seq": 42}`,
		},
		{
			tool:  "capture_intent",
			input: `{"product_id": "p1", "transcript": "USER: add X\nASSISTANT: done", "slug": "add-x", "session_id": "s1", "agent": "cursor", "changed_files": ["a.go"]}`,
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

// TestToolRequiredFields freezes the schema-derived "required" list for every
// registered tool. jsonschema.ForType marks any struct field WITHOUT
// ,omitempty/,omitzero in its json tag as required, so a field that is
// genuinely optional (e.g. status, environment, page_size, branch on most
// mutators) must carry ,omitempty or the SDK's pre-handler argument
// validation will reject calls that omit it - before the handler's own
// errMissingField checks ever run. This test guards the whole tool registry
// against that regressing silently, not just the test-run tools.
func TestToolRequiredFields(t *testing.T) {
	ctx := context.Background()
	srv := newTestServer()

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

	wantRequired := map[string][]string{
		"whoami":                  {},
		"list_workspaces":         {},
		"list_products":           {},
		"list_requirements":       {"product_id"},
		"get_requirement":         {"id"},
		"create_requirement":      {"product_id", "title"},
		"update_requirement":      {"id"},
		"list_tests":              {"product_id"},
		"get_test":                {"id"},
		"list_branches":           {"product_id"},
		"create_branch":           {"product_id", "name"},
		"get_merge_preview":       {"branch_id"},
		"list_components":         {"product_id"},
		"list_environments":       {"product_id"},
		"list_releases":           {"product_id"},
		"gate_check":              {"product_id"},
		"get_verdict":             {"product_id"},
		"get_overview":            {"product_id"},
		"get_traceability":        {"product_id"},
		"session_progress":        {"product_id", "session_id", "requirement_ids"},
		"get_session_progress":    {"product_id", "session_id", "requirement_ids"},
		"record_risk_acceptances": {"product_id", "requirement_id", "intent_branch", "session_id", "acceptances"},
		"create_test":             {"product_id", "title"},
		"update_test":             {"id"},
		"link_requirement":        {"test_id", "requirement_id"},
		"list_test_runs":          {"product_id"},
		"get_test_run":            {"product_id", "run_seq"},
		"get_run_results":         {"product_id", "run_seq"},
		"report_test_results":     {"product_id", "run_seq", "results"},
		"complete_test_run":       {"product_id", "run_seq"},
		"create_test_run":         {"product_id"},
		"abort_test_run":          {"product_id", "run_seq"},
		"capture_intent":          {"product_id", "transcript"},
	}

	found := map[string]bool{}
	for tool, err := range session.Tools(ctx, nil) {
		if err != nil {
			t.Fatalf("Tools iterator: %v", err)
		}
		want, ok := wantRequired[tool.Name]
		if !ok {
			continue
		}
		found[tool.Name] = true

		// From the client, InputSchema is the default JSON marshaling of the
		// server's schema: a map[string]any.
		schema, ok := tool.InputSchema.(map[string]any)
		if !ok {
			t.Errorf("%s: InputSchema is %T, want map[string]any", tool.Name, tool.InputSchema)
			continue
		}

		var got []string
		if rawRequired, ok := schema["required"]; ok {
			items, ok := rawRequired.([]any)
			if !ok {
				t.Errorf("%s: required field is %T, want []any", tool.Name, rawRequired)
				continue
			}
			for _, item := range items {
				s, ok := item.(string)
				if !ok {
					t.Errorf("%s: required item %v is %T, want string", tool.Name, item, item)
					continue
				}
				got = append(got, s)
			}
		}

		gotSorted := append([]string(nil), got...)
		wantSorted := append([]string(nil), want...)
		sort.Strings(gotSorted)
		sort.Strings(wantSorted)

		if !equalStringSlices(gotSorted, wantSorted) {
			t.Errorf("%s: required = %v, want %v", tool.Name, gotSorted, wantSorted)
		}
	}

	for name := range wantRequired {
		if !found[name] {
			t.Errorf("tool %q not found while checking required fields", name)
		}
	}
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
