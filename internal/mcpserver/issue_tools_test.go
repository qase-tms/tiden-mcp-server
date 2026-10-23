package mcpserver_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync/atomic"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/qase-tms/tiden-mcp-server/internal/api"
	"github.com/qase-tms/tiden-mcp-server/internal/mcpserver"
)

// TestListIssuesRejectsRetiredLevelsArgument pins what the CHANGELOG promises
// (TIDEN-113): list_issues no longer has a levels argument, and a call that
// still passes one is rejected before any request reaches tiden-app, rather
// than quietly ignored. The control case proves the same call without levels
// does reach the API, so the rejection is the schema's, not a transport error.
func TestListIssuesRejectsRetiredLevelsArgument(t *testing.T) {
	var hits atomic.Int32
	httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"issues":[],"pagination":{}}`))
	}))
	defer httpServer.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	server := mcpserver.New(api.New(httpServer.URL, "test-token"), "ws-default")
	go func() { _ = server.Run(ctx, serverTransport) }()

	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "v0"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = session.Close() }()

	ok, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "list_issues", Arguments: map[string]any{"product_id": "p1"}})
	if err != nil || ok.IsError {
		t.Fatalf("control call without levels failed: err=%v result=%+v", err, ok)
	}
	if got := hits.Load(); got != 1 {
		t.Fatalf("control call made %d API requests, want 1", got)
	}

	res, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "list_issues", Arguments: map[string]any{
		"product_id": "p1",
		"levels":     []string{"fatal"},
	}})
	if err == nil && !res.IsError {
		t.Fatalf("list_issues with levels succeeded, want it rejected: %+v", res)
	}
	if got := hits.Load(); got != 1 {
		t.Fatalf("list_issues with levels reached the API (%d requests total), want it rejected first", got)
	}
}

// TestSetIssueStatusSendsReason: set_issue_status passes its optional reason
// to tiden-app, and leaves the key out of the body when no reason is given.
func TestSetIssueStatusSendsReason(t *testing.T) {
	for _, tc := range []struct {
		name string
		args map[string]any
		want map[string]any
	}{
		{"with reason", map[string]any{"id": "i1", "status": "resolved", "reason": "fixed in #42"},
			map[string]any{"status": "resolved", "reason": "fixed in #42"}},
		{"without reason", map[string]any{"id": "i1", "status": "resolved"},
			map[string]any{"status": "resolved"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var got map[string]any
			handler := func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/v1/issues/i1:setStatus" {
					t.Errorf("path = %s", r.URL.Path)
				}
				_ = json.NewDecoder(r.Body).Decode(&got)
				_, _ = w.Write([]byte(`{"issue":{"id":"i1","status":"resolved","timesSeen":"1"}}`))
			}
			_, isErr := callTool(t, handler, "set_issue_status", tc.args)
			if isErr {
				t.Fatalf("set_issue_status returned a tool error")
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("body = %v, want %v", got, tc.want)
			}
		})
	}
}
