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

func TestCaptureIntentReportsTheBranchAndSkips(t *testing.T) {
	ctx := context.Background()
	rec := &holdRecorder{}
	backend := httptest.NewServer(rec.handler(t))
	defer backend.Close()

	srv := mcpserver.New(api.New(backend.URL, "token"), "")
	st, ct := mcp.NewInMemoryTransports()
	go func() { _ = srv.Run(ctx, st) }()
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "v0"}, nil)
	session, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = session.Close() }()

	call := func(args map[string]any) (string, bool) {
		t.Helper()
		result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "capture_intent", Arguments: args})
		if err != nil {
			t.Fatal(err)
		}
		return result.Content[0].(*mcp.TextContent).Text, result.IsError
	}

	t.Run("created", func(t *testing.T) {
		rec.status, rec.response = http.StatusOK, `{"intentBranch":"intent/2026-10-29-example","created":2,"updated":1}`
		text, isErr := call(map[string]any{"product_id": "p1", "transcript": "USER: add export"})
		if isErr {
			t.Fatalf("tool error: %s", text)
		}
		if !strings.Contains(text, "Created 2 + updated 1 spec(s) on branch intent/2026-10-29-example") {
			t.Errorf("text = %q", text)
		}
	})

	t.Run("skipped", func(t *testing.T) {
		rec.status, rec.response = http.StatusOK, `{"skipped":true,"skipReason":"no product decisions in transcript"}`
		text, isErr := call(map[string]any{"product_id": "p1", "transcript": "USER: hello"})
		if isErr {
			t.Fatalf("tool error: %s", text)
		}
		if text != "Skipped: no product decisions in transcript" {
			t.Errorf("text = %q", text)
		}
	})

	t.Run("missing required field sends nothing", func(t *testing.T) {
		before := rec.count()
		for _, args := range []map[string]any{
			{"transcript": "USER: add export"},
			{"product_id": "p1"},
		} {
			text, isErr := call(args)
			if !isErr {
				t.Errorf("args %v: expected a tool error, got %q", args, text)
			}
		}
		if got := rec.count(); got != before {
			t.Errorf("server saw %d extra requests, want 0", got-before)
		}
	})
}
