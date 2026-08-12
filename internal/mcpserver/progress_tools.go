package mcpserver

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/qase-tms/tiden-mcp-server/internal/api"
)

// -- session_progress ------------------------------------------------------------

type sessionProgressArgs struct {
	ProductID      string   `json:"product_id"              jsonschema:"Product ID (required)."`
	SessionID      string   `json:"session_id"              jsonschema:"Intent session UUID whose runs count as 'from this session' (required)."`
	RequirementIDs []string `json:"requirement_ids"         jsonschema:"Requirement UUIDs in the session's scope (required, non-empty). Typically the session's retrieved requirements plus its draft."`
	IntentBranch   string   `json:"intent_branch,omitempty" jsonschema:"Intent branch name; includes that branch's proposed test links in coverage. Omit for durable (main) links only."`
}

func registerSessionProgress(srv *mcp.Server, client *api.Client) {
	mcp.AddTool(srv, &mcp.Tool{
		Name: "session_progress",
		Description: "Per-requirement progress for one intent session: coverage on the ladder no_test -> not_run -> failing -> verified, whether the deciding tests came from this session, overall readiness, and next actions. Call after reporting test results to see what moved and what still blocks readiness. " +
			"This does NOT start, refine, or close the session: the intent lifecycle stays CLI-only, because closing grades against the local git diff and the session record in `~/.tiden/sessions/<id>.json`, neither of which this server has — use this tool to read where a session stands, and `tiden intent close` to act on it.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args sessionProgressArgs) (*mcp.CallToolResult, any, error) {
		if args.ProductID == "" {
			return toolError(errMissingField("product_id"))
		}
		if args.SessionID == "" {
			return toolError(errMissingField("session_id"))
		}
		if len(args.RequirementIDs) == 0 {
			return toolError(errMissingField("requirement_ids"))
		}
		resp, err := client.GetSessionProgress(ctx, args.ProductID, api.SessionProgressQuery{
			SessionID:      args.SessionID,
			RequirementIDs: args.RequirementIDs,
			IntentBranch:   args.IntentBranch,
		})
		if err != nil {
			return toolError(err)
		}
		return toolResult(resp)
	})
}
