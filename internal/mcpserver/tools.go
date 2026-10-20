package mcpserver

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/qase-tms/tiden-mcp-server/internal/api"
	"github.com/qase-tms/tiden-mcp-server/internal/model"
)

// registerTools adds all Tiden MCP tools to the server. Each tool is a
// thin wrapper over an existing api.Client method - no domain logic here.
func registerTools(srv *mcp.Server, client *api.Client, defaultWorkspaceID string) {
	registerLookupContext(srv, client)
	registerWhoami(srv, client)
	registerListWorkspaces(srv, client)
	registerListProducts(srv, client, defaultWorkspaceID)
	registerGetProduct(srv, client)
	registerListRequirements(srv, client)
	registerGetRequirement(srv, client)
	registerCreateRequirement(srv, client)
	registerUpdateRequirement(srv, client)
	registerListTests(srv, client)
	registerGetTest(srv, client)
	registerListBranches(srv, client)
	registerCreateBranch(srv, client)
	registerGetMergePreview(srv, client)
	registerHoldIntent(srv, client)
	registerListComponents(srv, client)
	registerListEnvironments(srv, client)
	registerListReleases(srv, client)
	registerGateCheck(srv, client)
	registerGetVerdict(srv, client)
	registerGetOverview(srv, client)
	registerGetTraceability(srv, client)
	registerSessionProgress(srv, client)
	registerRecordRiskAcceptances(srv, client)
	registerCreateTest(srv, client)
	registerUpdateTest(srv, client)
	registerLinkRequirement(srv, client)
	registerTestRunTools(srv, client)
	registerCaptureIntent(srv, client)
	registerListIssues(srv, client)
	registerGetIssue(srv, client)
	registerListIssueEvents(srv, client)
	registerGetIssueEvent(srv, client)
	registerGetIssueEventStats(srv, client)
	registerGetIssueFixContext(srv, client)
	registerListReleaseIssues(srv, client)
	registerSetIssueStatus(srv, client)
}

// -- whoami ------------------------------------------------------------------

type whoamiArgs struct{}

func registerWhoami(srv *mcp.Server, client *api.Client) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "whoami",
		Description: "Return the current authenticated user's profile (id, email, name).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ whoamiArgs) (*mcp.CallToolResult, any, error) {
		user, err := client.Me(ctx)
		if err != nil {
			return toolError(err)
		}
		return toolResult(user)
	})
}

// -- list_workspaces ----------------------------------------------------------

type listWorkspacesArgs struct{}

func registerListWorkspaces(srv *mcp.Server, client *api.Client) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_workspaces",
		Description: "List all workspaces the authenticated user belongs to.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ listWorkspacesArgs) (*mcp.CallToolResult, any, error) {
		resp, err := client.ListWorkspaces(ctx)
		if err != nil {
			return toolError(err)
		}
		return toolResult(resp)
	})
}

// -- list_products ------------------------------------------------------------

type listProductsArgs struct {
	WorkspaceID string `json:"workspace_id,omitempty" jsonschema:"Workspace ID. Omit to use the default workspace from config."`
}

func registerListProducts(srv *mcp.Server, client *api.Client, defaultWorkspaceID string) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_products",
		Description: "List products in a workspace. Omit workspace_id to use the default from config.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args listProductsArgs) (*mcp.CallToolResult, any, error) {
		wsID := args.WorkspaceID
		if wsID == "" {
			wsID = defaultWorkspaceID
		}
		if wsID == "" {
			return toolError(errMissingField("workspace_id"))
		}
		resp, err := client.ListProducts(ctx, wsID)
		if err != nil {
			return toolError(err)
		}
		return toolResult(resp)
	})
}

// -- get_product --------------------------------------------------------------

type getProductArgs struct {
	ID string `json:"id" jsonschema:"Product UUID (required)."`
}

func registerGetProduct(srv *mcp.Server, client *api.Client) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_product",
		Description: "Fetch a single product by id (name, code, description). Use when you have a product id and need its details without listing the whole workspace.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args getProductArgs) (*mcp.CallToolResult, any, error) {
		if args.ID == "" {
			return toolError(errMissingField("id"))
		}
		product, err := client.GetProduct(ctx, args.ID)
		if err != nil {
			return toolError(err)
		}
		return toolResult(product)
	})
}

