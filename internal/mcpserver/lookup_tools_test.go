package mcpserver_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/qase-tms/tiden-mcp-server/internal/api"
	"github.com/qase-tms/tiden-mcp-server/internal/mcpserver"
	"github.com/qase-tms/tiden-mcp-server/internal/model"
)

func TestLookupContextStructuredPartialAndReadOnlySchema(t *testing.T) {
	ctx := context.Background()
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/v1/products/product/feature-context:batch" {
			t.Errorf("unexpected backend call %s %s", r.Method, r.URL.Path)
		}
		var request struct{ Items []model.LookupItem }
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
		}
		out := model.LookupBatchResult{Version: 1, ProductID: "product", BranchID: "main", RetrievalStatus: "partial", ReadAt: "2026-09-07T00:00:00Z", Items: []model.LookupItemResult{}, Requirements: []model.LookupRequirement{}}
		for i, item := range request.Items {
			row := model.LookupItemResult{ID: item.ID, Question: item.Question, Status: "no_match", Matches: []model.LookupMatch{}, CandidateRequirementIDs: []string{}, CoverageGapIDs: []string{}, Diagnostics: []string{}}
			if i == 1 {
				row.Status = "error"
				row.Error = "timeout"
			}
			out.Items = append(out.Items, row)
		}
		_ = json.NewEncoder(w).Encode(out)
	}))
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
	for tool, err := range session.Tools(ctx, nil) {
		if err != nil {
			t.Fatal(err)
		}
		if tool.Name == "lookup_context" {
			if tool.OutputSchema == nil || tool.Annotations == nil || !tool.Annotations.ReadOnlyHint || tool.Annotations.DestructiveHint == nil || *tool.Annotations.DestructiveHint {
				t.Fatal("missing schema/read-only annotation")
			}
		}
	}
	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "lookup_context", Arguments: map[string]any{"product_id": "product", "items": []map[string]any{{"id": "P1", "question": "retrieval"}, {"id": "P2", "question": "ingest"}}}})
	if err != nil {
		t.Fatal(err)
	}
	if !result.IsError || result.StructuredContent == nil || len(result.Content) != 1 {
		t.Fatalf("partial result lost: %+v", result)
	}
	raw, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	var decoded model.LookupBatchResult
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.RetrievalStatus != "partial" || len(decoded.Items) != 2 || decoded.Items[1].Status != "error" {
		t.Fatalf("wrong structured result %s", raw)
	}
	text := result.Content[0].(*mcp.TextContent).Text
	var fromText model.LookupBatchResult
	if err := json.Unmarshal([]byte(text), &fromText); err != nil {
		t.Fatal(err)
	}
	if fromText.Items[0].ID != decoded.Items[0].ID {
		t.Fatal("text and structured content disagree")
	}
}
