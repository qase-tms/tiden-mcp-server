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

If you already configured the `tiden` CLI (`tiden setup`), this server reuses
the same `~/.tiden/config.json` and needs no configuration of its own — just
run it from inside the repository the CLI is bound to.

`~/.tiden/config.json` is workspace-keyed (v2), since one account can belong
to several workspaces and one machine can hold several accounts:

```json
{
  "version": 2,
  "workspaces": {
    "1f0a…-work": { "baseUrl": "https://app.tiden.ai", "apiToken": "tdn_…", "account": "you@company.com", "name": "Work" },
    "9e3d…-personal": { "baseUrl": "https://app.tiden.ai", "apiToken": "tdn_…", "account": "me@example.dev", "name": "Personal" }
  }
}
```

The older flat shape (`{"baseUrl", "apiToken", "workspaceId"}`, one login)
still works.

### Which workspace?

Because one machine can be logged into several workspaces, the server picks
one **per repository**, in this order — the first that applies wins:

1. `--api-token`/`TIDEN_API_TOKEN` (with `--base-url`/`TIDEN_BASE_URL`) — an
   explicit token override is used exactly as given, with no server lookup
   at all: its workspace is `--workspace-id`/`TIDEN_WORKSPACE_ID` when set,
   else the repo-local `.tiden/config.json`'s `workspaceId` when this
   directory is bound, else left unset (a tool call that needs one and gets
   none reports that plainly, the same as before TIDEN-68's per-repository
   resolution existed).
2. Without a token override: `--workspace-id`/`TIDEN_WORKSPACE_ID` — selects
   that workspace's entry.
3. The repo-local `.tiden/config.json` (walked up from the working
   directory) — its `workspaceId`, if bound.
4. That file's `productId` — looked up against each logged-in account until
   one can see the product.
5. The repository's `origin` remote — looked up against the server, if it
   already knows this repository.
6. The sole workspace across every logged-in account, when there is exactly
   one.

If the server is started from a directory outside any repository the CLI
has bound (or without a working directory that resolves to one at all) and
more than one workspace is stored, none of the steps above can pick one, so
it exits with the choice list — pass `-workspace-id <id>` (or set
`TIDEN_WORKSPACE_ID`) in the MCP registration for that case.

This is entirely **read-only**: the server never prompts, never binds a
repository, and never writes either config file — that stays the CLI's job
(`tiden setup`, `tiden workspace use`, `tiden product bind`). When none of
the above resolves — an unbound repository visible to several accounts, or a
repository bound to a workspace none of your logged-in accounts belongs to —
the server prints the reason and a suggested `tiden` command to stderr and
exits with status 2, instead of starting with no workspace.

On a successful start it prints one line to stderr naming the workspace,
account and how it was chosen, e.g.:

```
tiden-mcp-server: workspace Acme Inc (9e3d…) as me@example.dev [repo-binding]
```

**Upgrade `tiden` and `tiden-mcp-server` together.** An older
`tiden-mcp-server` binary cannot read the v2 config file (or a repo binding)
and will fail to start once `tiden setup` has migrated your login — always
run the two at the same version.

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
| `list_requirements` | Requirements for a product/branch; `view=identity` returns one read-only page of IDs, titles, hashes and source locators (`page_size` 1–200, `page_token` to continue); default remains the full list |
| `get_requirement` | Fetch one requirement |
| `lookup_context` | Read context for 1–8 independent plan questions, preserving each item's mappings and diagnostics; creates no session or branch |
| `create_requirement` | Create a requirement |
| `update_requirement` | Update a requirement |
| `list_tests` | One page of test suites and cases (default 100); continue with `page_token`, or explicitly traverse detail with `all=true`. `view=identity` is one read-only page of matching fields without bodies/steps/counts; incompatible with `all`. |
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
| `get_run_results` | Flat paginated attempts by default; `summary_view=overview`, `cases`, or `combos` for lean reads (case/combo pages 100, max 200; explicit continuation). `summary=true` without a view retains the full tree. |
| `report_test_results` | Submit a batch of test outcomes to a run (all-or-nothing, max 2000) |
| `complete_test_run` | Finalize a run: compute verdict, lock results, trigger live-doc sync |
| `create_test_run` | Create a run (status `new`) to report results into |
| `abort_test_run` | Abort a run (terminal; skips live-doc sync) |
| `capture_intent` | Distill a session's product decisions into a reviewable `intent/<date>-<slug>` branch (server-side distiller; may take a few minutes) |
| `list_issues` | Captured errors for a product, newest activity first (status/environment/release/component/platform/period filters, paginated) |
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

### Planning context

`lookup_context` accepts `product_id`, optional `branch`, and `items` with stable
`id`, a focused `question`, and optional `anchors` (`repository` in
`github.com/org/repo` form plus repository-relative `path`). Optional
`max_requirements` controls displayed detail (default 12, maximum 40).

Use one item per independent behavior or decision. Implementation and tests of
the same behavior can share context. Keep item mappings in the plan; a combined
requirement dictionary does not replace them. The result includes an output
schema, `structuredContent`, and compatible JSON text. Partial failures set
`isError` while preserving successful items. `no_match` means unknown relevance;
coverage means linked/proposed tests, not passed runs. Unknown branches fail.
The server requires the batch retrieval API; this tool never falls back to a
less precise lookup and never creates or advances an intent session.
