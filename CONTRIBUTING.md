# Contributing

Thanks for contributing to the Tiden MCP server.

Before opening a pull request:

- Run `gofmt -w .`.
- Run `go vet ./...`.
- Run `go test ./...`.
- Keep stdout reserved for the MCP protocol when changing server runtime code.

The API contract mirrors the Tiden public REST API. Keep request and response models hand-written and limited to the MCP tool surface.

Changelog: record each user-visible change as `docs/changelog/YYYY-MM-DD-<slug>.md` per [`docs/changelog/README.md`](docs/changelog/README.md). Do not edit `CHANGELOG.md` in a feature PR, not even `## [Unreleased]`: only a `release: X.Y.Z` PR changes the file, through `make changelog-cut RELEASE=X.Y.Z`. The file's grammar, which the release automation parses, is pinned in [`docs/changelog-spec.md`](docs/changelog-spec.md).