// -- list_specs ---------------------------------------------------------------

type listRequirementsArgs struct {
	ProductID string `json:"product_id"          jsonschema:"Product ID (required)."`
	Branch    string `json:"branch,omitempty"    jsonschema:"Branch name. Omit for the main branch."`
	View      string `json:"view,omitempty" jsonschema:"detail (default) returns full documents; identity returns read-only IDs, titles, hashes and source locators without content (always one page, incompatible with all). Identity objects must never be reused as updates."`
	PageSize  int    `json:"page_size,omitempty" jsonschema:"Specs per page (1-200). Default 25 for the detail view, 100 for identity. Keep the same value on every call that carries a page_token: tokens are page offsets, so changing it skips or repeats specs."`
	PageToken string `json:"page_token,omitempty" jsonschema:"Continuation token from pagination.nextPageToken. Valid only with the page_size of the call that returned it."`
	All       bool   `json:"all,omitempty" jsonschema:"Explicitly fetch up to 100 pages of 200 specs (detail view only). Cannot combine with paging arguments."`
}

func registerListRequirements(srv *mcp.Server, client *api.Client) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_specs",
		Description: "List one page of specs for a product/branch (25 full documents by default for the detail view, 100 rows for identity; the detail page is small because full bodies are large). Continue with pagination.nextPageToken as page_token, and keep page_size the same on every call of one traversal: tokens are page offsets, so a different page_size skips or repeats specs. For existence checks, duplicate detection or a spec index, use view=identity (IDs, titles, hashes, source locators) or lookup_context instead of reading full documents. Use all=true only for a full detail traversal (up to 100 pages; a remaining nextPageToken means the result is incomplete); identity is always one page and cannot combine with all. In the API a spec is the `requirement` resource, so results use requirement field names (requirements, requirementId).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args listRequirementsArgs) (*mcp.CallToolResult, any, error) {
		if args.ProductID == "" {
			return toolError(errMissingField("product_id"))
		}
		if args.All && (args.PageSize != 0 || args.PageToken != "") {
			return toolError(fmt.Errorf("all cannot be combined with page_size or page_token"))
		}
		if args.View != "" && args.View != "detail" && args.View != "identity" {
			return toolError(fmt.Errorf("view must be detail or identity"))
		}
		if args.View == "identity" {
			if args.All {
				return toolError(fmt.Errorf("identity is always one page; continue with page_token instead of all"))
			}
			resp, err := client.ListRequirementIdentitiesPage(ctx, args.ProductID, args.Branch, args.PageSize, args.PageToken)
			if err != nil {
				return toolError(err)
			}
			return toolResult(resp)
		}
		var resp *api.ListRequirementsResponse
		var err error
		if args.All {
			resp, err = client.ListRequirements(ctx, args.ProductID, args.Branch)
		} else {
			resp, err = client.ListRequirementsPage(ctx, args.ProductID, args.Branch, args.PageSize, args.PageToken)
		}
		if err != nil {
			return toolError(err)
		}
		return toolResult(resp)
	})
}

// -- get_spec -----------------------------------------------------------------

type getRequirementArgs struct {
	ID     string `json:"id"               jsonschema:"Spec UUID (required)."`
	Branch string `json:"branch,omitempty" jsonschema:"Branch name. Returns that branch's view of the spec (its copy-on-write copy when one exists). Omit for the main row."`
}

func registerGetRequirement(srv *mcp.Server, client *api.Client) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_spec",
		Description: "Fetch a single spec by its UUID, including title, description, status, priority, and hierarchy. Pass branch to read the spec as an intent branch sees it. In the API a spec is the `requirement` resource.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args getRequirementArgs) (*mcp.CallToolResult, any, error) {
		if args.ID == "" {
			return toolError(errMissingField("id"))
		}
		req, err := client.GetRequirement(ctx, args.ID, args.Branch)
		if err != nil {
			return toolError(err)
		}
		return toolResult(req)
	})
}

