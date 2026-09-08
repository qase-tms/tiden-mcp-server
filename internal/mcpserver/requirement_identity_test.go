package mcpserver_test

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/qase-tms/tiden-mcp-server/internal/api"
	"github.com/qase-tms/tiden-mcp-server/internal/mcpserver"
	"github.com/qase-tms/tiden-mcp-server/internal/model"
)

// Exercise the registered tool end to end: identity is one page with an
// explicit continuation, defaults remain full, and malformed/old-server answers
// never masquerade as identities with empty document bodies.
func TestRequirementIdentityTool(t *testing.T) {
	for _, tc := range []struct {
		name, view                string
		size                      int
		old, malformed, wantError bool
		requests                  int
	}{
		{name: "identity", view: "identity", requests: 1},
		{name: "custom_page", view: "identity", size: 20, requests: 1},
		{name: "legacy_default", requests: 1},
		{name: "legacy_detail", view: "detail", requests: 1},
		{name: "unknown_view", view: "unknown", wantError: true},
		{name: "oversize", view: "identity", size: 201, wantError: true},
		{name: "paging_without_identity", size: 20, wantError: true},
		{name: "old_server", view: "identity", old: true, wantError: true, requests: 1},
		{name: "missing_hash", view: "identity", malformed: true, wantError: true, requests: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			requests := 0
			httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests++
				q := r.URL.Query()
				if r.Method != "GET" || q.Get("branch") != "feature/x" {
					t.Errorf("request: %s %s", r.Method, r.URL)
				}
				if tc.view != "identity" || tc.old {
					_, _ = w.Write([]byte(`{"requirements":[{"id":"r","content":"full"}],"pagination":{"totalCount":1}}`))
					return
				}
				wantSize := tc.size
				if wantSize == 0 {
					wantSize = 100
				}
				if q.Get("view") != "identity" || q.Get("pagination.pageSize") != fmt.Sprint(wantSize) || q.Get("pagination.pageToken") != "start" {
					t.Errorf("identity query: %v", q)
				}
				hash := fmt.Sprintf("%x", sha256.Sum256(nil))
				if tc.malformed {
					hash = ""
				}
				_ = json.NewEncoder(w).Encode(api.ListRequirementIdentitiesResponse{View: "identity", Identities: []model.RequirementIdentity{{ID: "r", ProductID: "p", Title: "Title", ContentSHA256: hash, TrimmedContentSHA256: hash, Sources: []model.RequirementSourceLocator{}}}, Pagination: &model.Pagination{TotalCount: 2, NextPageToken: "next"}})
			}))
			defer httpServer.Close()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			st, ct := mcp.NewInMemoryTransports()
			server := mcpserver.New(api.New(httpServer.URL, "token"), "workspace")
			go func() { _ = server.Run(ctx, st) }()
			client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "v0"}, nil)
			session, err := client.Connect(ctx, ct, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = session.Close() }()
			args := map[string]any{"product_id": "p", "branch": "feature/x", "view": tc.view, "page_size": tc.size}
			if tc.view == "identity" {
				args["page_token"] = "start"
			}
			result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "list_requirements", Arguments: args})
			if err != nil {
				t.Fatal(err)
			}
			if result.IsError != tc.wantError || requests != tc.requests {
				t.Fatalf("requests=%d result=%+v", requests, result)
			}
			if tc.wantError {
				return
			}
			var body map[string]any
			if err := json.Unmarshal([]byte(result.Content[0].(*mcp.TextContent).Text), &body); err != nil {
				t.Fatal(err)
			}
			if tc.view == "identity" {
				identity := body["identities"].([]any)[0].(map[string]any)
				if _, ok := identity["content"]; ok {
					t.Fatal("identity became a partial writable requirement")
				}
				if body["pagination"].(map[string]any)["nextPageToken"] != "next" {
					t.Fatal("continuation lost")
				}
			} else if body["requirements"].([]any)[0].(map[string]any)["content"] != "full" {
				t.Fatal("legacy detail changed")
			}
		})
	}
}
