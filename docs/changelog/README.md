# docs/changelog: pending changelog entries

Each change is recorded as its own file here, not in `CHANGELOG.md`. Separate
files keep parallel PRs from conflicting in one shared section. The release cut
folds every entry into the new version's section of `CHANGELOG.md` and deletes
the files, so this directory holds only what has not been released yet.

`CHANGELOG.md` is read by the release automation (`/operator release`) and keeps
the grammar in [`../changelog-spec.md`](../changelog-spec.md). A feature PR does
not edit it, not even `## [Unreleased]`.

## Entry grammar

1. Filename: `YYYY-MM-DD-<kebab-slug>.md`, the authoring date.
2. Line 1: `# Changelog — <Title>` (em dash). The title appears in the release
   PR's summary; it is not copied into `CHANGELOG.md`.
3. Line 2 blank; line 3 `**Date:** YYYY-MM-DD` (the filename's date); line 4
   `**Section:** Added`, `Changed`, `Deprecated`, `Removed`, `Fixed` or
   `Security`; line 5 blank.
4. The rest is the body: the `CHANGELOG.md` bullet, written as it should read
   there, without the leading `- `. Several paragraphs and nested lists are
   fine; a line starting with `#` is not.

One entry is one bullet. A PR with changes in two sections adds two entries.
`go test ./internal/changelogcut/` checks every entry here against this grammar.

```md
# Changelog — list_specs returns the last page

**Date:** 2026-11-07
**Section:** Fixed

`list_specs` no longer drops the last page when the total is an exact
multiple of the page size. (AICP-123)
```

## When to skip

Skip the entry only for a change no user of the server can notice: a typo,
lint, CI or test-only work, a dependency bump with no behavior change.

## Cutting a release

1. On a branch from an up-to-date `main`: `make changelog-cut RELEASE=X.Y.Z`.
   It writes the version section and its link line, deletes the folded entries,
   and prints a summary. Bullets that a PR still added under `## [Unreleased]`
   go into the same section, so nothing merged is dropped.
2. `go test ./...`, then open a PR titled `release: X.Y.Z` with the summary in
   its body.
3. After it merges, `/operator release tiden-mcp` tags `vX.Y.Z`.
