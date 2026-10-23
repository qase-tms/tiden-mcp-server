package mcpserver

import (
	"context"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/qase-tms/tiden-mcp-server/internal/api"
)

// registerTestMergeTools adds the test case merge tools. Two test cases that
// are one test (a renamed test that kept its old case, a reporter that changed
// how it builds identity) are folded into one: the survivor keeps its number
// and history, the merged case keeps resolving to it, and every merge can be
// undone.
func registerTestMergeTools(srv *mcp.Server, client *api.Client) {
	registerMergeTestCases(srv, client)
	registerUnmergeTestCases(srv, client)
	registerListTestMerges(srv, client)
}

// -- merge_test_cases ------------------------------------------------------------

type mergeTestCasesArgs struct {
	ProductID string `json:"product_id"       jsonschema:"Product ID (required)."`
	Ref       string `json:"ref"              jsonschema:"The test case to merge away: its case number (seq) as a string such as \"61\", or its UUID (required)."`
	IntoRef   string `json:"into_ref"         jsonschema:"The test case that survives: its case number (seq) as a string such as \"57\", or its UUID (required)."`
	Reason    string `json:"reason,omitempty" jsonschema:"Why the two cases are one test; kept on the merge record (optional, recommended)."`
}

func registerMergeTestCases(srv *mcp.Server, client *api.Client) {
	mcp.AddTool(srv, &mcp.Tool{
		Name: "merge_test_cases",
		Description: "Merge one test case into another when both are the same test (for example a renamed test that was reported as a new case). " +
			"The survivor (into_ref) keeps its number, results and links; the merged case (ref) keeps resolving to it, so [test N] references and old links still work. " +
			"The merge is recorded at once and the server moves the rows in the background: queued=true until it has, and new results already land on the survivor. " +
			"Your merge skips the server's own equivalence check, so verify that both cases are one test first. " +
			"Refused with a tool error for the same case twice, a case of another product, or a pair an earlier undo blocked. " +
			"Returns mergeId, survivorSeq, mergedSeq and queued. Reversible with unmerge_test_cases.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args mergeTestCasesArgs) (*mcp.CallToolResult, any, error) {
		ref, into := strings.TrimSpace(args.Ref), strings.TrimSpace(args.IntoRef)
		if args.ProductID == "" {
			return toolError(errMissingField("product_id"))
		}
		if ref == "" {
			return toolError(errMissingField("ref"))
		}
		if into == "" {
			return toolError(errMissingField("into_ref"))
		}
		resp, err := client.MergeTestCases(ctx, args.ProductID, ref, api.MergeTestCasesRequest{
			IntoRef: into, Reason: strings.TrimSpace(args.Reason),
		})
		if err != nil {
			return toolError(err)
		}
		return toolResult(resp)
	})
}

// -- unmerge_test_cases ----------------------------------------------------------

type unmergeTestCasesArgs struct {
	ProductID string `json:"product_id" jsonschema:"Product ID (required)."`
	MergeID   string `json:"merge_id"   jsonschema:"The merge to undo, as returned by merge_test_cases or list_test_merges (required)."`
}

// unmergeTestCasesResult is unmerge_test_cases's answer: the server's, plus
// blocked, which is always true — an undone pair is never merged automatically
// again.
type unmergeTestCasesResult struct {
	RestoredSeq         int  `json:"restoredSeq"`
	AmbiguousAfterMerge int  `json:"ambiguousAfterMerge"`
	Blocked             bool `json:"blocked"`
}

func registerUnmergeTestCases(srv *mcp.Server, client *api.Client) {
	mcp.AddTool(srv, &mcp.Tool{
		Name: "unmerge_test_cases",
		Description: "Undo a test case merge: the merged case comes back with the results and links that moved with the merge. " +
			"Only the newest merge in the chain that touches either case can be undone; otherwise the tool errors and the message names the newer merge to undo first. " +
			"Results that arrived on the survivor after the merge move back only when they carry evidence of the restored case; the rest stay on the survivor and ambiguousAfterMerge counts them. " +
			"An undone pair is blocked from automatic merging (blocked=true); merge_test_cases still works. " +
			"Returns restoredSeq, ambiguousAfterMerge and blocked.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args unmergeTestCasesArgs) (*mcp.CallToolResult, any, error) {
		mergeID := strings.TrimSpace(args.MergeID)
		if args.ProductID == "" {
			return toolError(errMissingField("product_id"))
		}
		if mergeID == "" {
			return toolError(errMissingField("merge_id"))
		}
		resp, err := client.UndoTestCaseMerge(ctx, args.ProductID, mergeID)
		if err != nil {
			return toolError(err)
		}
		return toolResult(unmergeTestCasesResult{
			RestoredSeq: resp.RestoredSeq, AmbiguousAfterMerge: resp.AmbiguousAfterMerge, Blocked: true,
		})
	})
}

// -- list_test_merges ------------------------------------------------------------

type listTestMergesArgs struct {
	ProductID string `json:"product_id"         jsonschema:"Product ID (required)."`
	TestRef   string `json:"test_ref,omitempty" jsonschema:"Only merges that touch this test case: its case number (seq) as a string, or its UUID (optional)."`
}

func registerListTestMerges(srv *mcp.Server, client *api.Client) {
	mcp.AddTool(srv, &mcp.Tool{
		Name: "list_test_merges",
		Description: "List a product's test case merges, newest first: mergeId, status (queued, merged or undone), the merged and surviving cases, reason, actorName (empty for an automatic merge) and createdAt. " +
			"Pass test_ref to see only the merges that touch one case. Use it to find the mergeId for unmerge_test_cases.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args listTestMergesArgs) (*mcp.CallToolResult, any, error) {
		if args.ProductID == "" {
			return toolError(errMissingField("product_id"))
		}
		resp, err := client.ListTestCaseMerges(ctx, args.ProductID, strings.TrimSpace(args.TestRef))
		if err != nil {
			return toolError(err)
		}
		return toolResult(resp)
	})
}
