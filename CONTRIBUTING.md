# Contributing

Thanks for contributing to the Tiden MCP server.

Before opening a pull request:

- Run `gofmt -w .`.
- Run `go vet ./...`.
- Run `go test ./...`.
- Keep stdout reserved for the MCP protocol when changing server runtime code.

The API contract mirrors the Tiden public REST API. Keep request and response models hand-written and limited to the MCP tool surface.
