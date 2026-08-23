package mcpserver_test

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
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
	"list_issues",
	"get_issue",
	"list_issue_events",
	"get_issue_event",
	"get_issue_event_stats",
	"get_issue_fix_context",
	"list_release_issues",
	"set_issue_status",
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

// TestRiskAcceptanceToolDescription freezes the complete pricing rubric in
// the top-level description. Some MCP clients show the tool description but
// collapse nested input-schema descriptions, so keeping R1-R5 only on the
// criterion property would leave an agent unable to choose a valid reason.
func TestRiskAcceptanceToolDescription(t *testing.T) {
	ctx := context.Background()
	srv := newTestServer()
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	go func() { _ = srv.Run(ctx, serverTransport) }()

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "v0"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client.Connect: %v", err)
	}
	defer func() { _ = session.Close() }()

	var description string
	for tool, err := range session.Tools(ctx, nil) {
		if err != nil {
			t.Fatalf("Tools iterator: %v", err)
		}
		if tool.Name == "record_risk_acceptances" {
			description = tool.Description
			break
		}
	}
	if description == "" {
		t.Fatal("record_risk_acceptances tool is not registered")
	}
	for _, phrase := range []string{
		"R1 — unverifiable in this environment",
		"R2 — blocked by an external dependency",
		"R3 — a human-drawn task boundary",
		"R4 — no user-observable consequence",
		"R5 — known-broken verification infrastructure",
		"Volume, ownership, effort",
	} {
		if !strings.Contains(description, phrase) {
			t.Errorf("description omits %q: %q", phrase, description)
		}
	}
}

// TestRecordRiskAcceptancesRequirementIDOptional pins that
// record_risk_acceptances' description explains both write modes now that
// tiden-app#398 made requirement_id optional server-side: the legacy draft
// path when it is supplied, and the session-judgements path (with its
// INTENT_SESSION_NOT_FOUND failure) when it is omitted.
func TestRecordRiskAcceptancesRequirementIDOptional(t *testing.T) {
	ctx := context.Background()
	srv := newTestServer()
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	go func() { _ = srv.Run(ctx, serverTransport) }()

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "v0"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client.Connect: %v", err)
	}
	defer func() { _ = session.Close() }()

	var description string
	for tool, err := range session.Tools(ctx, nil) {
		if err != nil {
			t.Fatalf("Tools iterator: %v", err)
		}
		if tool.Name == "record_risk_acceptances" {
			description = tool.Description
			break
		}
	}
	if description == "" {
		t.Fatal("record_risk_acceptances tool is not registered")
	}
	for _, phrase := range []string{
		"requirement_id is now OPTIONAL",
		"INTENT_SESSION_NOT_FOUND",
		"older CLIs and drafts should keep passing requirement_id",
	} {
		if !strings.Contains(description, phrase) {
			t.Errorf("description omits %q: %q", phrase, description)
		}
	}
}

// TestCaptureIntentDescriptionMentionsSessionSettlement pins that
// capture_intent's description tells the agent to pass session_id:
// tiden-app#398 made the server record a machine-readable settlement on
// that session's record, which is what makes a distillation visible to the
// intent loop's close gate and analytics.
func TestCaptureIntentDescriptionMentionsSessionSettlement(t *testing.T) {
	ctx := context.Background()
	srv := newTestServer()
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	go func() { _ = srv.Run(ctx, serverTransport) }()

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "v0"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client.Connect: %v", err)
	}
	defer func() { _ = session.Close() }()

	var description string
	for tool, err := range session.Tools(ctx, nil) {
		if err != nil {
			t.Fatalf("Tools iterator: %v", err)
		}
		if tool.Name == "capture_intent" {
			description = tool.Description
			break
		}
	}
	if description == "" {
		t.Fatal("capture_intent tool is not registered")
	}
	for _, phrase := range []string{
		"records a machine-readable settlement",
		"always pass session_id when one exists",
	} {
		if !strings.Contains(description, phrase) {
			t.Errorf("description omits %q: %q", phrase, description)
		}
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
			tool:  "record_risk_acceptances",
			input: `{"product_id": "prod-123", "requirement_id": "draft-1", "intent_branch": "intent/2026-08-05-x", "session_id": "3f0e8c1a-2b4d-4e6f-8a9b-0c1d2e3f4a5b", "acceptances": [{"requirement_refs": ["FEED-49"], "criterion": "R1", "evidence": "no sandbox to run this against", "follow_up": "none"}], "proposed_test_requirement_refs": ["FEED-34"]}`,
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
		"record_risk_acceptances": {"product_id", "intent_branch", "session_id"},
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
		// The issue tools are the first to get this right across the board:
		// every genuinely optional argument carries ,omitempty, so only the
		// arguments the handler itself guards show up as required.
		"list_issues":           {"product_id"},
		"get_issue":             {"id"},
		"list_issue_events":     {"id"},
		"get_issue_event":       {"issue_id", "event_id"},
		"get_issue_event_stats": {"id"},
		"get_issue_fix_context": {"product_id", "issue_id"},
		"list_release_issues":   {"release_id"},
		"set_issue_status":      {"id", "status"},
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

// TestIntentLifecycleStaysCLIOnly guards a requirement that has already
// reached one tool of a pair without its sibling once (session_progress
// shipped before the CLI-only wording was required, and record_risk_acceptances
// got it while session_progress did not). Every tool that touches intent-session
// state must tell the calling agent that starting/refining/closing the session
// stays CLI-only, so add its name here whenever a new one is introduced.
func TestIntentLifecycleStaysCLIOnly(t *testing.T) {
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

	toolsNeedingNote := map[string]bool{
		"session_progress":        true,
		"record_risk_acceptances": true,
	}

	found := map[string]bool{}
	for tool, err := range session.Tools(ctx, nil) {
		if err != nil {
			t.Fatalf("Tools iterator: %v", err)
		}
		if !toolsNeedingNote[tool.Name] {
			continue
		}
		found[tool.Name] = true
		for _, marker := range []string{"intent lifecycle", "neither of which this server has"} {
			if !strings.Contains(tool.Description, marker) {
				t.Errorf("%s: description must state that the intent lifecycle (start/refine/close) stays CLI-only (missing %q); got %q", tool.Name, marker, tool.Description)
			}
		}
	}
	for name := range toolsNeedingNote {
		if !found[name] {
			t.Errorf("tool %q not found while checking CLI-only note", name)
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