// -- create_spec --------------------------------------------------------------

type createRequirementArgs struct {
	ProductID   string                         `json:"product_id"           jsonschema:"Product ID (required)."`
	Title       string                         `json:"title"                jsonschema:"Spec title (required)."`
	Description string                         `json:"description,omitempty" jsonschema:"Markdown description body (optional)."`
	ParentID    string                         `json:"parent_id,omitempty"  jsonschema:"UUID of the parent spec for nested hierarchy (optional)."`
	Branch      string                         `json:"branch,omitempty"     jsonschema:"Branch name. Omit to create on the main branch."`
	Sources     []model.RequirementSourceInput `json:"sources,omitempty"    jsonschema:"Structured spec sources/provenance. Omit to leave empty."`
}

func registerCreateRequirement(srv *mcp.Server, client *api.Client) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "create_spec",
		Description: "Create a new spec in a product. Optionally nest it under a parent, create it on a non-main branch, and attach structured sources.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args createRequirementArgs) (*mcp.CallToolResult, any, error) {
		if args.ProductID == "" {
			return toolError(errMissingField("product_id"))
		}
		if args.Title == "" {
			return toolError(errMissingField("title"))
		}
		req, err := client.CreateRequirementTypedWithSources(ctx, args.ProductID, args.Title, args.Description, ptr(args.ParentID), nil, "", args.Sources, args.Branch)
		if err != nil {
			return toolError(err)
		}
		return toolResult(req)
	})
}

// -- update_spec --------------------------------------------------------------

type updateRequirementArgs struct {
	ID            string                          `json:"id"                       jsonschema:"Spec UUID (required)."`
	Title         string                          `json:"title,omitempty"          jsonschema:"New title (optional - omit to leave unchanged)."`
	Description   string                          `json:"description,omitempty"    jsonschema:"New markdown description (optional - omit to leave unchanged)."`
	Status        string                          `json:"status,omitempty"         jsonschema:"New status: Backlog, Active, Review, or Done (optional)."`
	Priority      string                          `json:"priority,omitempty"       jsonschema:"New priority: Not set, Low, Medium, High, or Critical (optional)."`
	Branch        string                          `json:"branch,omitempty"         jsonschema:"Branch name for the update (optional - defaults to main)."`
	SourcesUpdate *model.RequirementSourcesUpdate `json:"sources_update,omitempty" jsonschema:"Whole-set replacement for structured sources. Omit to leave sources unchanged; pass {sources: []} to clear."`
}

func registerUpdateRequirement(srv *mcp.Server, client *api.Client) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "update_spec",
		Description: "Update an existing spec's title, description, status, priority, or structured sources. Only supplied fields are changed.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args updateRequirementArgs) (*mcp.CallToolResult, any, error) {
		if args.ID == "" {
			return toolError(errMissingField("id"))
		}
		body := api.UpdateRequirementRequest{
			Title:         ptr(args.Title),
			Content:       ptr(args.Description),
			Status:        ptr(args.Status),
			Priority:      ptr(args.Priority),
			Branch:        args.Branch,
			SourcesUpdate: args.SourcesUpdate,
		}
		req, err := client.UpdateRequirement(ctx, args.ID, body)
		if err != nil {
			return toolError(err)
		}
		return toolResult(req)
	})
}

// -- list_tests ---------------------------------------------------------------

type listTestsArgs struct {
	ProductID string `json:"product_id"       jsonschema:"Product ID (required)."`
	Branch    string `json:"branch,omitempty" jsonschema:"Branch name. Omit for the main branch."`
	PageSize  int    `json:"page_size,omitempty" jsonschema:"Tests per page (1–200; defaults to 100)."`
	PageToken string `json:"page_token,omitempty" jsonschema:"Continuation token from pagination.nextPageToken. Valid only with the page_size of the call that returned it."`
	All       bool   `json:"all,omitempty" jsonschema:"Explicitly fetch up to 100 pages of 200 tests. Cannot combine with paging arguments."`
	View      string `json:"view,omitempty" jsonschema:"detail (default) or identity (read-only matching fields without bodies, steps or counts; always one page, incompatible with all)."`
}

