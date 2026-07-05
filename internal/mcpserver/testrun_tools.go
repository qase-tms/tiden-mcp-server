package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/qase-tms/tiden-mcp-server/internal/api"
)

// registerTestRunTools adds the Test Runs module tools. Runs are product-
// scoped CI/reporter executions with their own per-product seq counter
// ("Run #42") — run_seq is a small integer, never a UUID. delete_test_run is
// deliberately not exposed (destructive), and UpdateTestRun is web-only.
func registerTestRunTools(srv *mcp.Server, client *api.Client) {
	registerListTestRuns(srv, client)
	registerGetTestRun(srv, client)
	registerGetRunResults(srv, client)
	registerReportTestResults(srv, client)
	registerCompleteTestRun(srv, client)
	registerCreateTestRun(srv, client)
	registerAbortTestRun(srv, client)
}

// -- list_test_runs -------------------------------------------------------

type listTestRunsArgs struct {
	ProductID   string `json:"product_id"            jsonschema:"Product ID (required)."`
	Status      string `json:"status,omitempty"      jsonschema:"Filter by run status: new, in_progress, passed, failed, or aborted (optional)."`
	Environment string `json:"environment,omitempty" jsonschema:"Filter by environment slug (optional; an unknown slug returns an empty page, it never errors)."`
	Branch      string `json:"branch,omitempty"      jsonschema:"Filter by exact branch name (optional)."`
	Search      string `json:"search,omitempty"      jsonschema:"Title substring filter (optional)."`
	PageSize    int    `json:"page_size,omitempty"   jsonschema:"Page size (default 20)."`
	PageToken   string `json:"page_token,omitempty"  jsonschema:"Opaque token from a previous response's pagination.nextPageToken (optional)."`
}

func registerListTestRuns(srv *mcp.Server, client *api.Client) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_test_runs",
		Description: "List test runs for a product, optionally filtered by status/environment/branch/title search. A run is one CI/local execution of the automated suite; runs are paginated newest-first.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args listTestRunsArgs) (*mcp.CallToolResult, any, error) {
		if args.ProductID == "" {
			return toolError(errMissingField("product_id"))
		}
		resp, err := client.ListTestRuns(ctx, args.ProductID, api.ListTestRunsOptions{
			Status: args.Status, Environment: args.Environment, Branch: args.Branch,
			Search: args.Search, PageSize: args.PageSize, PageToken: args.PageToken,
		})
		if err != nil {
			return toolError(err)
		}
		return toolResult(resp)
	})
}

// -- get_test_run ----------------------------------------------------------

type getTestRunArgs struct {
	ProductID string `json:"product_id" jsonschema:"Product ID (required)."`
	RunSeq    int    `json:"run_seq"    jsonschema:"The run's per-product sequence number (e.g. 42), NOT a UUID (required)."`
}

func registerGetTestRun(srv *mcp.Server, client *api.Client) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_test_run",
		Description: "Fetch one test run by its per-product sequence number, including stats and the live-documentation sync outcome (liveDocStatus settles from pending to succeeded/failed — or is skipped for aborted runs and products with live documentation disabled — re-poll to observe it).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args getTestRunArgs) (*mcp.CallToolResult, any, error) {
		if args.ProductID == "" {
			return toolError(errMissingField("product_id"))
		}
		if args.RunSeq <= 0 {
			return toolError(fmt.Errorf("run_seq must be a positive integer (the run's per-product sequence number)"))
		}
		run, err := client.GetTestRun(ctx, args.ProductID, args.RunSeq)
		if err != nil {
			return toolError(err)
		}
		return toolResult(run)
	})
}

// -- get_run_results --------------------------------------------------------

