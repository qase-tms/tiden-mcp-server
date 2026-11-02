package mcpserver

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/qase-tms/tiden-mcp-server/internal/api"
	"github.com/qase-tms/tiden-mcp-server/internal/model"
)

// scopeFor picks the verdict scope: a release id -> release scope, else a
// branch name -> branch scope, else current main (not tied to a release).
func scopeFor(releaseID, branch string) string {
	if releaseID != "" {
		return "release"
	}
	if branch != "" {
		return "branch"
	}
	return "main"
}

// releaseGatePending is the answer for a release whose gate is waiting on its
// build's test results. The server refuses the read with QG_CAPTURE_PENDING;
// that is a state to report with its next move, not a tool failure.
type releaseGatePending struct {
	Status     string `json:"status"` // always "pending"
	ReleaseID  string `json:"releaseId"`
	Message    string `json:"message"`
	NextAction string `json:"nextAction"`
}

// releaseGateError answers a pending release gate as a normal result; every
// other error stays a tool error.
func releaseGateError(err error, releaseID string) (*mcp.CallToolResult, any, error) {
	if releaseID == "" || !api.IsCapturePending(err) {
		return toolError(err)
	}
	return toolResult(releaseGatePending{
		Status:    "pending",
		ReleaseID: releaseID,
		Message:   api.CapturePendingMessage(err),
		NextAction: "Wait for the test runs of the release's build to finish: the gate is captured automatically once they settle. " +
			"To capture it now from the results that exist, run `tiden release capture " + releaseID + "`.",
	})
}

// -- gate_check ----------------------------------------------------------------

type gateCheckArgs struct {
	ProductID string `json:"product_id"           jsonschema:"Product ID (required)."`
	ReleaseID string `json:"release_id,omitempty" jsonschema:"Release UUID to gate. Omit to gate current main."`
	Branch    string `json:"branch,omitempty"     jsonschema:"Branch name to gate (e.g. an intent branch). Ignored when release_id is set; omit both for current main."`
}

func registerGateCheck(srv *mcp.Server, client *api.Client) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "gate_check",
		Description: "Compute the Quality Gate verdict for a release, a branch, or current main if neither is given. Returns the verdict status (pass / blocked / not_verified / risk_accepted), the next action to take, the touched specs with their own next action, the accepted risks, the per-component breakdown, and fix hints. With branch, the verdict answers for what that branch touched: not_verified means work is unfinished (a linked test never ran, or a spec has no test) rather than broken, and the next action names the run to make or the cases to write. Use this as the 'verify' step, and act on the next action rather than on the status alone. For a release, the verdict is its current capture: releaseCapture says whether it was computed from the release's build SHA or copied from main, and recapturePending says a newer capture is still to come. While the release's build has no settled test results, the answer is status pending with the next action instead of a verdict.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args gateCheckArgs) (*mcp.CallToolResult, any, error) {
		if args.ProductID == "" {
			return toolError(errMissingField("product_id"))
		}
		v, err := client.ComputeVerdict(ctx, args.ProductID, scopeFor(args.ReleaseID, args.Branch), args.ReleaseID, args.Branch)
		if err != nil {
			return releaseGateError(err, args.ReleaseID)
		}
		return toolResult(v)
	})
}

// -- get_verdict ---------------------------------------------------------------

type getVerdictArgs struct {
	ProductID string `json:"product_id"           jsonschema:"Product ID (required)."`
	ReleaseID string `json:"release_id,omitempty" jsonschema:"Release UUID. Omit for current main."`
	Branch    string `json:"branch,omitempty"     jsonschema:"Branch name (e.g. an intent branch). Ignored when release_id is set; omit both for current main."`
}

