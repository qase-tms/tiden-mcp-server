package mcpserver

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/qase-tms/tiden-mcp-server/internal/api"
)

// -- record_risk_acceptances -------------------------------------------------

type sessionRiskAcceptanceArg struct {
	RequirementRefs []string `json:"requirement_refs" jsonschema:"Requirement refs this acceptance covers, exactly as typed: '<CODE>-<N>' (e.g. 'FEED-49') or a UUID (required, non-empty). The server resolves each against the intent branch's view."`
	Criterion       string   `json:"criterion"         jsonschema:"One of R1..R5 (required). R1 unverifiable in this environment. R2 blocked by an external dependency. R3 human-drawn task boundary — evidence MUST quote the boundary. R4 no user-observable consequence. R5 known-broken verification infra. Volume, ownership, effort, and a bare 'out of scope' are NOT criteria and will be refused."`
	Evidence        string   `json:"evidence"          jsonschema:"One checkable line of evidence for the criterion (required, non-empty) — e.g. the command that can't run here, the dependency name, or the exact boundary text for R3."`
	FollowUp        string   `json:"follow_up"         jsonschema:"Where the gap goes next (required): 'proposed-test' | 'none' | 'external:<detail>' | 'issue:<ref>'."`
}

type recordRiskAcceptancesArgs struct {
	ProductID                   string                     `json:"product_id"                              jsonschema:"Product ID (required)."`
	RequirementID               string                     `json:"requirement_id"                          jsonschema:"The intent session's draft requirement UUID — the row these artifacts are recorded on (required). Not any other requirement."`
	IntentBranch                string                     `json:"intent_branch"                           jsonschema:"The session's Tiden branch name; must not be 'main' (required)."`
	SessionID                   string                     `json:"session_id"                              jsonschema:"The intent session UUID that opened this branch (required)."`
	Acceptances                 []sessionRiskAcceptanceArg `json:"acceptances,omitempty"                   jsonschema:"Priced risk acceptances (the R1-R5 rubric). Omit if this call only records test deferrals — but then proposed_test_requirement_refs must be non-empty."`
	ProposedTestRequirementRefs []string                   `json:"proposed_test_requirement_refs,omitempty" jsonschema:"Requirement refs whose missing test is deferred to a future session rather than risk-accepted now. Collapse into one deferral record server-side."`
}

func registerRecordRiskAcceptances(srv *mcp.Server, client *api.Client) {
	mcp.AddTool(srv, &mcp.Tool{
		Name: "record_risk_acceptances",
		Description: "Record one intent session's risk acceptances and test deferrals — the priced exceptions that let `tiden intent close` exit despite open coverage. " +
			"This does NOT close the session, start one, or replace `tiden intent close`: the intent lifecycle (start/refine/close) needs local git state and the CLI's session record, neither of which this server has — call this only against a session the CLI already opened, to hand it a disposition it can act on. " +
			"Validation here is structural only (required fields present, at least one acceptance or one deferral); the server owns the real rules (known criterion, non-empty evidence, known follow-up kind, refs that resolve on the branch) and its refusal message says exactly what is wrong — read it rather than guessing. " +
			"A re-call with the same session_id and requirement set REPLACES that call's rows rather than stacking a contradicting one, so correcting a criterion is safe to retry. " +
			"Ordering note for anyone also writing this draft's sources in the same flow: this call rewrites the draft's whole source array, so write it FIRST and re-fetch the requirement before building any other sources write to the same draft — otherwise that write silently erases what this call just recorded.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args recordRiskAcceptancesArgs) (*mcp.CallToolResult, any, error) {
		if args.ProductID == "" {
			return toolError(errMissingField("product_id"))
		}
		if args.RequirementID == "" {
			return toolError(errMissingField("requirement_id"))
		}
		if args.IntentBranch == "" {
			return toolError(errMissingField("intent_branch"))
		}
		if args.SessionID == "" {
			return toolError(errMissingField("session_id"))
		}
		if len(args.Acceptances) == 0 && len(args.ProposedTestRequirementRefs) == 0 {
			return toolError(errMissingField("acceptances or proposed_test_requirement_refs (at least one)"))
		}

		acceptances := make([]api.SessionRiskAcceptance, len(args.Acceptances))
		for i, a := range args.Acceptances {
			acceptances[i] = api.SessionRiskAcceptance{
				RequirementRefs: a.RequirementRefs,
				Criterion:       a.Criterion,
				Evidence:        a.Evidence,
				FollowUp:        a.FollowUp,
			}
		}

		resp, err := client.RecordSessionRiskAcceptances(ctx, args.ProductID, api.RecordSessionRiskAcceptancesRequest{
			RequirementID:               args.RequirementID,
			IntentBranch:                args.IntentBranch,
			SessionID:                   args.SessionID,
			Acceptances:                 acceptances,
			ProposedTestRequirementRefs: args.ProposedTestRequirementRefs,
		})
		if err != nil {
			return toolError(err)
		}
		return toolResult(resp)
	})
}
