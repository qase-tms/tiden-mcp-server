# Changelog

All notable changes to `tiden-mcp-server` are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- `merge_test_cases` folds one test case into another when both are the same
  test, for example a renamed test that was reported as a new case. Pass
  `product_id`, `ref` (the case to merge away) and `into_ref` (the survivor),
  each a case number or UUID, and an optional `reason`. It returns `mergeId`,
  `survivorSeq`, `mergedSeq` and `queued`. The merge skips the server's own
  equivalence check; a refused merge is a tool error that shows the server's
  reason.
- `unmerge_test_cases` undoes a merge by `merge_id`. Only the newest merge in
  the chain that touches either case can be undone; otherwise the error names
  the newer merge to undo first. It returns `restoredSeq`,
  `ambiguousAfterMerge` and `blocked`.
- `list_test_merges` lists a product's test case merges, newest first, with an
  optional `test_ref` to see only the merges that touch one case.
- `get_test` returns a case's `repository`, `keys` (every key the case is known
  by), `mergedIntoId` and `redirectedFrom`.

### Changed

- The `report_test_results` description tells agents that a result may carry
  `identity` (language, repository, scope, name) next to its signature, and that
  a test reported as a second case is folded back with `merge_test_cases`. The
  tool passes the object to the server unchanged; an older server that does not
  know the field rejects it.

## [0.10.0] - 2026-10-23

### Added

- `set_issue_status` accepts an optional `reason` (max 1000 characters), for
  example the fix PR or commit. The reason belongs to the new status: a status
  change without a reason clears the previous one. Issues return it as
  `statusReason`. It needs a Tiden server that stores the reason for an
  issue's status.

## [0.9.0] - 2026-10-21

### Added

- `hold_intent` tool: holds or releases the automatic merge of an intent
  branch. Pass `branch_id` and `reason`, with either `for` (`3d`, `12h`;
  default `7d`) or `until` (an RFC 3339 timestamp), or pass `clear=true` to
  release the current hold. The tool refuses a call without `reason` before it
  sends a request, and shows the server's message when the server refuses the
  hold, for example for a branch that is already merged or a hold longer than
  7 days (the longest hold an agent may set; a person can hold an intent for up
  to 30 days). It returns the
  branch's `autoMerge` state. A hold on a branch Tiden closed as abandoned
  opens that branch again. It needs a Tiden server that supports automatic
  intent merge.

### Changed

- `list_branches` and `get_merge_preview` return `autoMerge`, where the
  automatic merge of an intent branch stands: `state`, `reason`, `nextCheckAt`,
  the latest hold (`holdUntil`, `holdReason`, `heldByName`, `holdSource`), and,
  for a partly delivered intent, `openPromises` and `partialUntil`. A merged
  branch also returns `mergeMode`, `mergedByName` and `continuesBranchId`. A
  server without these fields returns the same output as before.

## [0.8.0] - 2026-10-20

### Changed

- **Behavior change:** `list_specs` now returns one page instead of the full
  list: 25 full documents by default (identity view: 100 rows), at most 200.
  Continue with `page_token`, and keep `page_size` the same on every call of one
  traversal, because the server's tokens are page offsets. Pass `all=true` for
  the former full traversal (up to 100 pages of 200; a remaining
  `nextPageToken` means the result is incomplete). `page_size` and `page_token`
  now also work without `view=identity`; `all` cannot be combined with them or
  with `view=identity`. For existence checks and indexes use `view=identity` or
  `lookup_context`.
- **Behavior change:** `get_traceability` output is sized for an agent's
  context.
  - With neither `feature_id` nor `component_id` it returns a per-component
    rollup (`view: "rollup"`): component id, name and status, the number of
    specs per coverage state, and the display reference, id and coverage of the
    specs that are not verified (up to 50 per component, then
    `notVerifiedMore`).
  - `feature_id` (a spec UUID; the spec and its descendants) or the new
    `component_id` returns one summary row per spec (`view: "summary"`), with
    `linked`, `passed`, `failed` and `stale` counts when the server provides
    them.
  - `include_tests=true` returns the detail view with each linked test, and now
    requires `feature_id` or `component_id`; a whole-product detail matrix no
    longer fits the output limit.
  - `feature_id` and `component_id` must be UUIDs. A display reference such as
    `QA-12` is rejected with a pointer to `get_spec` / `list_specs`.
  - A server that does not apply the filter (the current hosted API ignores
    `feature_id`) returns the whole product. The tool then filters the rows
    itself and sets `featureFilter` or `componentFilter` to `"client"`;
    component status is then component-wide, not computed over the filtered
    rows only. A server that does not support the summary view answers with the
    full matrix, which the tool reduces to the same summary shape.