func registerGetVerdict(srv *mcp.Server, client *api.Client) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_verdict",
		Description: "Read the latest Quality Gate verdict for a release, a branch, or current main without recomputing. Carries the same next action, touched specs and accepted risks as gate_check. For a release, the verdict is its current capture: releaseCapture says whether it was computed from the release's build SHA or copied from main, and recapturePending says a newer capture is still to come. While the release's build has no settled test results, the answer is status pending with the next action instead of a verdict.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args getVerdictArgs) (*mcp.CallToolResult, any, error) {
		if args.ProductID == "" {
			return toolError(errMissingField("product_id"))
		}
		v, err := client.GetVerdict(ctx, args.ProductID, scopeFor(args.ReleaseID, args.Branch), args.ReleaseID, args.Branch)
		if err != nil {
			return releaseGateError(err, args.ReleaseID)
		}
		return toolResult(v)
	})
}

// -- get_overview --------------------------------------------------------------

type getOverviewArgs struct {
	ProductID string `json:"product_id" jsonschema:"Product ID (required)."`
}

func registerGetOverview(srv *mcp.Server, client *api.Client) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_overview",
		Description: "Current product gate state on main: the overall verdict plus each component's status and residual risk. The fastest 'is the product healthy right now?' read.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args getOverviewArgs) (*mcp.CallToolResult, any, error) {
		if args.ProductID == "" {
			return toolError(errMissingField("product_id"))
		}
		v, err := client.GetVerdict(ctx, args.ProductID, "main", "", "")
		if err != nil {
			return toolError(err)
		}
		return toolResult(v)
	})
}

// -- get_traceability ----------------------------------------------------------

type getTraceabilityArgs struct {
	ProductID    string `json:"product_id"              jsonschema:"Product ID (required)."`
	ReleaseID    string `json:"release_id,omitempty"    jsonschema:"Release UUID. Omit for current main."`
	Branch       string `json:"branch,omitempty"        jsonschema:"Branch name (e.g. an intent branch). Ignored when release_id is set; omit both for current main."`
	FeatureID    string `json:"feature_id,omitempty"    jsonschema:"Spec UUID (the id field of get_spec or list_specs, not a display reference such as QA-12). Limits the answer to that spec and its descendants. Not combinable with component_id."`
	ComponentID  string `json:"component_id,omitempty"  jsonschema:"Component UUID (componentId from the default rollup). Limits the answer to that component. Not combinable with feature_id."`
	IncludeTests bool   `json:"include_tests,omitempty" jsonschema:"Return the detail view with the linked tests of every spec in scope. Requires feature_id or component_id."`
}