func registerListTests(srv *mcp.Server, client *api.Client) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_tests",
		Description: "List one page of test suites and cases for a product, optionally scoped to a branch (100 items by default). Continue with pagination.nextPageToken as page_token. Use all=true only for a full detail catalog traversal (up to 100 pages; a remaining nextPageToken means the result is incomplete). Each item has a 'kind' field: 'suite' or 'case'. Opt into view=identity for separate read-only identities with tags and signatures but no descriptions, steps, execution payloads or counts; fetch get_test before editing omitted fields. Identity is always one page and cannot combine with all.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args listTestsArgs) (*mcp.CallToolResult, any, error) {
		if args.ProductID == "" {
			return toolError(errMissingField("product_id"))
		}
		if args.All && (args.PageSize != 0 || args.PageToken != "") {
			return toolError(fmt.Errorf("all cannot be combined with page_size or page_token"))
		}
		if args.View != "" && args.View != "detail" && args.View != "identity" {
			return toolError(fmt.Errorf("view must be detail or identity"))
		}
		if args.View == "identity" {
			if args.All {
				return toolError(fmt.Errorf("identity is always one page; continue with page_token instead of all"))
			}
			response, err := client.ListTestIdentitiesPage(ctx, args.ProductID, args.Branch, args.PageSize, args.PageToken)
			if err != nil {
				return toolError(err)
			}
			return toolResult(response)
		}
		var resp *api.ListTestsResponse
		var err error
		if args.All {
			resp, err = client.ListTests(ctx, args.ProductID, args.Branch)
		} else {
			resp, err = client.ListTestsPage(ctx, args.ProductID, args.Branch, args.PageSize, args.PageToken)
		}
		if err != nil {
			return toolError(err)
		}
		return toolResult(resp)
	})
}

// -- get_test -----------------------------------------------------------------

type getTestArgs struct {
	ID string `json:"id" jsonschema:"Test UUID (required)."`
}

func registerGetTest(srv *mcp.Server, client *api.Client) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_test",
		Description: "Fetch a single test suite or case by its UUID, including steps, status, priority, and linked specs.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args getTestArgs) (*mcp.CallToolResult, any, error) {
		if args.ID == "" {
			return toolError(errMissingField("id"))
		}
		test, err := client.GetTest(ctx, args.ID)
		if err != nil {
			return toolError(err)
		}
		return toolResult(test)
	})
}

// -- list_branches -------------------------------------------------------------

type listBranchesArgs struct {
	ProductID string `json:"product_id" jsonschema:"Product ID (required)."`
}

func registerListBranches(srv *mcp.Server, client *api.Client) {
	mcp.AddTool(srv, &mcp.Tool{
		Name: "list_branches",
		Description: "List all branches for a product, including their status (open, merged, closed) and metadata. " +
			"An intent branch also carries autoMerge, where Tiden's automatic merge of it stands (state, why it still waits, when it merges, any hold), " +
			"and a merged branch carries mergeMode and mergedByName, which say how and by whom it was merged.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args listBranchesArgs) (*mcp.CallToolResult, any, error) {
		if args.ProductID == "" {
			return toolError(errMissingField("product_id"))
		}
		resp, err := client.ListBranches(ctx, args.ProductID)
		if err != nil {
			return toolError(err)
		}
		return toolResult(resp)
	})
}

// -- create_branch -------------------------------------------------------------

type createBranchArgs struct {
	ProductID   string `json:"product_id"           jsonschema:"Product ID (required)."`
	Name        string `json:"name"                 jsonschema:"Branch name (required). Must match ^[a-z0-9][a-z0-9/_-]*$ and be unique within the product."`
	Description string `json:"description,omitempty" jsonschema:"Human-readable description of the branch's purpose (optional)."`
}

