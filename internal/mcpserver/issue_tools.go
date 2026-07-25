package mcpserver

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/qase-tms/tiden-mcp-server/internal/api"
)

// Issue tools expose Tiden's error tracking to an agent: discover captured
// errors, read one occurrence with symbolicated stack frames, learn where it is
// happening, and close it once the fix is proven.
//
// Every genuinely optional argument here carries ,omitempty in its json tag.
// jsonschema-go marks any field without it as schema-required, which makes the
// SDK reject calls that omit the field before the handler ever runs. The older
// tools in tools.go predate that discovery; do not copy their tags.
//
// An IssueEvent's Payload is the full raw event JSON as the SDK sent it. It is
// large enough to swamp an agent's context and the symbolicated frames carry
// everything needed to fix the bug, so these tools clear it unless
// include_payload is set.

// -- list_issues ---------------------------------------------------------------

type listIssuesArgs struct {
	ProductID     string   `json:"product_id"               jsonschema:"Product ID (required)."`
	Status        string   `json:"status,omitempty"         jsonschema:"Filter by status: unresolved (default), resolved, ignored, or all."`
	EnvironmentID string   `json:"environment_id,omitempty" jsonschema:"Environment UUID. Use list_environments to resolve a name such as production."`
	ReleaseID     string   `json:"release_id,omitempty"     jsonschema:"Release UUID. Use list_releases to resolve a version."`
	ComponentID   string   `json:"component_id,omitempty"   jsonschema:"Component UUID. Use list_components to resolve a name."`
	Levels        []string `json:"levels,omitempty"         jsonschema:"Filter by level: fatal, error, warning, info, debug."`
	Platforms     []string `json:"platforms,omitempty"      jsonschema:"Filter by platform, e.g. go, javascript, python."`
	Period        string   `json:"period,omitempty"         jsonschema:"Trailing last-seen window: 3m, 1h, 12h, 1d, 7d, 30d. Omit for all time."`
	Sort          string   `json:"sort,omitempty"           jsonschema:"Sort by last_seen (default), first_seen, or times_seen."`
	Limit         int      `json:"limit,omitempty"          jsonschema:"Page size, 1-200 (default 50)."`
	PageToken     string   `json:"page_token,omitempty"     jsonschema:"Opaque cursor from a previous page."`
}

func registerListIssues(srv *mcp.Server, client *api.Client) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_issues",
		Description: "List a product's captured errors (issues), newest activity first. Filter by status, environment, release, component, level, platform, and a trailing time window; sort by recency, age, or event count. Start here when asked what is broken in production. An issue has no environment of its own — filter by environment_id, or call get_issue_event_stats for the split.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args listIssuesArgs) (*mcp.CallToolResult, any, error) {
		if args.ProductID == "" {
			return toolError(errMissingField("product_id"))
		}
		status := args.Status
		if status == "all" {
			status = ""
		}
		resp, err := client.ListIssues(ctx, args.ProductID, api.ListIssuesOptions{
			Status:        status,
			EnvironmentID: args.EnvironmentID,
			ReleaseID:     args.ReleaseID,
			ComponentID:   args.ComponentID,
			Levels:        args.Levels,
			Platforms:     args.Platforms,
			Period:        args.Period,
			Sort:          args.Sort,
			PageSize:      args.Limit,
			PageToken:     args.PageToken,
		})
		if err != nil {
			return toolError(err)
		}
		return toolResult(resp)
	})
}

// -- get_issue -----------------------------------------------------------------

type getIssueArgs struct {
	ID             string `json:"id"                        jsonschema:"Issue UUID (required)."`
	IncludePayload bool   `json:"include_payload,omitempty" jsonschema:"Include the full raw event JSON. Large — omit unless the symbolicated frames were not enough."`
}

