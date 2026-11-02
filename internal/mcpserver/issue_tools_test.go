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

// TestGetIssueFixContextPaging: get_issue_fix_context passes page_token and
// page_size to tiden-app, returns the pagination, clears the payload, and
// still works against a server that sends no pagination.
func TestGetIssueFixContextPaging(t *testing.T) {
	var gotQuery map[string][]string
	handler := func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		_, _ = w.Write([]byte(`{"context":{"latestEvent":{"id":"e1","payload":"secret"},"suspectRequirements":[{"id":"r1"}]},"pagination":{"nextPageToken":"n2","totalCount":45}}`))
	}
	body, isErr := callTool(t, handler, "get_issue_fix_context", map[string]any{
		"product_id": "p1", "issue_id": "i1", "page_token": "n1", "page_size": 20,
	})
	if isErr {
		t.Fatalf("tool error: %v", body)
	}
	if gotQuery["pagination.pageToken"][0] != "n1" || gotQuery["pagination.pageSize"][0] != "20" {
		t.Errorf("query = %v", gotQuery)
	}
	pg, _ := body["pagination"].(map[string]any)
	if pg["nextPageToken"] != "n2" || pg["totalCount"] != float64(45) {
		t.Errorf("pagination = %v", body["pagination"])
	}
	if ev, _ := body["latestEvent"].(map[string]any); ev != nil && ev["payload"] != nil {
		t.Errorf("payload not cleared: %v", ev)
	}

	old := func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"context":{"suspectPaths":["a.go"]}}`))
	}
	body, isErr = callTool(t, old, "get_issue_fix_context", map[string]any{"product_id": "p1", "issue_id": "i1"})
	if isErr {
		t.Fatalf("tool error against old server: %v", body)
	}
	if _, has := body["pagination"]; has {
		t.Errorf("pagination = %v, want absent", body["pagination"])
	}
}
