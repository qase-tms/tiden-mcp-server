# Changelog

All notable changes to `tiden-mcp-server` are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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

[0.3.0]: https://github.com/qase-tms/tiden-mcp-server/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/qase-tms/tiden-mcp-server/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/qase-tms/tiden-mcp-server/releases/tag/v0.1.0