- `session_progress` asks the server for compact tests: each spec lists up to 20
  failing tests, then up to 5 others, plus a `testCount` of all linked tests.
  The new `all_tests=true` lists every linked test. Servers without compact
  tests return every test. The server response is now passed through unchanged,
  so fields such as `canonicalId`, `stale`, `intentBranchStatus`,
  `freshnessApplied` and the test `signature` reach the agent.
- `get_spec` accepts `branch` and returns that branch's view of the spec.

## [0.7.0] - 2026-10-13

### Changed

- **Breaking:** Tiden's product record is now called a *spec* (formerly a
  *requirement*), and the tools and arguments agents see say so. Tools:
  `list_requirements` → `list_specs`, `get_requirement` → `get_spec`,
  `create_requirement` → `create_spec`, `update_requirement` → `update_spec`,
  `link_requirement` → `link_spec`. Arguments: `link_spec`'s `requirement_id`
  → `spec_id`; `session_progress`'s `requirement_ids` → `spec_ids`;
  `record_risk_acceptances`' `requirement_id` → `spec_id`,
  `acceptances[].requirement_refs` → `spec_refs` and
  `proposed_test_requirement_refs` → `proposed_test_spec_refs`;
  `lookup_context`'s `max_requirements` → `max_specs`. Which arguments are
  required is unchanged. The old names are not kept as aliases, since two tools
  for one action confuse agents: agents that discover tools at connect time
  pick up the new names on their own, but permission allowlists naming a tool
  (e.g. `mcp__tiden__list_requirements`) and scripts or prompts that call one
  by name must be updated, and a call that passes an old argument name is
  refused as an unknown argument. Tool descriptions, `capture_intent`'s summary
  line and `list_specs` validation errors say "spec" too. JSON results are
  unchanged: they mirror the Tiden public API, where a spec is still the
  `requirement` resource (`requirements`, `requirementId`).

## [0.6.0] - 2026-09-25

### Removed

- The `levels` argument of `list_issues`. tiden-app no longer filters issues
  by level (production issues are error-only) and rejects a `levels` filter.
  A call that still passes `levels` is refused as an unknown argument before
  any request is sent; drop the argument. Each issue's `level` field is still
  returned.

## [0.5.0] - 2026-09-24

### Changed

- The server resolves its workspace **per repository** instead of from one
  flat file: it reads the `tiden` CLI's workspace-keyed `~/.tiden/config.json`
  (still falling back to the older single-login shape), then the repo-local
  `.tiden/config.json` a `tiden setup`/`tiden workspace use`/`tiden product
  bind` run left behind, then a product or repository lookup against the
  server, in the same order the CLI itself resolves. It is read-only — no
  prompts, no file writes, no migration — and prints the chosen workspace,
  account and resolution source to stderr on a successful start. A repository
  that cannot be resolved (bound to a workspace none of your accounts
  belongs to, or visible to more than one account with no binding yet) now
  fails loudly with the specific reason and a `tiden` command to fix it,
  exit code 2, instead of the previous generic "missing required config"
  message a v2 config file always triggered. Upgrade `tiden` and
  `tiden-mcp-server` together — an older binary cannot read the v2 file.

## [0.4.0] - 2026-09-08

### Added

- `lookup_context` retrieves up to eight independent plan questions through the
  canonical batch API, with repository-qualified anchors, per-item mappings,
  explicit partial errors, structured content and a read-only tool contract.

## [0.3.2] - 2026-09-08

### Changed

- `list_tests` accepts `view=identity` for one bounded read-only page of tags,
  signatures and hierarchy identities, without bodies, steps or counts. The
  default remains full detail. Identity cannot combine with `all`; explicit
  continuation, invalid/foreign field checks and old-server errors prevent
  incomplete data from appearing as writable full tests.

- `get_run_results` accepts `summary_view=overview|cases|combos` for suite
  statistics or one bounded case/combination page (default 100, maximum 200).
  Combination pages require `identity_key`; continuations remain explicit.
  Existing flat and full-summary defaults are unchanged. Unsupported old
  servers and malformed pages fail instead of silently returning a full tree.

- `list_requirements` accepts `view=identity` for one page of read-only IDs,
  titles, content hashes and source locators without bodies or source metadata.
  `page_size` defaults to 100 (maximum 200); `page_token` explicitly continues.
  Omitted/default or `view=detail` preserves the full list. Invalid projections,
  missing hashes and unsupported old servers return errors rather than partial
  writable requirement objects.

- `list_tests` fetches one page of 100 tests by default, with `page_size` (1–200)
  and `page_token` for explicit continuation. Agents that need the previous
  catalog traversal must pass `all=true`; this fetches at most 100 pages of 200
  tests and preserves any remaining `pagination.nextPageToken`. Total count,
  response fields and concrete test IDs are unchanged. Full traversal cannot
  be combined with paging arguments.

