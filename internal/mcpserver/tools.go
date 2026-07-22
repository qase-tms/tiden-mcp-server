package mcpserver

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/qase-tms/tiden-mcp-server/internal/api"
	"github.com/qase-tms/tiden-mcp-server/internal/model"
)

// registerTools adds all Tiden MCP tools to the server. Each tool is a
// thin wrapper over an existing api.Client method - no domain logic here.
func registerTools(srv *mcp.Server, client *api.Client, defaultWorkspaceID string) {
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
	registerListComponents(srv, client)
	registerListEnvironments(srv, client)
	registerListReleases(srv, client)
	registerGateCheck(srv, client)
	registerGetVerdict(srv, client)
	registerGetOverview(srv, client)
	registerGetTraceability(srv, client)
	registerCreateTest(srv, client)
	registerUpdateTest(srv, client)
	registerLinkRequirement(srv, client)
	registerTestRunTools(srv, client)
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

// -- list_requirements --------------------------------------------------------

type listRequirementsArgs struct {
	ProductID string `json:"product_id"          jsonschema:"Product ID (required)."`
	Branch    string `json:"branch,omitempty"    jsonschema:"Branch name. Omit for the main branch."`
}

func registerListRequirements(srv *mcp.Server, client *api.Client) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_requirements",
		Description: "List all requirements for a product, optionally scoped to a branch; use branch to view branch-local copies.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args listRequirementsArgs) (*mcp.CallToolResult, any, error) {
		if args.ProductID == "" {
			return toolError(errMissingField("product_id"))
		}
		resp, err := client.ListRequirements(ctx, args.ProductID, args.Branch)
		if err != nil {
			return toolError(err)
		}
		return toolResult(resp)
	})
}

// -- get_requirement ----------------------------------------------------------

type getRequirementArgs struct {
	ID string `json:"id" jsonschema:"Requirement UUID (required)."`
}

func registerGetRequirement(srv *mcp.Server, client *api.Client) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_requirement",
		Description: "Fetch a single requirement by its UUID, including title, description, status, priority, and hierarchy.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args getRequirementArgs) (*mcp.CallToolResult, any, error) {
		if args.ID == "" {
			return toolError(errMissingField("id"))
		}
		req, err := client.GetRequirement(ctx, args.ID)
		if err != nil {
			return toolError(err)
		}
		return toolResult(req)
	})
}

// -- create_requirement -------------------------------------------------------

type createRequirementArgs struct {
	ProductID   string                         `json:"product_id"           jsonschema:"Product ID (required)."`
	Title       string                         `json:"title"                jsonschema:"Requirement title (required)."`
	Description string                         `json:"description,omitempty" jsonschema:"Markdown description body (optional)."`
	ParentID    string                         `json:"parent_id,omitempty"  jsonschema:"UUID of the parent requirement for nested hierarchy (optional)."`
	Branch      string                         `json:"branch,omitempty"     jsonschema:"Branch name. Omit to create on the main branch."`
	Sources     []model.RequirementSourceInput `json:"sources,omitempty"    jsonschema:"Structured requirement sources/provenance. Omit to leave empty."`
}

func registerCreateRequirement(srv *mcp.Server, client *api.Client) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "create_requirement",
		Description: "Create a new requirement in a product. Optionally nest it under a parent, create it on a non-main branch, and attach structured sources.",
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

// -- update_requirement -------------------------------------------------------

type updateRequirementArgs struct {
	ID            string                          `json:"id"                       jsonschema:"Requirement UUID (required)."`
	Title         string                          `json:"title,omitempty"          jsonschema:"New title (optional - omit to leave unchanged)."`
	Description   string                          `json:"description,omitempty"    jsonschema:"New markdown description (optional - omit to leave unchanged)."`
	Status        string                          `json:"status,omitempty"         jsonschema:"New status: Backlog, Active, Review, or Done (optional)."`
	Priority      string                          `json:"priority,omitempty"       jsonschema:"New priority: Not set, Low, Medium, High, or Critical (optional)."`
	Branch        string                          `json:"branch,omitempty"         jsonschema:"Branch name for the update (optional - defaults to main)."`
	SourcesUpdate *model.RequirementSourcesUpdate `json:"sources_update,omitempty" jsonschema:"Whole-set replacement for structured sources. Omit to leave sources unchanged; pass {sources: []} to clear."`
}

func registerUpdateRequirement(srv *mcp.Server, client *api.Client) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "update_requirement",
		Description: "Update an existing requirement's title, description, status, priority, or structured sources. Only supplied fields are changed.",
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
}

func registerListTests(srv *mcp.Server, client *api.Client) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_tests",
		Description: "List all test suites and cases for a product, optionally scoped to a branch. Each item has a 'kind' field: 'suite' or 'case'.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args listTestsArgs) (*mcp.CallToolResult, any, error) {
		if args.ProductID == "" {
			return toolError(errMissingField("product_id"))
		}
		resp, err := client.ListTests(ctx, args.ProductID, args.Branch)
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
		Description: "Fetch a single test suite or case by its UUID, including steps, status, priority, and linked requirements.",
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
		Name:        "list_branches",
		Description: "List all branches for a product, including their status (open, merged, closed) and metadata.",
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
		Description: "Create a new branch off main in a product. Use branches to isolate requirement and test changes before merging.",
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
		Name:        "get_merge_preview",
		Description: "Read-only preview of what merging a branch into main would change. Returns additions, modifications, deletions, and any conflicts that need resolution.",
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

// -- list_components -----------------------------------------------------------

type listComponentsArgs struct {
	ProductID string `json:"product_id" jsonschema:"Product ID (required)."`
}

func registerListComponents(srv *mcp.Server, client *api.Client) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_components",
		Description: "List a product's components with their repository scope. A component is a deployment/architectural unit (a service, or a build-manifest tier such as frontend/backend) that requirements and tests are grouped under; each may carry a repository + path scope (repository/componentPaths/repositoryAliases) mapping it to code.",
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