func registerGetTraceability(srv *mcp.Server, client *api.Client) {
	mcp.AddTool(srv, &mcp.Tool{
		Name: "get_traceability",
		Description: "Spec->test traceability for a release, a branch, or current main. Output size depends on the scope. " +
			"With neither feature_id nor component_id it returns a per-component rollup (view=rollup): per component its id, name and status, the number of specs per coverage state (verified / not_run / no_test), and the display reference (e.g. QA-12), id and coverage of each spec that is not verified (up to 50 per component, then notVerifiedMore with the remaining count). Use it to find coverage gaps, then scope down. " +
			"With feature_id (a spec UUID; the spec and its descendants) or component_id (a component UUID) it returns one summary row per spec in that scope (view=summary): display, title, coverage and, when the server provides them, linked/passed/failed/stale test counts. " +
			"include_tests=true returns the detail view (view=detail) for that scope with each linked test and its latest execution status; it requires feature_id or component_id because a whole product does not fit the output limit. " +
			"A parent spec with no test links of its own reads its coverage from its child specs: its row then carries viaChildren, childSpecs and childSpecsCovered (and coverageExemption child_specs when its child specs are graded in the same answer). " +
			"If the server does not apply feature_id or component_id, the tool filters the rows itself and sets featureFilter or componentFilter to \"client\"; component status is then component-wide, not computed over the filtered rows only. " +
			"For a release whose build has no settled test results yet, the answer is status pending with the next action instead of rows.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args getTraceabilityArgs) (*mcp.CallToolResult, any, error) {
		if args.ProductID == "" {
			return toolError(errMissingField("product_id"))
		}
		if err := validateSubjectID("feature_id", args.FeatureID); err != nil {
			return toolError(err)
		}
		if err := validateSubjectID("component_id", args.ComponentID); err != nil {
			return toolError(err)
		}
		if args.FeatureID != "" && args.ComponentID != "" {
			return toolError(fmt.Errorf("pass feature_id or component_id, not both"))
		}
		scoped := args.FeatureID != "" || args.ComponentID != ""
		if args.IncludeTests && !scoped {
			return toolError(fmt.Errorf("include_tests requires feature_id or component_id; call without it first to get the rollup"))
		}
		opts := api.TraceabilityOptions{}
		switch {
		case args.FeatureID != "":
			opts.SubjectType, opts.SubjectID = "feature", args.FeatureID
		case args.ComponentID != "":
			opts.SubjectType, opts.SubjectID = "component", args.ComponentID
		}
		if !args.IncludeTests {
			opts.View = "summary"
		}
		r, err := client.GetTraceabilityView(ctx, args.ProductID, scopeFor(args.ReleaseID, args.Branch), args.ReleaseID, args.Branch, opts)
		if err != nil {
			return releaseGateError(err, args.ReleaseID)
		}
		out := traceabilityOutput{}
		dropped := false
		if args.IncludeTests {
			m := r.Matrix
			if args.FeatureID != "" {
				m, dropped = filterMatrixToFeature(m, args.FeatureID)
			} else {
				m, dropped = filterMatrixToComponent(m, args.ComponentID)
			}
			out.View, out.Components = "detail", m.Components
		} else {
			// A server that ignores view=summary answers with the full matrix;
			// strip its cells so the output stays small.
			var s model.TraceabilitySummary
			if r.Summary != nil && r.View == "summary" {
				s = *r.Summary
			} else {
				s = summaryFromMatrix(r.Matrix)
			}
			switch {
			case args.FeatureID != "":
				s, dropped = filterSummaryToFeature(s, args.FeatureID)
			case args.ComponentID != "":
				s, dropped = filterSummaryToComponent(s, args.ComponentID)
			}
			if scoped {
				out.View, out.Components = "summary", s.Components
			} else {
				out.View, out.Components = "rollup", rollup(s)
			}
		}
		if dropped {
			if args.FeatureID != "" {
				out.FeatureFilter = "client"
			} else {
				out.ComponentFilter = "client"
			}
			out.Note = clientFilterNote
		}
		return toolResult(out)
	})
}

// -- create_test ---------------------------------------------------------------

type createTestArgs struct {
	ProductID   string `json:"product_id"            jsonschema:"Product ID (required)."`
	Title       string `json:"title"                 jsonschema:"Test title (required)."`
	Kind        string `json:"kind,omitempty"        jsonschema:"'case' (default) or 'suite'."`
	Description string `json:"description,omitempty" jsonschema:"Markdown description (optional)."`
	ParentID    string `json:"parent_id,omitempty"   jsonschema:"Parent suite UUID (optional)."`
	ComponentID string `json:"component_id,omitempty" jsonschema:"Component UUID to assign (optional)."`
	Branch      string `json:"branch,omitempty"      jsonschema:"Branch name (optional - defaults to main)."`
}

func registerCreateTest(srv *mcp.Server, client *api.Client) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "create_test",
		Description: "Create a test suite or case in a product. Defaults to a case; pass kind 'suite' for a suite. Optionally nest under a parent suite, assign a component, or target a non-main branch.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args createTestArgs) (*mcp.CallToolResult, any, error) {
		if args.ProductID == "" {
			return toolError(errMissingField("product_id"))
		}
		if args.Title == "" {
			return toolError(errMissingField("title"))
		}
		kind := args.Kind
		if kind == "" {
			kind = "case"
		}
		body := api.CreateTestRequest{
			Kind:        kind,
			Title:       args.Title,
			Description: args.Description,
			Branch:      args.Branch,
		}
		if args.ParentID != "" {
			body.ParentID = ptr(args.ParentID)
		}
		if args.ComponentID != "" {
			body.ComponentID = ptr(args.ComponentID)
		}
		t, err := client.CreateTest(ctx, args.ProductID, body)
		if err != nil {
			return toolError(err)
		}
		return toolResult(t)
	})
}