## [0.3.1] - 2026-09-04

### Fixed

- `gate_check` and `get_verdict` no longer drop the fields that say whose work a
  requirement is and what the verdict left out. The tool answer is decoded into
  this repo's model and re-encoded from it, so a field absent from the model is
  silently discarded between the server and the agent: the per-provenance counts,
  the sentences naming what was excluded from the grading, the divergence flag,
  and each requirement's `touched` provenance and `uncovered_on_main` baseline
  all vanished on the way through. An agent that cannot tell its own work from
  adjacent debt reads the whole verdict as somebody else's problem, which is the
  failure the branch scope work exists to remove — reappearing one layer down.
  The two new scope absences (`unmeasured`, `retrieval_only`) also arrive intact,
  and a round-trip test refuses any future field that does not survive.

## [0.3.0] - 2026-09-03

### Changed

- `gate_check` and `get_verdict` mirror the branch Quality Gate's answer: the
  verdict's `next_action`, its `annotation`, the `accepted_risks` a session
  priced, the `scope_info` saying how a branch's touched scope was derived, and
  each subject's touched requirements and non-passing tests. The tool
  descriptions now name all four statuses (`pass` / `blocked` / `not_verified` /
  `risk_accepted`) and tell the agent to act on the next action rather than on
  the status alone — a branch verdict is `not_verified` far more often than
  `blocked`, and a description that omitted it taught agents to read it as noise.

### Added

- Executable changelog contract tests (`internal/changelogcontract`): a reference
  parser, ported from tiden-cli, that falsifies this repo's `CHANGELOG.md` against
  the grammar in `docs/changelog-spec.md` — pending-version derivation and
  version-section extraction, with the real `CHANGELOG.md` checked as a subcase.

## [0.2.0] - 2026-08-24

### Added

- Issue tools: `list_issues`, `get_issue`, `list_issue_events`,
  `get_issue_event`, `get_issue_event_stats`, `get_issue_fix_context`,
  `list_release_issues`, `set_issue_status` (#6).
- `session_progress` tool: per-requirement progress of an intent session —
  coverage ladder, session attribution, readiness, and next actions; the
  gate tools (`gate_check`, `get_verdict`, `get_traceability`) can now scope
  to a branch (#8).
- `record_risk_acceptances` tool: record an intent session's risk
  acceptances and test deferrals, exposing the full risk-acceptance rubric
  (#11).

### Changed

- `record_risk_acceptances` no longer requires `requirement_id` (#11).

### Fixed

- Removed references to other repositories' PR numbers from public code
  comments (#12).

## [0.1.0] - 2026-07-23

### Added

- Initial release: a stdio MCP server exposing Tiden's public REST API as
  tools — workspaces, products, requirements, tests, branches, merge
  previews, components, environments, releases, and Quality Gate
  verdicts/traceability.
- `get_product` tool: fetch a product by id (#1).
- Seven test-run tools: `create_test_run`, `list_test_runs`, `get_test_run`,
  `get_run_results`, `report_test_results`, `complete_test_run`,
  `abort_test_run` (#2).
- `capture_intent` tool: distill a session's product decisions into a
  reviewable intent branch (#5).

### Fixed

- `list_components` exposes each component's repository scope; optional
  tool arguments across all tools are now schema-optional, with caps and
  clamps applied to list results (#3).
- `list_tests` and `list_requirements` no longer truncate — results are now
  fetched across all pages (#4).

[0.10.0]: https://github.com/qase-tms/tiden-mcp-server/compare/v0.9.0...v0.10.0
[0.9.0]: https://github.com/qase-tms/tiden-mcp-server/compare/v0.8.0...v0.9.0
[0.8.0]: https://github.com/qase-tms/tiden-mcp-server/compare/v0.7.0...v0.8.0
[0.7.0]: https://github.com/qase-tms/tiden-mcp-server/compare/v0.6.0...v0.7.0
[0.6.0]: https://github.com/qase-tms/tiden-mcp-server/compare/v0.5.0...v0.6.0
[0.5.0]: https://github.com/qase-tms/tiden-mcp-server/compare/v0.4.0...v0.5.0
[0.4.0]: https://github.com/qase-tms/tiden-mcp-server/compare/v0.3.2...v0.4.0
[0.3.2]: https://github.com/qase-tms/tiden-mcp-server/compare/v0.3.1...v0.3.2
[0.3.1]: https://github.com/qase-tms/tiden-mcp-server/compare/v0.3.0...v0.3.1
[0.3.0]: https://github.com/qase-tms/tiden-mcp-server/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/qase-tms/tiden-mcp-server/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/qase-tms/tiden-mcp-server/releases/tag/v0.1.0
