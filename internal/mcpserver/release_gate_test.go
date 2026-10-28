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

// callReleaseGateTool runs one tool against a server that answers every
// request with status and body, and returns the tool result.
func callReleaseGateTool(t *testing.T, tool string, args map[string]any, status int, body string) *mcp.CallToolResult {
	t.Helper()
	httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
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

	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: tool, Arguments: args})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func resultText(t *testing.T, result *mcp.CallToolResult) string {
	t.Helper()
	if len(result.Content) != 1 {
		t.Fatalf("content = %+v, want one text block", result.Content)
	}
	text, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("content = %T, want text", result.Content[0])
	}
	return text.Text
}

const pendingBody = `{"code":9,"message":"QG_CAPTURE_PENDING: the Quality Gate of this release is pending: build abc123def456 has no settled test results yet","details":[]}`

// TestReleaseGateTools_PendingReleaseIsAStateNotAnError (AICP-89): a release
// whose build has no settled test results is refused with QG_CAPTURE_PENDING.
// The three release gate reads answer it as status pending, naming the build
// and the command that captures it now, instead of a tool error.
func TestReleaseGateTools_PendingReleaseIsAStateNotAnError(t *testing.T) {
	for _, tool := range []string{"gate_check", "get_verdict", "get_traceability"} {
		t.Run(tool, func(t *testing.T) {
			result := callReleaseGateTool(t, tool, map[string]any{"product_id": "p1", "release_id": "rel-1"}, http.StatusBadRequest, pendingBody)
			if result.IsError {
				t.Fatalf("pending release answered as a tool error: %s", resultText(t, result))
			}
			var got struct {
				Status, ReleaseID, Message, NextAction string
			}
			if err := json.Unmarshal([]byte(resultText(t, result)), &got); err != nil {
				t.Fatalf("decode answer: %v", err)
			}
			if got.Status != "pending" || got.ReleaseID != "rel-1" {
				t.Errorf("answer = %+v, want status pending for rel-1", got)
			}
			if !strings.Contains(got.Message, "build abc123def456") || strings.Contains(got.Message, "QG_CAPTURE_PENDING") {
				t.Errorf("message = %q, want the build named without the wire prefix", got.Message)
			}
			if !strings.Contains(got.NextAction, "tiden release capture rel-1") {
				t.Errorf("next action = %q, want the capture command for the release", got.NextAction)
			}
		})
	}
}

// TestReleaseGateTools_OtherRefusalsStayErrors: only the pending refusal is
// turned into an answer; a release that was never captured, or the same
// prefix on a non-release scope, is still a tool error.
func TestReleaseGateTools_OtherRefusalsStayErrors(t *testing.T) {
	for name, tc := range map[string]struct {
		args map[string]any
		body string
	}{
		"not captured": {map[string]any{"product_id": "p1", "release_id": "rel-1"}, `{"code":9,"message":"QG_NOT_CAPTURED: quality gate was not captured for this release"}`},
		"branch scope": {map[string]any{"product_id": "p1", "branch": "intent/x"}, pendingBody},
	} {
		t.Run(name, func(t *testing.T) {
			result := callReleaseGateTool(t, "get_verdict", tc.args, http.StatusBadRequest, tc.body)
			if !result.IsError {
				t.Fatalf("refusal answered as a result: %s", resultText(t, result))
			}
		})
	}
}

// TestGetVerdictTool_CarriesReleaseCapture: a captured release's verdict
// reaches the agent with where its capture came from.
func TestGetVerdictTool_CarriesReleaseCapture(t *testing.T) {
	const body = `{"verdict":{"id":"v1","productId":"p1","scope":"VERDICT_SCOPE_RELEASE","releaseId":"rel-1","status":"VERDICT_STATUS_PASS",
		"releaseCapture":{"seq":1,"source":"build_sha","trigger":"executions","buildSha":"1111111111111111111111111111111111111111","releaseBuildSha":"1111111111111111111111111111111111111111"}}}`
	result := callReleaseGateTool(t, "get_verdict", map[string]any{"product_id": "p1", "release_id": "rel-1"}, http.StatusOK, body)
	if result.IsError {
		t.Fatalf("tool error: %s", resultText(t, result))
	}
	text := resultText(t, result)
	for _, want := range []string{`"releaseCapture"`, `"source":"build_sha"`, `"trigger":"executions"`, `"buildSha":"1111111111111111111111111111111111111111"`} {
		if !strings.Contains(text, want) {
			t.Errorf("answer does not carry %s:\n%s", want, text)
		}
	}
}

// TestListReleasesTool_ReturnsEveryPageWithBuildSHA: list_releases gives the
// agent every release, not the gateway's default first page of 20.
func TestListReleasesTool_ReturnsEveryPageWithBuildSHA(t *testing.T) {
	httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("pagination.pageToken") == "" {
			_, _ = io.WriteString(w, `{"releases":[{"id":"r2","version":"2.0.0"}],"pagination":{"nextPageToken":"t2","totalCount":2}}`)
			return
		}
		_, _ = io.WriteString(w, `{"releases":[{"id":"r1","version":"1.0.0","buildSha":"1111111111111111111111111111111111111111"}],"pagination":{"totalCount":2}}`)
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

	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "list_releases", Arguments: map[string]any{"product_id": "p1"}})
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError {
		t.Fatalf("tool error: %s", resultText(t, result))
	}
	var got api.ListReleasesResponse
	if err := json.Unmarshal([]byte(resultText(t, result)), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got.Releases) != 2 || got.Releases[1].BuildSHA != "1111111111111111111111111111111111111111" {
		t.Errorf("releases = %+v, want both pages and the build SHA", got.Releases)
	}
}