func registerCreateBranch(srv *mcp.Server, client *api.Client) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "create_branch",
		Description: "Create a new branch off main in a product. Use branches to isolate spec and test changes before merging.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args createBranchArgs) (*mcp.CallToolResult, any, error) {
		if args.ProductID == "" {
			return toolError(errMissingField("product_id"))
		}
		if args.Name == "" {
			return toolError(errMissingField("name"))
		}
		branch, err := client.CreateBranch(ctx, args.ProductID, args.Name, args.Description)
		if err != nil {
			return toolError(err)
		}
		return toolResult(branch)
	})
}

// -- get_merge_preview ---------------------------------------------------------

type getMergePreviewArgs struct {
	BranchID string `json:"branch_id" jsonschema:"Branch UUID (required)."`
}

func registerGetMergePreview(srv *mcp.Server, client *api.Client) {
	mcp.AddTool(srv, &mcp.Tool{
		Name: "get_merge_preview",
		Description: "Read-only preview of what merging a branch into main would change. Returns additions, modifications, deletions, and any conflicts that need resolution. " +
			"For an intent branch it also returns autoMerge, the state of the automatic merge.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args getMergePreviewArgs) (*mcp.CallToolResult, any, error) {
		if args.BranchID == "" {
			return toolError(errMissingField("branch_id"))
		}
		preview, err := client.GetMergePreview(ctx, args.BranchID)
		if err != nil {
			return toolError(err)
		}
		return toolResult(preview)
	})
}

// -- hold_intent ---------------------------------------------------------------

// defaultHold is how long a hold lasts when the caller gives neither until nor
// for. The server owns the longest hold it accepts and refuses a longer one.
const defaultHold = 7 * 24 * time.Hour

// maxHoldDays keeps a days value small enough that converting it to a
// time.Duration cannot overflow.
const maxHoldDays = math.MaxInt64 / int64(24*time.Hour)

type holdIntentArgs struct {
	BranchID string `json:"branch_id"        jsonschema:"Intent branch UUID (required). Find it with list_branches."`
	Reason   string `json:"reason,omitempty" jsonschema:"Why the branch is held, for example the next pull request that is planned. Required unless clear is true."`
	For      string `json:"for,omitempty"    jsonschema:"How long the hold lasts, counted from now: a number of days such as 3d, or a duration such as 12h. Default 7d when neither for nor until is given. Do not combine with until."`
	Until    string `json:"until,omitempty"  jsonschema:"When the hold ends, as an RFC 3339 timestamp such as 2026-11-01T09:00:00Z. Do not combine with for."`
	Clear    bool   `json:"clear,omitempty"  jsonschema:"Release the current hold instead of setting one. Then do not pass for or until."`
}

func registerHoldIntent(srv *mcp.Server, client *api.Client) {
	mcp.AddTool(srv, &mcp.Tool{
		Name: "hold_intent",
		Description: "Hold or release the automatic merge of a Tiden intent branch. " +
			"Tiden merges a finished intent branch by itself about 24 hours after its last linked pull request merges. " +
			"Set a hold before handing back when more pull requests for the same intent are planned: a hold stops every automatic-merge timer until it ends, and the timers count again from the end of the hold. " +
			"Pass reason and, optionally, either for (3d, 12h; default 7d) or until (an RFC 3339 timestamp). The server refuses a hold that ends in the past or later than it allows. " +
			"Pass clear=true to release the current hold. " +
			"A branch that is already merged cannot be held (BRANCH_NOT_OPEN). " +
			"Returns autoMerge: the state of the automatic merge after the change, including holdUntil, holdReason and heldByName. " +
			"This tool does not merge a branch and does not reopen one.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args holdIntentArgs) (*mcp.CallToolResult, any, error) {
		if args.BranchID == "" {
			return toolError(errMissingField("branch_id"))
		}
		body := api.SetIntentHoldRequest{Source: "agent"}
		if args.Clear {
			if args.For != "" || args.Until != "" {
				return toolError(fmt.Errorf("clear releases the hold; do not pass for or until with it"))
			}
			body.Clear = true
			body.Reason = strings.TrimSpace(args.Reason)
		} else {
			reason := strings.TrimSpace(args.Reason)
			if reason == "" {
				return toolError(errMissingField("reason"))
			}
			until, err := holdEnd(args.For, args.Until, time.Now())
			if err != nil {
				return toolError(err)
			}
			body.Reason = reason
			body.Until = until.UTC().Format(time.RFC3339)
		}
		resp, err := client.SetIntentHold(ctx, args.BranchID, body)
		if err != nil {
			return toolError(err)
		}
		return toolResult(resp)
	})
}