func registerGetIssue(srv *mcp.Server, client *api.Client) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_issue",
		Description: "Get one issue with its most recent occurrence, including symbolicated stack frames with surrounding source lines. The raw event payload is omitted unless include_payload is set. If you intend to fix the error, call get_issue_fix_context instead — it returns this plus the implicated files, the environment split, and their test coverage.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args getIssueArgs) (*mcp.CallToolResult, any, error) {
		if args.ID == "" {
			return toolError(errMissingField("id"))
		}
		resp, err := client.GetIssue(ctx, args.ID)
		if err != nil {
			return toolError(err)
		}
		if !args.IncludePayload && resp.LatestEvent != nil {
			resp.LatestEvent.Payload = ""
		}
		return toolResult(resp)
	})
}

// -- list_issue_events ---------------------------------------------------------

type listIssueEventsArgs struct {
	ID             string `json:"id"                        jsonschema:"Issue UUID (required)."`
	Limit          int    `json:"limit,omitempty"           jsonschema:"Page size, 1-200 (default 50)."`
	PageToken      string `json:"page_token,omitempty"      jsonschema:"Opaque cursor from a previous page."`
	IncludePayload bool   `json:"include_payload,omitempty" jsonschema:"Include each occurrence's full raw event JSON. Very large across a page — omit it."`
}

func registerListIssueEvents(srv *mcp.Server, client *api.Client) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_issue_events",
		Description: "List an issue's individual occurrences, newest first, with their release and environment. Use it to find the occurrence you actually care about — for example the production one when the latest event came from dev — then pass its eventId to get_issue_event. Stack frames are not resolved here; fetch a single occurrence for those. Raw payloads are omitted unless include_payload is set.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args listIssueEventsArgs) (*mcp.CallToolResult, any, error) {
		if args.ID == "" {
			return toolError(errMissingField("id"))
		}
		resp, err := client.ListIssueEvents(ctx, args.ID, args.Limit, args.PageToken)
		if err != nil {
			return toolError(err)
		}
		if !args.IncludePayload {
			for i := range resp.Events {
				resp.Events[i].Payload = ""
			}
		}
		return toolResult(resp)
	})
}

// -- get_issue_event -----------------------------------------------------------

type getIssueEventArgs struct {
	IssueID        string `json:"issue_id"                  jsonschema:"Issue UUID (required)."`
	EventID        string `json:"event_id"                  jsonschema:"Occurrence UUID (required) — the id of an event from list_issue_events."`
	IncludePayload bool   `json:"include_payload,omitempty" jsonschema:"Include the full raw event JSON. Large — omit unless the symbolicated frames were not enough."`
}

func registerGetIssueEvent(srv *mcp.Server, client *api.Client) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_issue_event",
		Description: "Get one specific occurrence of an issue with symbolicated stack frames and surrounding source lines. Use this when the occurrence you need is not the most recent — for example the production occurrence of an issue whose latest event came from staging. The raw event payload is omitted unless include_payload is set.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args getIssueEventArgs) (*mcp.CallToolResult, any, error) {
		if args.IssueID == "" {
			return toolError(errMissingField("issue_id"))
		}
		if args.EventID == "" {
			return toolError(errMissingField("event_id"))
		}
		ev, err := client.GetIssueEvent(ctx, args.IssueID, args.EventID)
		if err != nil {
			return toolError(err)
		}
		if !args.IncludePayload && ev != nil {
			ev.Payload = ""
		}
		return toolResult(ev)
	})
}

// -- get_issue_event_stats -----------------------------------------------------

type getIssueEventStatsArgs struct {
	ID string `json:"id" jsonschema:"Issue UUID (required)."`
}

func registerGetIssueEventStats(srv *mcp.Server, client *api.Client) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_issue_event_stats",
		Description: "Occurrence statistics for one issue: counts bucketed over time, the last-24h total, and the per-environment split. An issue carries no environment of its own, so this is the only way to learn whether it is happening in production or is only dev noise — check it before deciding an error is urgent. get_issue_fix_context already includes this split.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args getIssueEventStatsArgs) (*mcp.CallToolResult, any, error) {
		if args.ID == "" {
			return toolError(errMissingField("id"))
		}
		stats, err := client.GetIssueEventStats(ctx, args.ID)
		if err != nil {
			return toolError(err)
		}
		return toolResult(stats)
	})
}