// -- update_test ---------------------------------------------------------------

type updateTestArgs struct {
	ID          string `json:"id"                    jsonschema:"Test UUID (required)."`
	Title       string `json:"title,omitempty"       jsonschema:"New title (optional)."`
	Description string `json:"description,omitempty" jsonschema:"New markdown description (optional)."`
	Status      string `json:"status,omitempty"      jsonschema:"New status: Draft, Active, or Deprecated (optional)."`
	Priority    string `json:"priority,omitempty"    jsonschema:"New priority: Not set, Low, Medium, High, or Critical (optional)."`
	ComponentID string `json:"component_id,omitempty" jsonschema:"Reassign component UUID (optional)."`
	Branch      string `json:"branch,omitempty"      jsonschema:"Branch name (optional - defaults to main)."`
}

func registerUpdateTest(srv *mcp.Server, client *api.Client) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "update_test",
		Description: "Update a test case's title, description, status, priority, or component. Only supplied fields change.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args updateTestArgs) (*mcp.CallToolResult, any, error) {
		if args.ID == "" {
			return toolError(errMissingField("id"))
		}
		body := api.UpdateTestRequest{Branch: args.Branch}
		if args.Title != "" {
			body.Title = ptr(args.Title)
		}
		if args.Description != "" {
			body.Description = ptr(args.Description)
		}
		if args.Status != "" {
			body.Status = ptr(args.Status)
		}
		if args.Priority != "" {
			body.Priority = ptr(args.Priority)
		}
		if args.ComponentID != "" {
			body.ComponentID = ptr(args.ComponentID)
		}
		t, err := client.UpdateTest(ctx, args.ID, body)
		if err != nil {
			return toolError(err)
		}
		return toolResult(t)
	})
}

// -- link_spec -----------------------------------------------------------------

type linkRequirementArgs struct {
	TestID        string `json:"test_id"                 jsonschema:"Test (case) UUID (required)."`
	RequirementID string `json:"spec_id"                 jsonschema:"Spec UUID to link (required)."`
	Branch        string `json:"branch,omitempty"        jsonschema:"Branch name (optional - defaults to main). On a branch the call stages a link proposal that a merge turns into a link. An unknown branch is an error unless create_branch is true."`
	CreateBranch  bool   `json:"create_branch,omitempty" jsonschema:"Create the branch when no branch of that name exists (optional - default false)."`
}

func registerLinkRequirement(srv *mcp.Server, client *api.Client) {
	mcp.AddTool(srv, &mcp.Tool{
		Name: "link_spec",
		Description: "Link a test case to a spec, so the spec counts as covered by that test in the traceability matrix and the gate. " +
			"Returns status, branch, testId and requirementId. On main status is linked. On a branch the link is staged as a proposal and status is the proposal's status (proposed, accepted or rejected; a rejected proposal stays rejected). " +
			"A test that is not in the branch's view, or an unknown branch, is an error.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args linkRequirementArgs) (*mcp.CallToolResult, any, error) {
		if args.TestID == "" {
			return toolError(errMissingField("test_id"))
		}
		if args.RequirementID == "" {
			return toolError(errMissingField("spec_id"))
		}
		resp, err := client.LinkRequirement(ctx, args.TestID, args.RequirementID, args.Branch, args.CreateBranch)
		if err != nil {
			return toolError(err)
		}
		out := map[string]string{"status": "linked", "testId": args.TestID, "requirementId": args.RequirementID}
		if resp.Branch != "" {
			out["branch"] = resp.Branch
		}
		if resp.Proposal != nil && resp.Proposal.Status != "" {
			out["status"] = resp.Proposal.Status
		}
		return toolResult(out)
	})
}
