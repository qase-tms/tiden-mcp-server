package mcpserver

import (
	"context"
	"fmt"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/qase-tms/tiden-mcp-server/internal/api"
)

// distillTimeout gives the server-side LLM distill room to run: the shared
// client's default 30s cap would fire client-side while the server keeps
// writing the branch. See Client.WithTimeout.
const distillTimeout = 180 * time.Second

// -- capture_intent ------------------------------------------------------------

type captureIntentArgs struct {
	ProductID    string   `json:"product_id"             jsonschema:"Product ID to distill into"`
	Transcript   string   `json:"transcript"             jsonschema:"The session's product-relevant decisions rendered as 'USER: ...' / 'ASSISTANT: ...' lines (or a plain decision summary)"`
	Slug         string   `json:"slug,omitempty"         jsonschema:"Optional branch slug (defaults to 'session')"`
	SessionID    string   `json:"session_id,omitempty"   jsonschema:"Session id for provenance"`
	Agent        string   `json:"agent,omitempty"        jsonschema:"Coding agent name for provenance"`
	ChangedFiles []string `json:"changed_files,omitempty" jsonschema:"Repo-relative files changed this session (enables file anchors)"`
}

func registerCaptureIntent(srv *mcp.Server, client *api.Client) {
	mcp.AddTool(srv, &mcp.Tool{
		Name: "capture_intent",
		Description: "Distill this session's product decisions into a reviewable Tiden intent branch. " +
			"Call at the end of a working session with a summary of user decisions. " +
			"May take a few minutes. If the call times out, the branch may still have been written — " +
			"retry with the same slug; the server reuses the same-day intent branch, so a retry converges instead of duplicating. " +
			"When session_id is passed, the server also records a machine-readable settlement on that session's record, which is what makes this distillation visible to the intent loop's close gate and analytics — always pass session_id when one exists.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args captureIntentArgs) (*mcp.CallToolResult, any, error) {
		if args.ProductID == "" {
			return toolError(errMissingField("product_id"))
		}
		if args.Transcript == "" {
			return toolError(errMissingField("transcript"))
		}
		resp, err := client.WithTimeout(distillTimeout).DistillIntent(ctx, args.ProductID, api.DistillIntentRequest{
			Transcript:   args.Transcript,
			Slug:         args.Slug,
			SessionID:    args.SessionID,
			Agent:        args.Agent,
			ChangedFiles: args.ChangedFiles,
		})
		if err != nil {
			return toolError(err)
		}
		if resp.Skipped {
			return toolText(fmt.Sprintf("Skipped: %s", resp.SkipReason))
		}
		return toolText(fmt.Sprintf(
			"Created %d + updated %d requirement(s) on branch %s — review with merge-preview.",
			resp.Created, resp.Updated, resp.IntentBranch,
		))
	})
}
