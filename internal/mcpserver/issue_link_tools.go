package mcpserver

import (
	"context"
	"fmt"
	"regexp"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/qase-tms/tiden-mcp-server/internal/api"
)

// Issue-to-spec link tools. A link records which spec an error violates. The
// server stores it as a relation, so no issue id belongs in spec text.

const maxIssueLinkPairs = 200

var uuidRe = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

type issueSpecLink struct {
	IssueID string `json:"issue_id" jsonschema:"Issue UUID (required)."`
	SpecRef string `json:"spec_ref" jsonschema:"Spec ref (required): KEY-N such as AUTH-12, a bare spec number, or a spec UUID."`
}

type issueSpecLinksArgs struct {
	ProductID string          `json:"product_id"       jsonschema:"Product ID (required)."`
	Links     []issueSpecLink `json:"links"            jsonschema:"Pairs to change (required), 1-200 entries. All pairs succeed or none is written."`
	Branch    string          `json:"branch,omitempty" jsonschema:"Branch the spec refs are read on. Omit for main."`
}

// resolveSpecRefs maps spec refs to spec ids with one resolve call. UUIDs pass
// through. Any ref that is unresolved or ambiguous is an error, so nothing is
// written for a partly resolved batch.
func resolveSpecRefs(ctx context.Context, client *api.Client, productID, branch string, refs []string) (map[string]string, error) {
	ids := make(map[string]string, len(refs))
	var toResolve []string
	for _, ref := range refs {
		if _, done := ids[ref]; done {
			continue
		}
		if uuidRe.MatchString(ref) {
			ids[ref] = ref
			continue
		}
		ids[ref] = ""
		toResolve = append(toResolve, ref)
	}
	if len(toResolve) == 0 {
		return ids, nil
	}
	resp, err := client.ResolveSpecs(ctx, productID, branch, toResolve)
	if err != nil {
		return nil, err
	}
	byRef := make(map[string]api.ResolvedSpec, len(resp.Requirements))
	for _, r := range resp.Requirements {
		byRef[r.Ref] = r
	}
	var bad []string
	for _, ref := range toResolve {
		r, ok := byRef[ref]
		// The row the branch shows: the server maps it to the canonical id
		// itself and checks that this row is live.
		id := r.ViewID
		if id == "" {
			id = r.CanonicalID
		}
		if !ok || r.Status != "ok" || id == "" {
			status := "unresolved"
			if ok && r.Status != "" {
				status = r.Status
			}
			bad = append(bad, fmt.Sprintf("%s (%s)", ref, status))
			continue
		}
		ids[ref] = id
	}
	if len(bad) > 0 {
		return nil, fmt.Errorf("spec refs not resolved: %v", bad)
	}
	return ids, nil
}

func buildIssueSpecPairs(ctx context.Context, client *api.Client, args issueSpecLinksArgs) ([]api.IssueSpecPair, error) {
	if args.ProductID == "" {
		return nil, errMissingField("product_id")
	}
	if len(args.Links) == 0 {
		return nil, fmt.Errorf("links must hold at least one {issue_id, spec_ref} pair")
	}
	if len(args.Links) > maxIssueLinkPairs {
		return nil, fmt.Errorf("links holds %d pairs; the limit is %d per call", len(args.Links), maxIssueLinkPairs)
	}
	refs := make([]string, 0, len(args.Links))
	for i, l := range args.Links {
		if l.IssueID == "" {
			return nil, fmt.Errorf("links[%d].issue_id is required", i)
		}
		if l.SpecRef == "" {
			return nil, fmt.Errorf("links[%d].spec_ref is required", i)
		}
		refs = append(refs, l.SpecRef)
	}
	ids, err := resolveSpecRefs(ctx, client, args.ProductID, args.Branch, refs)
	if err != nil {
		return nil, err
	}
	pairs := make([]api.IssueSpecPair, len(args.Links))
	for i, l := range args.Links {
		pairs[i] = api.IssueSpecPair{IssueID: l.IssueID, RequirementID: ids[l.SpecRef]}
	}
	return pairs, nil
}

// -- link_issue_specs ----------------------------------------------------------

func registerLinkIssueSpecs(srv *mcp.Server, client *api.Client) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "link_issue_specs",
		Description: "Record which specs an issue violates. Use this instead of writing issue ids into spec text. Each link pairs an issue_id with a spec_ref (KEY-N, a bare number, or a spec UUID); refs are resolved on branch (main when omitted) in one call, then all links are written in one all-or-nothing call. Returns one result per pair: linked or already_linked.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args issueSpecLinksArgs) (*mcp.CallToolResult, any, error) {
		pairs, err := buildIssueSpecPairs(ctx, client, args)
		if err != nil {
			return toolError(err)
		}
		resp, err := client.UpdateIssueSpecLinks(ctx, args.ProductID, pairs, nil)
		if err != nil {
			return toolError(err)
		}
		return toolResult(resp)
	})
}

// -- unlink_issue_specs --------------------------------------------------------

func registerUnlinkIssueSpecs(srv *mcp.Server, client *api.Client) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "unlink_issue_specs",
		Description: "Remove issue-to-spec links recorded with link_issue_specs, for example a link made to the wrong spec. Each entry pairs an issue_id with a spec_ref (KEY-N, a bare number, or a spec UUID); refs are resolved on branch (main when omitted). All removals succeed or none is applied. Returns one result per pair: unlinked or not_linked.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args issueSpecLinksArgs) (*mcp.CallToolResult, any, error) {
		pairs, err := buildIssueSpecPairs(ctx, client, args)
		if err != nil {
			return toolError(err)
		}
		resp, err := client.UpdateIssueSpecLinks(ctx, args.ProductID, nil, pairs)
		if err != nil {
			return toolError(err)
		}
		return toolResult(resp)
	})
}

// -- list_spec_issues ----------------------------------------------------------

type listSpecIssuesArgs struct {
	ProductID string `json:"product_id"       jsonschema:"Product ID (required)."`
	SpecRef   string `json:"spec_ref"         jsonschema:"Spec ref (required): KEY-N such as AUTH-12, a bare spec number, or a spec UUID."`
	Branch    string `json:"branch,omitempty" jsonschema:"Branch the spec ref is read on. Omit for main."`
	Limit     int    `json:"limit,omitempty"  jsonschema:"Issues to return, 1-100 (default 50)."`
}

func registerListSpecIssues(srv *mcp.Server, client *api.Client) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_spec_issues",
		Description: "List the issues linked to one spec, unresolved first and then most recently seen, with the total count of linked issues. Use it to see which errors a spec is known to be violated by. spec_ref is KEY-N, a bare number, or a spec UUID, read on branch (main when omitted).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args listSpecIssuesArgs) (*mcp.CallToolResult, any, error) {
		if args.ProductID == "" {
			return toolError(errMissingField("product_id"))
		}
		if args.SpecRef == "" {
			return toolError(errMissingField("spec_ref"))
		}
		if args.Limit < 0 || args.Limit > 100 {
			return toolError(fmt.Errorf("limit must be between 1 and 100"))
		}
		ids, err := resolveSpecRefs(ctx, client, args.ProductID, args.Branch, []string{args.SpecRef})
		if err != nil {
			return toolError(err)
		}
		resp, err := client.ListSpecIssues(ctx, ids[args.SpecRef], args.Limit)
		if err != nil {
			return toolError(err)
		}
		return toolResult(resp)
	})
}
