# Shared config fixtures

These fixtures are the golden source for parsing `~/.tiden/config.json`
(v1 flat and v2 workspace-keyed shapes). They are **copied verbatim** into
`tiden-mcp-server/internal/config/testdata/shared` (see TIDEN-68 decision D2)
so that the CLI's and the MCP server's config loaders parse the same bytes
into the same shape — `diff -r` between the two directories must stay empty.

- `v1.json` — flat v1 config with a bound workspace.
- `v1-no-workspace.json` — flat v1 config with a token but no `workspaceId`
  (a "loose" login: a token not placed in any workspace entry).
- `v2.json` — v2 config with three workspace entries: two share one
  `(baseUrl, apiToken)` login under one account, the third belongs to a
  second account.
- `v2-with-extra-keys.json` — v2 config that also carries `intentCapture`
  and `skillsBaseUrl`, which a v2-aware loader must preserve verbatim on
  save without understanding their shape.
- `mixed-v1-v2.json` — a file carrying both the legacy flat keys
  (`baseUrl`/`apiToken`/`workspaceId`) and the `workspaces` map. The v2 shape
  wins; the flat keys are dropped on save.
- `expected.json` — the parsed result each fixture must produce, keyed by
  fixture name (without extension): `fileVersion`, `entries` (by workspace
  id, each with `baseUrl`/`account`/`name`/`apiToken`), `loose` (logins with
  no workspace), and `extraKeys` (the top-level keys, beyond
  `version`/`timeout`/`workspaces`/the legacy flat keys, that must survive a
  save byte-for-byte).

Any change here must be applied to both copies in the same change, and both
parsers' tests must be run to confirm they still agree.