// holdEnd works out when a hold ends from the for and until inputs, counting
// for from now. With neither input the hold lasts defaultHold.
func holdEnd(forInput, untilInput string, now time.Time) (time.Time, error) {
	forInput = strings.TrimSpace(forInput)
	untilInput = strings.TrimSpace(untilInput)
	switch {
	case forInput != "" && untilInput != "":
		return time.Time{}, fmt.Errorf("pass either for or until, not both")
	case untilInput != "":
		t, err := time.Parse(time.RFC3339, untilInput)
		if err != nil {
			return time.Time{}, fmt.Errorf("until must be an RFC 3339 timestamp such as 2026-11-01T09:00:00Z: %w", err)
		}
		return t, nil
	case forInput != "":
		d, err := parseHoldDuration(forInput)
		if err != nil {
			return time.Time{}, err
		}
		return now.Add(d), nil
	default:
		return now.Add(defaultHold), nil
	}
}

// parseHoldDuration reads a whole number of days ("3d") or a Go duration
// ("12h", "90m"). It accepts positive values only.
func parseHoldDuration(s string) (time.Duration, error) {
	invalid := fmt.Errorf("for must be a positive duration such as 3d or 12h, got %q", s)
	if days, ok := strings.CutSuffix(s, "d"); ok {
		n, err := strconv.ParseInt(days, 10, 64)
		if err != nil || n <= 0 || n > maxHoldDays {
			return 0, invalid
		}
		return time.Duration(n) * 24 * time.Hour, nil
	}
	d, err := time.ParseDuration(s)
	if err != nil || d <= 0 {
		return 0, invalid
	}
	return d, nil
}

// -- list_components -----------------------------------------------------------

type listComponentsArgs struct {
	ProductID string `json:"product_id" jsonschema:"Product ID (required)."`
}

func registerListComponents(srv *mcp.Server, client *api.Client) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_components",
		Description: "List a product's components with their repository scope. A component is a deployment/architectural unit (a service, or a build-manifest tier such as frontend/backend) that specs and tests are grouped under; each may carry a repository + path scope (repository/componentPaths/repositoryAliases) mapping it to code.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args listComponentsArgs) (*mcp.CallToolResult, any, error) {
		if args.ProductID == "" {
			return toolError(errMissingField("product_id"))
		}
		resp, err := client.ListComponents(ctx, args.ProductID)
		if err != nil {
			return toolError(err)
		}
		return toolResult(resp)
	})
}

// -- list_environments ---------------------------------------------------------

type listEnvironmentsArgs struct {
	ProductID string `json:"product_id" jsonschema:"Product ID (required)."`
}

func registerListEnvironments(srv *mcp.Server, client *api.Client) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_environments",
		Description: "List deployment environments configured for a product (e.g. dev, staging, prod).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args listEnvironmentsArgs) (*mcp.CallToolResult, any, error) {
		if args.ProductID == "" {
			return toolError(errMissingField("product_id"))
		}
		resp, err := client.ListEnvironments(ctx, args.ProductID)
		if err != nil {
			return toolError(err)
		}
		return toolResult(resp)
	})
}

// -- list_releases -------------------------------------------------------------

type listReleasesArgs struct {
	ProductID string `json:"product_id" jsonschema:"Product ID (required)."`
}

func registerListReleases(srv *mcp.Server, client *api.Client) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_releases",
		Description: "List releases for a product across all environments.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args listReleasesArgs) (*mcp.CallToolResult, any, error) {
		if args.ProductID == "" {
			return toolError(errMissingField("product_id"))
		}
		resp, err := client.ListReleases(ctx, args.ProductID)
		if err != nil {
			return toolError(err)
		}
		return toolResult(resp)
	})
}

// -- helpers -------------------------------------------------------------------

func errMissingField(name string) error {
	return fmt.Errorf("%s is required", name)
}