// -- get_issue_fix_context -----------------------------------------------------

type getIssueFixContextArgs struct {
	ProductID string `json:"product_id"           jsonschema:"Product ID (required)."`
	IssueID   string `json:"issue_id"             jsonschema:"Issue UUID (required)."`
	Branch    string `json:"branch,omitempty"     jsonschema:"Branch for requirement lookup. Omit for main."`
	MaxFrames int    `json:"max_frames,omitempty" jsonschema:"In-app stack frames to include (default 10, max 50)."`
}

func registerGetIssueFixContext(srv *mcp.Server, client *api.Client) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_issue_fix_context",
		Description: "Everything needed to fix one error, in one call: the issue, its symbolicated stack frames, the repo files those frames implicate (suspectPaths — open these first), where it is happening by environment, and — for each requirement those files implement — whether a test already covers it. This replaces combining get_issue, get_issue_event_stats and requirement lookups: one round-trip, and it reports which suspect path matched which requirement so a wrong match is visible. Start every fix here.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args getIssueFixContextArgs) (*mcp.CallToolResult, any, error) {
		if args.ProductID == "" {
			return toolError(errMissingField("product_id"))
		}
		if args.IssueID == "" {
			return toolError(errMissingField("issue_id"))
		}
		fc, err := client.GetIssueFixContext(ctx, args.ProductID, args.IssueID, args.Branch, args.MaxFrames)
		if err != nil {
			return toolError(err)
		}
		// The server already clears the raw payload on this endpoint. Clearing
		// it again keeps the guarantee local: this tool has no include_payload
		// argument, so it must never emit one.
		if fc != nil && fc.LatestEvent != nil {
			fc.LatestEvent.Payload = ""
		}
		return toolResult(fc)
	})
}

// -- list_release_issues -------------------------------------------------------

type listReleaseIssuesArgs struct {
	ReleaseID string `json:"release_id" jsonschema:"Release UUID (required). Use list_releases to resolve a version."`
}

func registerListReleaseIssues(srv *mcp.Server, client *api.Client) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_release_issues",
		Description: "The post-deploy regression check for one release: the issues first seen in it — newly shipped bugs and regressions — plus a count of all issues seen during it. Call it after shipping, or when asked whether a deploy broke anything.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args listReleaseIssuesArgs) (*mcp.CallToolResult, any, error) {
		if args.ReleaseID == "" {
			return toolError(errMissingField("release_id"))
		}
		resp, err := client.ListReleaseIssues(ctx, args.ReleaseID)
		if err != nil {
			return toolError(err)
		}
		return toolResult(resp)
	})
}

// -- set_issue_status ----------------------------------------------------------

type setIssueStatusArgs struct {
	ID     string `json:"id"     jsonschema:"Issue UUID (required)."`
	Status string `json:"status" jsonschema:"New status (required): unresolved, resolved, or ignored."`
}

func registerSetIssueStatus(srv *mcp.Server, client *api.Client) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "set_issue_status",
		Description: "Set an issue's status to unresolved, resolved, or ignored. Only resolve an issue whose fix you have verified — every transition is recorded with its actor, and the grouping worker will reopen the issue as a regression if it recurs.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args setIssueStatusArgs) (*mcp.CallToolResult, any, error) {
		if args.ID == "" {
			return toolError(errMissingField("id"))
		}
		if args.Status == "" {
			return toolError(errMissingField("status"))
		}
		iss, err := client.UpdateIssueStatus(ctx, args.ID, args.Status)
		if err != nil {
			return toolError(err)
		}
		return toolResult(iss)
	})
}
