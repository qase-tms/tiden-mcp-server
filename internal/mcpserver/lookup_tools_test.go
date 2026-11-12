package mcpserver_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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

// viaChildrenIds passes through lookup_context unchanged, and is absent when
// the server sends none.
func TestLookupContextCarriesViaChildrenIDs(t *testing.T) {
	for _, tc := range []struct {
		name string
		via  []string
	}{{"with", []string{"gap-1"}}, {"without", nil}} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				out := model.LookupBatchResult{
					Version: 1, ProductID: "product", BranchID: "main", RetrievalStatus: "complete", ReadAt: "2026-09-07T00:00:00Z",
					Requirements: []model.LookupRequirement{{ID: "gap-1", CanonicalID: "gap-1", Title: "Parent"}},
					Items: []model.LookupItemResult{{
						ID: "P1", Question: "q", Status: "ok", CandidateRequirementIDs: []string{"gap-1"}, CoverageGapIDs: []string{"gap-1"},
						ViaChildrenIDs: tc.via, Matches: []model.LookupMatch{{RequirementID: "gap-1", FeatureID: "gap-1", Roles: []string{"seed"}}}, Diagnostics: []string{},
					}},
				}
				_ = json.NewEncoder(w).Encode(out)
			}))
			defer backend.Close()
			srv := mcpserver.New(api.New(backend.URL, "token"), "")
			st, ct := mcp.NewInMemoryTransports()
			go func() { _ = srv.Run(ctx, st) }()
			session, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "v0"}, nil).Connect(ctx, ct, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = session.Close() }()
			result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "lookup_context", Arguments: map[string]any{"product_id": "product", "items": []map[string]any{{"id": "P1", "question": "q"}}}})
			if err != nil {
				t.Fatal(err)
			}
			if result.IsError {
				t.Fatalf("tool error: %+v", result.Content)
			}
			raw, _ := json.Marshal(result.StructuredContent)
			var decoded model.LookupBatchResult
			if err := json.Unmarshal(raw, &decoded); err != nil {
				t.Fatal(err)
			}
			got := decoded.Items[0].ViaChildrenIDs
			if len(got) != len(tc.via) {
				t.Fatalf("viaChildrenIds = %v, want %v (%s)", got, tc.via, raw)
			}
			if tc.via == nil && strings.Contains(string(raw), "viaChildrenIds") {
				t.Fatalf("absent field appeared: %s", raw)
			}
		})
	}
}

// evidence and anchor line ranges pass through lookup_context from the
// server's raw response; the tool's input schema stays {repository, path}.
func TestLookupContextPassesEvidenceAndAnchorLines(t *testing.T) {
	ctx := context.Background()
	const wire = `{"version":1,"productId":"product","branchId":"main","retrievalStatus":"complete","readAt":"2026-11-12T00:00:00Z",
"requirements":[{"id":"r1","canonicalId":"r1","seqNum":2142,"title":"Lookup","description":"d","anchors":[{"repository":"github.com/org/app","path":"internal/lookup.go","lineStart":178,"lineEnd":217}]}],
"items":[{"id":"P1","question":"q","status":"ok","candidateRequirementIds":["r1"],"coverageGapIds":[],"diagnostics":[],"truncated":false,"omitted":0,"limitReached":false,
"matches":[{"requirementId":"r1","featureId":"r1","roles":["seed"],"evidence":[
{"kind":"anchor","signals":["anchor"],"anchor":{"repository":"github.com/org/app","path":"internal/lookup.go","lineStart":178,"lineEnd":217}},
{"kind":"seed","seedRank":2,"signals":["vector","fts"]},
{"kind":"co_anchored","viaRequirementId":"r2","viaSeqNum":2143,"anchor":{"repository":"github.com/org/app","path":"internal/lookup.go","lineStart":120,"lineEnd":180},"confidence":0.25},
{"kind":"ancestor","viaRequirementId":"r3","viaSeqNum":709,"hops":1}]}]}]}`
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(wire))
	}))
	defer backend.Close()
	srv := mcpserver.New(api.New(backend.URL, "token"), "")
	st, ct := mcp.NewInMemoryTransports()
	go func() { _ = srv.Run(ctx, st) }()
	session, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "v0"}, nil).Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = session.Close() }()

	for tool, err := range session.Tools(ctx, nil) {
		if err != nil {
			t.Fatal(err)
		}
		if tool.Name != "lookup_context" {
			continue
		}
		in, _ := json.Marshal(tool.InputSchema)
		if strings.Contains(string(in), "lineStart") || strings.Contains(string(in), "lineEnd") || strings.Contains(string(in), "evidence") {
			t.Fatalf("input schema must stay {repository, path}: %s", in)
		}
		if !strings.Contains(tool.Description, "evidence") || !strings.Contains(tool.Description, "line ranges") {
			t.Fatalf("description must mention evidence and line ranges: %s", tool.Description)
		}
	}

	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "lookup_context", Arguments: map[string]any{"product_id": "product", "items": []map[string]any{{"id": "P1", "question": "q"}}}})
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError {
		t.Fatalf("tool error: %+v", result.Content)
	}
	raw, _ := json.Marshal(result.StructuredContent)
	var decoded model.LookupBatchResult
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	ev := decoded.Items[0].Matches[0].Evidence
	if len(ev) != 4 || ev[0].Kind != "anchor" || ev[0].Anchor == nil || ev[0].Anchor.LineStart != 178 || ev[0].Anchor.LineEnd != 217 {
		t.Fatalf("anchor evidence lost: %s", raw)
	}
	if ev[1].SeedRank != 2 || len(ev[1].Signals) != 2 {
		t.Fatalf("seed evidence lost: %s", raw)
	}
	if ev[2].ViaSeqNum != 2143 || ev[2].Confidence != 0.25 || ev[2].Anchor.LineStart != 120 {
		t.Fatalf("co_anchored evidence lost: %s", raw)
	}
	if ev[3].ViaSeqNum != 709 || ev[3].Hops != 1 {
		t.Fatalf("ancestor evidence lost: %s", raw)
	}
	a := decoded.Requirements[0].Anchors[0]
	if a.LineStart != 178 || a.LineEnd != 217 {
		t.Fatalf("dictionary anchor lines lost: %s", raw)
	}
	text := result.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(text, `"lineStart":178`) || !strings.Contains(text, `"seedRank":2`) {
		t.Fatalf("text content lacks evidence: %s", text)
	}
}
