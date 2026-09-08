package mcpserver

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/qase-tms/tiden-mcp-server/internal/api"
	"github.com/qase-tms/tiden-mcp-server/internal/model"
)

type lookupContextArgs struct {
	ProductID       string             `json:"product_id" jsonschema:"Product UUID; all items use this product."`
	Branch          string             `json:"branch,omitempty" jsonschema:"Tiden branch name; omit for main. Unknown branch fails without creating it."`
	Items           []model.LookupItem `json:"items" jsonschema:"1 to 8 independent questions with stable unique IDs. Each has up to 16 repository-qualified anchors."`
	MaxRequirements int                `json:"max_requirements,omitempty" jsonschema:"Displayed requirements per item: default 12, maximum 40. Candidate IDs survive presentation truncation."`
}

func registerLookupContext(srv *mcp.Server, client *api.Client) {
	destructive := false
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "lookup_context",
		Description: `Read existing requirement context for each independent part of a plan, before choosing implementation details. Decompose by behavior or decision; code and tests for the same behavior can reuse one item. Example items: [{"id":"P1","question":"How are retrieval seeds ranked?","anchors":[{"repository":"github.com/org/app","path":"src/retrieval.go"}]},{"id":"P2","question":"How are run summaries calculated?"}]. Keep each item's mappings, constraints and unknowns in the plan; the deduplicated dictionary is a reference. no_match means relevance is unknown, error/partial means retrieval failed, truncated means a limit was reached. Coverage describes linked/proposed tests, not passed runs. Follow up only on unresolved items; get_requirement retrieves a full description. Creates no session, branch, draft or commitment; several lookups can inform one implementation session. At most 8 items per call; do not fan out concurrent batches.`,
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, DestructiveHint: &destructive, IdempotentHint: true},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args lookupContextArgs) (*mcp.CallToolResult, *model.LookupBatchResult, error) {
		if args.ProductID == "" {
			return nil, nil, errMissingField("product_id")
		}
		plan := model.LookupPlan{Version: 1, Items: args.Items}
		if err := model.ValidateLookupPlan(plan, 8); err != nil {
			return nil, nil, err
		}
		if args.MaxRequirements < 0 || args.MaxRequirements > 40 {
			return nil, nil, fmt.Errorf("max_requirements must be between 0 and 40")
		}
		result, err := client.ResolveFeatureContextBatch(ctx, args.ProductID, args.Branch, plan, args.MaxRequirements)
		if err != nil {
			return nil, nil, err
		}
		// Typed output asks the SDK to publish/validate OutputSchema and emit both
		// structuredContent and the compatible JSON TextContent, even on partials.
		return &mcp.CallToolResult{IsError: result.RetrievalStatus != "complete"}, result, nil
	})
}
