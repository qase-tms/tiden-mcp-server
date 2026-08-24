# Changelog specification

This repository's `CHANGELOG.md` is machine-read by the release automation
(`/operator release` in Slack). The grammar below is normative: a file that
deviates from it breaks version derivation or renders wrong release notes.

## File grammar

1. The file starts with a `# Changelog` title and an intro paragraph.
2. A `## [Unreleased]` heading is always present and is the first version
   heading in the file.
3. Every released or pending version has a heading of exactly:

   ```
   ## [X.Y.Z] - YYYY-MM-DD
   ```

   - `X.Y.Z` is semver, without a `v` prefix.
   - One space on each side of the hyphen; the date is the day the section
     was cut.
4. A version's section body is everything from its heading down to the next
   `## ` heading (exclusive). Subsections are limited to `### Added`,
   `### Changed`, `### Fixed`, `### Deprecated`, `### Removed`,
   `### Security`.
5. The bottom of the file holds the link-reference block, newest first:

   ```
   [X.Y.Z]: https://github.com/qase-tms/tiden-mcp-server/compare/vA.B.C...vX.Y.Z
   ```

   The oldest version links to `.../releases/tag/vX.Y.Z` instead of a
   compare URL.

## Release-flow invariant

The git tag for a release is `v` + the heading version (`vX.Y.Z`), and the
version section is cut in a release PR **before** the tag is pushed. Only
the newest version heading — the first below `## [Unreleased]` — can be
pending: if its `vX.Y.Z` tag does not exist yet, that is *the* pending
release the automation will tag; if its tag exists, there is nothing to
release. Older headings are history. A version that was never tagged on its
own (its changes shipped inside a later release) stays in the file as
history; note that in plain text inside its section body — never as extra
text on the heading line.

## How the automation reads this file

- Pending version = the newest version heading's `X.Y.Z`, and only if its
  `vX.Y.Z` git tag does not exist. Older untagged headings are never
  considered pending.
- Release notes for a version = its section body, truncated to Slack message
  limits with a link to the full file.