type getRunResultsArgs struct {
	ProductID   string `json:"product_id"             jsonschema:"Product ID (required)."`
	RunSeq      int    `json:"run_seq"                 jsonschema:"The run's per-product sequence number (required)."`
	Summary     bool   `json:"summary,omitempty"       jsonschema:"true = pre-aggregated suite tree + per-case rollups with param combos (GetRunSummary); false = flat paginated result rows (default)."`
	Status      string `json:"status,omitempty"        jsonschema:"Flat form only. Filter: passed, failed, blocked, skipped, or invalid. With latest_only it means 'currently red', not 'failed at least once'."`
	Search      string `json:"search,omitempty"        jsonschema:"Flat form only. Title substring filter."`
	IdentityKey string `json:"identity_key,omitempty"  jsonschema:"Flat form only. Exact case identity — all attempts of one case, e.g. its retry history."`
	LatestOnly  bool   `json:"latest_only,omitempty"   jsonschema:"Flat form only. Collapse retries to the single latest attempt per parameter combination BEFORE other filters apply."`
	PageSize    int    `json:"page_size,omitempty"     jsonschema:"Flat form only. Page size (default 50)."`
	PageToken   string `json:"page_token,omitempty"    jsonschema:"Flat form only. Opaque pagination token."`
}

func registerGetRunResults(srv *mcp.Server, client *api.Client) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_run_results",
		Description: "Fetch results for a test run - either the flat, paginated attempt list, or (with summary=true) the pre-aggregated suite-tree + per-case rollup. Use summary=true for an overview; the flat form to page through individual attempts.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args getRunResultsArgs) (*mcp.CallToolResult, any, error) {
		if args.ProductID == "" {
			return toolError(errMissingField("product_id"))
		}
		if args.RunSeq <= 0 {
			return toolError(fmt.Errorf("run_seq must be a positive integer (the run's per-product sequence number)"))
		}
		if args.Summary {
			sum, err := client.GetRunSummary(ctx, args.ProductID, args.RunSeq)
			if err != nil {
				return toolError(err)
			}
			return toolResult(sum)
		}
		resp, err := client.ListRunResults(ctx, args.ProductID, args.RunSeq, api.ListRunResultsOptions{
			Status: args.Status, Search: args.Search, IdentityKey: args.IdentityKey,
			LatestOnly: args.LatestOnly, PageSize: args.PageSize, PageToken: args.PageToken,
		})
		if err != nil {
			return toolError(err)
		}
		return toolResult(resp)
	})
}

// -- report_test_results ------------------------------------------------------

type reportTestResultsArgs struct {
	ProductID string           `json:"product_id" jsonschema:"Product ID (required)."`
	RunSeq    int              `json:"run_seq"    jsonschema:"The run's per-product sequence number (required)."`
	Results   []map[string]any `json:"results"    jsonschema:"Qase v2 ResultCreate-compatible objects (max 2000). Each needs title and execution.status (passed/failed/blocked/skipped/invalid). Strongly recommended: a stable UUID 'id' per entry so retries dedupe. Optional identity: external_id, signature, testops_ids (repository case seq_nums — every listed id must resolve or the batch is rejected). Mute via fields: {\"muted\": \"true\"}. Steps max 500 nodes / 5 levels."`
}

func registerReportTestResults(srv *mcp.Server, client *api.Client) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "report_test_results",
		Description: "Submit a batch of test outcomes to a run (validate-then-write: either every entry is accepted or none are and per-entry errors are returned). Fails on a run that is already passed/failed/aborted. Max 2000 results per call.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args reportTestResultsArgs) (*mcp.CallToolResult, any, error) {
		if args.ProductID == "" {
			return toolError(errMissingField("product_id"))
		}
		if args.RunSeq <= 0 {
			return toolError(fmt.Errorf("run_seq must be a positive integer (the run's per-product sequence number)"))
		}
		if len(args.Results) == 0 {
			return toolError(errMissingField("results"))
		}
		resp, err := client.ReportResults(ctx, args.ProductID, args.RunSeq, args.Results)
		if err != nil {
			var apiErr *api.APIError
			if errors.As(err, &apiErr) {
				if entries, ok := api.ParseReportErrors(apiErr.Raw); ok {
					var b strings.Builder
					fmt.Fprintf(&b, "%s — %d entries rejected, nothing was written:\n", apiErr.Message, len(entries))
					for _, e := range entries {
						id := e.ResultID
						if id == "" {
							id = "<none>"
						}
						fmt.Fprintf(&b, "  #%d (id=%s): %s: %s\n", e.Index, id, e.Code, e.Message)
					}
					return toolError(errors.New(b.String()))
				}
			}
			return toolError(err)
		}
		return toolResult(resp)
	})
}

