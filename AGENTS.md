# AGENTS.md

Guidance for AI coding agents working in this repository. Conventions: [CONTRIBUTING.md](CONTRIBUTING.md); build and test commands: [README.md](README.md).

## Changelog

Changelog: record each user-visible change as `docs/changelog/YYYY-MM-DD-<slug>.md` per [`docs/changelog/README.md`](docs/changelog/README.md). Do not edit `CHANGELOG.md` in a feature PR, not even `## [Unreleased]`: only a `release: X.Y.Z` PR changes the file, through `make changelog-cut RELEASE=X.Y.Z`. The file's grammar, which the release automation parses, is pinned in [`docs/changelog-spec.md`](docs/changelog-spec.md).
