package mcpserver

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/qase-tms/tiden-mcp-server/internal/api"
)

// scopeFor picks the verdict scope: a release id -> release scope, empty -> current
// main (not tied to a release).
func scopeFor(releaseID string) string {
	if releaseID != "" {
		return "release"
	}
	return "main"
}

// -- gate_check ----------------------------------------------------------------

type gateCheckArgs struct {
	ProductID string `json:"product_id"           jsonschema:"Product ID (required)."`
	ReleaseID string `json:"release_id,omitempty" jsonschema:"Release UUID to gate. Omit to gate current main."`
}

func registerGateCheck(srv *mcp.Server, client *api.Client) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "gate_check",
		Description: "Compute the Quality Gate verdict (go/no-go) for a release, or current main if no release is given. Returns the verdict status (pass / blocked / risk_accepted), the per-component breakdown, and fix hints. Use this as the 'verify gates' step before shipping.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args gateCheckArgs) (*mcp.CallToolResult, any, error) {
		if args.ProductID == "" {
			return toolError(errMissingField("product_id"))
		}
		v, err := client.ComputeVerdict(ctx, args.ProductID, scopeFor(args.ReleaseID), args.ReleaseID)
		if err != nil {
			return toolError(err)
		}
		return toolResult(v)
	})
}

// -- get_verdict ---------------------------------------------------------------

type getVerdictArgs struct {
	ProductID string `json:"product_id"           jsonschema:"Product ID (required)."`
	ReleaseID string `json:"release_id,omitempty" jsonschema:"Release UUID. Omit for current main."`
}

func registerGetVerdict(srv *mcp.Server, client *api.Client) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_verdict",
		Description: "Read the latest Quality Gate verdict for a release (or current main) without recomputing.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args getVerdictArgs) (*mcp.CallToolResult, any, error) {
		if args.ProductID == "" {
			return toolError(errMissingField("product_id"))
		}
		v, err := client.GetVerdict(ctx, args.ProductID, scopeFor(args.ReleaseID), args.ReleaseID)
		if err != nil {
			return toolError(err)
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
		v, err := client.GetVerdict(ctx, args.ProductID, "main", "")
		if err != nil {
			return toolError(err)
		}
		return toolResult(v)
	})
}

// -- get_traceability ----------------------------------------------------------

type getTraceabilityArgs struct {
	ProductID string `json:"product_id"           jsonschema:"Product ID (required)."`
	ReleaseID string `json:"release_id,omitempty" jsonschema:"Release UUID. Omit for current main."`
}

func registerGetTraceability(srv *mcp.Server, client *api.Client) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_traceability",
		Description: "Requirement->test traceability matrix for a release (or current main): per requirement, its test-coverage state (verified / not_run / no_test) and the linked tests with their latest execution status. Use it to find coverage gaps.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args getTraceabilityArgs) (*mcp.CallToolResult, any, error) {
		if args.ProductID == "" {
			return toolError(errMissingField("product_id"))
		}
		m, err := client.GetTraceability(ctx, args.ProductID, scopeFor(args.ReleaseID), args.ReleaseID)
		if err != nil {
			return toolError(err)
		}
		return toolResult(m)
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

// -- link_requirement ----------------------------------------------------------

type linkRequirementArgs struct {
	TestID        string `json:"test_id"          jsonschema:"Test (case) UUID (required)."`
	RequirementID string `json:"requirement_id"   jsonschema:"Requirement UUID to link (required)."`
	Branch        string `json:"branch,omitempty" jsonschema:"Branch name (optional - links are managed on main in v1)."`
}

func registerLinkRequirement(srv *mcp.Server, client *api.Client) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "link_requirement",
		Description: "Link a test case to a requirement, so the requirement counts as covered by that test in the traceability matrix and the gate.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args linkRequirementArgs) (*mcp.CallToolResult, any, error) {
		if args.TestID == "" {
			return toolError(errMissingField("test_id"))
		}
		if args.RequirementID == "" {
			return toolError(errMissingField("requirement_id"))
		}
		if err := client.LinkRequirement(ctx, args.TestID, args.RequirementID, args.Branch); err != nil {
			return toolError(err)
		}
		return toolResult(map[string]string{"status": "linked", "testId": args.TestID, "requirementId": args.RequirementID})
	})
}
