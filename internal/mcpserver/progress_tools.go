package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/qase-tms/tiden-mcp-server/internal/api"
)

// -- session_progress ------------------------------------------------------------

type sessionProgressArgs struct {
	ProductID      string   `json:"product_id"              jsonschema:"Product ID (required)."`
	SessionID      string   `json:"session_id"              jsonschema:"Intent session UUID whose runs count as 'from this session' (required)."`
	RequirementIDs []string `json:"spec_ids"                jsonschema:"Spec UUIDs in the session's scope (required, non-empty). Typically the session's retrieved specs plus its draft."`
	IntentBranch   string   `json:"intent_branch,omitempty" jsonschema:"Intent branch name; includes that branch's proposed test links in coverage. Omit for durable (main) links only."`
	AllTests       bool     `json:"all_tests,omitempty"     jsonschema:"List every linked test of every spec. Default false: each spec lists at most 20 failing tests, then at most 5 others, plus testCount."`
}

type sessionProgressGuidance struct {
	CoverageScope            string `json:"coverageScope"`
	LocalGitEvidenceProvided bool   `json:"localGitEvidenceProvided"`
	LocalCloseCheck          string `json:"localCloseCheck"`
}

func registerSessionProgress(srv *mcp.Server, client *api.Client) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "session_progress",
		Description: "Read server coverage for the supplied spec_ids with an optional intent_branch overlay. The summary and ready flag describe the resolved, assessed specs. Stale IDs can be omitted, branch copies can resolve to one spec, and additional test-linked specs can appear in the rows. This tool sends no local Git freshness evidence. To check whether the session can close on the current diff, run tiden intent progress with the same session and change ranges. By default each spec lists up to 20 failing tests and 5 other tests plus testCount. Pass all_tests=true for the full list. Older servers may always return all tests. The intent lifecycle stays in the CLI because close reads the local session record and Git diff, neither of which this server has.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args sessionProgressArgs) (*mcp.CallToolResult, any, error) {
		if args.ProductID == "" {
			return toolError(errMissingField("product_id"))
		}
		if args.SessionID == "" {
			return toolError(errMissingField("session_id"))
		}
		if len(args.RequirementIDs) == 0 {
			return toolError(errMissingField("spec_ids"))
		}
		resp, err := client.GetSessionProgress(ctx, args.ProductID, api.SessionProgressQuery{
			SessionID:      args.SessionID,
			RequirementIDs: args.RequirementIDs,
			IntentBranch:   args.IntentBranch,
			CompactTests:   !args.AllTests,
		})
		if err != nil {
			return toolError(err)
		}
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(resp, &fields); err != nil {
			return toolError(fmt.Errorf("decode session progress: %w", err))
		}
		if fields == nil {
			return toolError(fmt.Errorf("session progress returned no response object"))
		}
		sessionArg := "'" + strings.ReplaceAll(args.SessionID, "'", "'\\''") + "'"
		guidance, err := json.Marshal(sessionProgressGuidance{
			CoverageScope: "The summary and ready flag describe the specs assessed by the server.",
			LocalCloseCheck: "For close conditions on the current diff, run `tiden intent progress --session=" + sessionArg +
				" --format text` in the session checkout with its original --changes ranges, if used.",
		})
		if err != nil {
			return toolError(err)
		}
		fields["mcpGuidance"] = guidance
		return toolResult(fields)
	})
}
