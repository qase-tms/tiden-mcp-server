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
| `gate_check` | Compute a Quality Gate verdict |
| `get_verdict` | Read the latest Quality Gate verdict |
| `get_overview` | Current product gate state on main |
| `get_traceability` | Requirement-to-test traceability matrix |
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

All tools return compact JSON matching the Tiden public API response shapes.
