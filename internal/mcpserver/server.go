// Package mcpserver implements a stdio MCP server that exposes the Tiden
// platform's public API as MCP tools. It is a thin adapter over internal/api -
// all domain logic lives there; this package only translates between the MCP
// calling convention and the existing client methods.
package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/qase-tms/tiden-mcp-server/internal/api"
	"github.com/qase-tms/tiden-mcp-server/internal/version"
)

// New builds and returns an MCP server with all Tiden tools registered.
// client must be fully initialised (base URL + API token set).
// defaultWorkspaceID is used when a tool's workspace_id input is omitted.
func New(client *api.Client, defaultWorkspaceID string) *mcp.Server {
	srv := mcp.NewServer(&mcp.Implementation{
		Name:    "tiden",
		Version: version.Get(),
	}, nil)

	registerTools(srv, client, defaultWorkspaceID)
	return srv
}

// Run connects the server to the stdio transport and blocks until the client
// disconnects. All log output must go to stderr - stdout is the protocol wire.
func Run(ctx context.Context, srv *mcp.Server) error {
	return srv.Run(ctx, &mcp.StdioTransport{})
}

// toolResult wraps any JSON-serialisable value into a single MCP TextContent
// block. This is the standard output format for all Tiden tools.
func toolResult(v any) (*mcp.CallToolResult, any, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal response: %w", err)
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: string(b)},
		},
	}, nil, nil
}

// toolText wraps a plain human-readable string as a single MCP TextContent
// block. Unlike toolResult it does not JSON-encode the value - use it for
// tools whose output is a summary sentence rather than an API response shape.
func toolText(s string) (*mcp.CallToolResult, any, error) {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: s},
		},
	}, nil, nil
}

// toolError converts an API error into an MCP tool error result.
// The server message is forwarded verbatim so the LLM has full context.
func toolError(err error) (*mcp.CallToolResult, any, error) {
	return &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{
			&mcp.TextContent{Text: err.Error()},
		},
	}, nil, nil
}

// ptr returns a pointer to s, or nil when s is the zero value.
// Used to convert optional string inputs to *string API params.
func ptr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
