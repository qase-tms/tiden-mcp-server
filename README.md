# tiden-mcp-server

`tiden-mcp-server` is a stdio Model Context Protocol server for Tiden. MCP-capable clients can connect to it and call Tiden as a set of tools.

The server is a thin adapter over the Tiden public REST API. It supports read operations and safe creates/updates used by coding agents.

## Install

Download a prebuilt binary from the public GitHub Releases page:

https://github.com/qase-tms/tiden-mcp-server/releases/latest

Or install from source:

```bash
go install github.com/qase-tms/tiden-mcp-server/cmd/tiden-mcp-server@latest
```

## Configuration

Configuration is resolved from lowest to highest priority:

1. `~/.tiden/config.json`
2. Environment variables: `TIDEN_BASE_URL`, `TIDEN_API_TOKEN`, `TIDEN_WORKSPACE_ID`, `TIDEN_TIMEOUT`
3. Flags: `--base-url`, `--api-token`, `--workspace-id`, `--timeout`

If you already configured the `tiden` CLI with `tiden setup`, this server can reuse the same `~/.tiden/config.json`.

Example config:

```json
{
  "baseUrl": "https://app.tiden.ai",
  "apiToken": "tid_...",
  "workspaceId": "..."
}
```

## Running

```bash
tiden-mcp-server
```

Stdout is the MCP protocol wire. All diagnostics go to stderr.

Register with Claude Code:

```bash
claude mcp add tiden -- tiden-mcp-server
```

## Tools

| Tool | Description |
|---|---|
| `whoami` | Current authenticated user |
| `list_workspaces` | Workspaces the user belongs to |
| `list_products` | Products in a workspace |
| `get_product` | Fetch one product by id (name, code, description) |
| `list_requirements` | Requirements for a product, optionally scoped to a branch |
| `get_requirement` | Fetch one requirement |
| `create_requirement` | Create a requirement |
| `update_requirement` | Update a requirement |
| `list_tests` | Test suites and cases for a product |
| `get_test` | Fetch one test suite or case |
| `list_branches` | Branches for a product |
| `create_branch` | Create a branch off main |
| `get_merge_preview` | Read-only preview of a branch merge |
| `list_components` | Components for a product |
| `list_environments` | Deployment environments for a product |
| `list_releases` | Releases for a product |
| `gate_check` | Compute a Quality Gate verdict (release, branch, or current main) — status, next action, touched requirements, accepted risks |
| `get_verdict` | Read the latest Quality Gate verdict (release, branch, or current main) |
| `get_overview` | Current product gate state on main |
| `get_traceability` | Requirement-to-test traceability matrix (release, branch, or current main) |
| `session_progress` | Per-requirement progress of one intent session: coverage ladder, session attribution, readiness, next actions (does not start/refine/close the session; that stays CLI-only) |
| `record_risk_acceptances` | Record one intent session's risk acceptances and test deferrals (does not close the session; that stays CLI-only) |
| `create_test` | Create a test suite or case |
| `update_test` | Update a test case |
| `link_requirement` | Link a test case to a requirement |
| `list_test_runs` | List test runs for a product (status/environment/branch/search filters, paginated) |
| `get_test_run` | Fetch one run by per-product seq number, incl. stats + live-doc sync outcome |
| `get_run_results` | Run results: flat paginated attempts, or `summary=true` for the suite-tree rollup |
| `report_test_results` | Submit a batch of test outcomes to a run (all-or-nothing, max 2000) |
| `complete_test_run` | Finalize a run: compute verdict, lock results, trigger live-doc sync |
| `create_test_run` | Create a run (status `new`) to report results into |
| `abort_test_run` | Abort a run (terminal; skips live-doc sync) |
| `capture_intent` | Distill a session's product decisions into a reviewable `intent/<date>-<slug>` branch (server-side distiller; may take a few minutes) |
| `list_issues` | Captured errors for a product, newest activity first (status/environment/release/component/level/platform/period filters, paginated) |
| `get_issue` | One issue with its most recent occurrence and symbolicated stack frames |
| `list_issue_events` | An issue's individual occurrences with release + environment, newest first |
| `get_issue_event` | One specific occurrence with symbolicated frames — for when the latest event is not the one you care about |
| `get_issue_event_stats` | Occurrence counts over time, last-24h total, and the per-environment split (the only way to learn an issue's environment) |
| `get_issue_fix_context` | Everything needed to fix one error in one call: frames, implicated repo files, environment split, and the covering tests of the requirements those files implement |
| `list_release_issues` | Post-deploy regression check: issues first seen in a release, plus a count of all issues seen during it |
| `set_issue_status` | Set an issue to unresolved, resolved, or ignored |

All tools return compact JSON matching the Tiden public API response shapes.

Occurrence payloads (the full raw event JSON an SDK sent) are omitted by default
because they are large enough to swamp an agent's context. `get_issue`,
`list_issue_events` and `get_issue_event` take `include_payload: true` when the
symbolicated frames were not enough.