// -- complete_test_run ---------------------------------------------------------

type completeTestRunArgs struct {
	ProductID string `json:"product_id" jsonschema:"Product ID (required)."`
	RunSeq    int    `json:"run_seq"    jsonschema:"The run's per-product sequence number (required)."`
}

func registerCompleteTestRun(srv *mcp.Server, client *api.Client) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "complete_test_run",
		Description: "Finalize a test run: computes the pass/fail verdict once from the latest attempt of each reported test (muted failures never fail a run), locks the run against further results, and kicks off the background live-documentation sync if enabled. Idempotent if already completed; fails if the run was aborted.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args completeTestRunArgs) (*mcp.CallToolResult, any, error) {
		if args.ProductID == "" {
			return toolError(errMissingField("product_id"))
		}
		if args.RunSeq <= 0 {
			return toolError(fmt.Errorf("run_seq must be a positive integer (the run's per-product sequence number)"))
		}
		run, err := client.CompleteTestRun(ctx, args.ProductID, args.RunSeq)
		if err != nil {
			return toolError(err)
		}
		return toolResult(run)
	})
}

// -- create_test_run -----------------------------------------------------------

type createTestRunArgs struct {
	ProductID      string            `json:"product_id"                jsonschema:"Product ID (required)."`
	Title          string            `json:"title,omitempty"           jsonschema:"Run title (optional - server defaults to 'Automated run <timestamp>')."`
	Description    string            `json:"description,omitempty"     jsonschema:"Run description (optional)."`
	Environment    string            `json:"environment,omitempty"     jsonschema:"Environment slug (optional; an unknown slug auto-creates the environment)."`
	Branch         string            `json:"branch,omitempty"          jsonschema:"CI branch name, free text (optional). On completion the live-doc sync resolves/creates the Tiden branch of this name."`
	BuildSha       string            `json:"build_sha,omitempty"       jsonschema:"Build/commit SHA (optional)."`
	Configurations map[string]string `json:"configurations,omitempty"  jsonschema:"Free-form configuration map, e.g. {\"browser\": \"chromium\"} (optional)."`
	ClientMeta     map[string]string `json:"client_meta,omitempty"     jsonschema:"Reporter metadata, e.g. {\"framework\": \"playwright\"} (optional)."`
}

func registerCreateTestRun(srv *mcp.Server, client *api.Client) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "create_test_run",
		Description: "Create a test run (status 'new') to report results into. Returns the run including its assigned per-product seq number — pass that run_seq to report_test_results/complete_test_run.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args createTestRunArgs) (*mcp.CallToolResult, any, error) {
		if args.ProductID == "" {
			return toolError(errMissingField("product_id"))
		}
		run, err := client.CreateTestRun(ctx, args.ProductID, api.CreateTestRunRequest{
			Title: args.Title, Description: args.Description, Environment: args.Environment,
			Branch: args.Branch, BuildSha: args.BuildSha,
			Configurations: args.Configurations, ClientMeta: args.ClientMeta,
		})
		if err != nil {
			return toolError(err)
		}
		return toolResult(run)
	})
}

// -- abort_test_run ------------------------------------------------------------

type abortTestRunArgs struct {
	ProductID string `json:"product_id" jsonschema:"Product ID (required)."`
	RunSeq    int    `json:"run_seq"    jsonschema:"The run's per-product sequence number (required)."`
}

func registerAbortTestRun(srv *mcp.Server, client *api.Client) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "abort_test_run",
		Description: "Abort a test run (terminal; only legal from status new or in_progress). Aborted runs never trigger live-documentation sync.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args abortTestRunArgs) (*mcp.CallToolResult, any, error) {
		if args.ProductID == "" {
			return toolError(errMissingField("product_id"))
		}
		if args.RunSeq <= 0 {
			return toolError(fmt.Errorf("run_seq must be a positive integer (the run's per-product sequence number)"))
		}
		run, err := client.AbortTestRun(ctx, args.ProductID, args.RunSeq)
		if err != nil {
			return toolError(err)
		}
		return toolResult(run)
	})
}
