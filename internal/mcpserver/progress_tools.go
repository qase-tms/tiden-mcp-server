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
	handler := func(ctx context.Context, _ *mcp.CallToolRequest, args sessionProgressArgs) (*mcp.CallToolResult, any, error) {
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
	}
	description := "Per-requirement progress for one intent session: coverage on the ladder no_test -> not_run -> failing -> verified, whether the deciding tests came from this session, overall readiness, and next actions. Call after reporting test results. Starting, refining, and closing a session remain CLI-only because they require local git and session state."
	mcp.AddTool(srv, &mcp.Tool{Name: "session_progress", Description: description}, handler)
	mcp.AddTool(srv, &mcp.Tool{Name: "get_session_progress", Description: description}, handler)
}

// -- record_risk_acceptances --------------------------------------------------

type riskAcceptanceArgs struct {
	RequirementIDs []string `json:"requirement_ids" jsonschema:"Requirement UUIDs covered by this decision (required, non-empty)."`
	Criterion      string   `json:"criterion" jsonschema:"Close criterion R1, R2, R3, R4, or R5 (required)."`
	Evidence       string   `json:"evidence" jsonschema:"One checkable evidence line (required)."`
	FollowUp       string   `json:"follow_up" jsonschema:"proposed-test, external:<detail>, issue:<ref>, or none (required)."`
}

type recordRiskAcceptancesArgs struct {
	ProductID                  string               `json:"product_id" jsonschema:"Product ID (required)."`
	RequirementID              string               `json:"requirement_id" jsonschema:"Session draft requirement UUID that owns the audit records (required)."`
	IntentBranch               string               `json:"intent_branch" jsonschema:"Non-main intent branch containing the session draft (required)."`
	SessionID                  string               `json:"session_id" jsonschema:"Intent session UUID (required)."`
	Acceptances                []riskAcceptanceArgs `json:"acceptances" jsonschema:"Structured R1-R5 risk acceptances. Pass an empty array when recording proposed-test deferrals only."`
	ProposedTestRequirementIDs []string             `json:"proposed_test_requirement_ids,omitempty" jsonschema:"Requirement UUIDs whose connected debt is deferred to a proposed test."`
}

func registerRiskAcceptances(srv *mcp.Server, client *api.Client) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "record_risk_acceptances",
		Description: "Record structured R1-R5 close decisions and proposed-test deferrals on an intent session draft. This writes the audit ledger only; use the Tiden CLI to start, refine, or close sessions because those steps require local git and session state.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args recordRiskAcceptancesArgs) (*mcp.CallToolResult, any, error) {
		for field, value := range map[string]string{
			"product_id": args.ProductID, "requirement_id": args.RequirementID,
			"intent_branch": args.IntentBranch, "session_id": args.SessionID,
		} {
			if value == "" {
				return toolError(errMissingField(field))
			}
		}
		acceptances := make([]api.SessionRiskAcceptance, len(args.Acceptances))
		for i, acceptance := range args.Acceptances {
			acceptances[i] = api.SessionRiskAcceptance{
				RequirementIDs: acceptance.RequirementIDs, Criterion: acceptance.Criterion,
				Evidence: acceptance.Evidence, FollowUp: acceptance.FollowUp,
			}
		}
		response, err := client.RecordSessionRiskAcceptances(ctx, args.ProductID, api.RecordSessionRiskAcceptancesRequest{
			RequirementID: args.RequirementID, IntentBranch: args.IntentBranch, SessionID: args.SessionID,
			Acceptances: acceptances, ProposedTestRequirementIDs: args.ProposedTestRequirementIDs,
		})
		if err != nil {
			return toolError(err)
		}
		return toolResult(response)
	})
}
