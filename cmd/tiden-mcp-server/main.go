package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/qase-tms/tiden-mcp-server/internal/api"
	"github.com/qase-tms/tiden-mcp-server/internal/config"
	"github.com/qase-tms/tiden-mcp-server/internal/gitremote"
	"github.com/qase-tms/tiden-mcp-server/internal/mcpserver"
	"github.com/qase-tms/tiden-mcp-server/internal/repoconfig"
	"github.com/qase-tms/tiden-mcp-server/internal/resolve"
	"github.com/qase-tms/tiden-mcp-server/internal/version"
)

func main() {
	flag.CommandLine.SetOutput(os.Stderr)

	var (
		flagBaseURL     string
		flagAPIToken    string
		flagWorkspaceID string
		flagTimeout     string
		flagVersion     bool
	)

	flag.StringVar(&flagBaseURL, "base-url", "", "Tiden API base URL")
	flag.StringVar(&flagAPIToken, "api-token", "", "API token")
	flag.StringVar(&flagWorkspaceID, "workspace-id", "", "Workspace ID")
	flag.StringVar(&flagTimeout, "timeout", "", "Per-request API timeout as a Go duration (default 30s)")
	flag.BoolVar(&flagVersion, "version", false, "Print version and exit")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [flags]\n\n", os.Args[0])
		fmt.Fprintln(os.Stderr, "Start the Tiden stdio MCP server. Stdout is the MCP protocol wire; diagnostics go to stderr.")
		fmt.Fprintln(os.Stderr)
		flag.PrintDefaults()
	}
	flag.Parse()

	if flagVersion {
		fmt.Println(version.Get())
		return
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	if err := run(ctx, flagBaseURL, flagAPIToken, flagWorkspaceID, flagTimeout); err != nil {
		if errors.Is(err, context.Canceled) {
			fmt.Fprintln(os.Stderr, "interrupted")
			os.Exit(130)
		}
		var rerr *resolve.Error
		if errors.As(err, &rerr) {
			printResolutionError(os.Stderr, rerr)
			os.Exit(2)
		}
		if errors.Is(err, api.ErrUnauthorized) {
			os.Exit(2)
		}
		fmt.Fprintf(os.Stderr, "error: %s\n", err)
		os.Exit(1)
	}
}

// printResolutionError writes an unresolved-ladder error's message and each
// hint on its own line to stderr - never the old "missing required config"
// text, and never a silent empty token.
func printResolutionError(w *os.File, rerr *resolve.Error) {
	fmt.Fprintln(w, rerr.Message)
	for _, hint := range rerr.Hints {
		fmt.Fprintln(w, "  "+hint)
	}
}

func run(ctx context.Context, flagBaseURL, flagAPIToken, flagWorkspaceID, flagTimeout string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("resolve working directory: %w", err)
	}

	resolved, timeout, err := resolveConfig(ctx, cwd, flagBaseURL, flagAPIToken, flagWorkspaceID, flagTimeout)
	if err != nil {
		return err
	}

	printDiagnostic(os.Stderr, resolved)

	client := api.NewWithTimeout(resolved.BaseURL, resolved.APIToken, timeout)
	srv := mcpserver.New(client, resolved.WorkspaceID)
	return mcpserver.Run(ctx, srv)
}

// resolveConfig loads the global config store and the repo-local binding
// (walking up from cwd), determines the repository's canonical identity via
// its origin remote, and runs the read-only resolution ladder
// (internal/resolve). It never writes either file and never prompts.
func resolveConfig(ctx context.Context, cwd, flagBaseURL, flagAPIToken, flagWorkspaceID, flagTimeout string) (*resolve.Resolved, time.Duration, error) {
	store, err := config.LoadStore()
	if err != nil {
		return nil, 0, err
	}

	repoFile, err := repoconfig.Find(cwd)
	if err != nil {
		return nil, 0, err
	}

	repositoryID := gitremote.OriginIdentity(cwd)

	timeoutStr := flagTimeout
	if timeoutStr == "" {
		timeoutStr = os.Getenv("TIDEN_TIMEOUT")
	}
	if timeoutStr == "" {
		timeoutStr = store.Timeout
	}
	timeout, err := config.ParseTimeout(timeoutStr)
	if err != nil {
		return nil, 0, err
	}

	deps := resolve.Deps{
		Store:           store,
		RepoFile:        repoFile,
		RepositoryID:    repositoryID,
		FlagBaseURL:     flagBaseURL,
		EnvBaseURL:      os.Getenv("TIDEN_BASE_URL"),
		FlagAPIToken:    flagAPIToken,
		EnvAPIToken:     os.Getenv("TIDEN_API_TOKEN"),
		FlagWorkspaceID: flagWorkspaceID,
		EnvWorkspaceID:  os.Getenv("TIDEN_WORKSPACE_ID"),
		NewClient: func(baseURL, token string) resolve.Client {
			return api.NewWithTimeout(baseURL, token, timeout)
		},
	}

	resolved, err := resolve.Resolve(ctx, deps)
	if err != nil {
		return nil, 0, err
	}
	return resolved, timeout, nil
}

// printDiagnostic prints the one required startup line naming the resolved
// workspace, account and how it was decided.
func printDiagnostic(w *os.File, r *resolve.Resolved) {
	account := r.Account
	if account == "" {
		account = "unknown"
	}
	if r.WorkspaceID == "" {
		// An explicitly given token (flag/env) that named no single
		// workspace: the server still starts (see internal/resolve's F6m
		// doc), but there is nothing to put in parentheses - naming it
		// "<none>" beats printing an empty "()" pair.
		fmt.Fprintf(w, "tiden-mcp-server: workspace <none> as %s [%s]\n", account, r.Source)
		return
	}
	name := r.WorkspaceName
	if name == "" {
		name = r.WorkspaceID
	}
	fmt.Fprintf(w, "tiden-mcp-server: workspace %s (%s) as %s [%s]\n", name, r.WorkspaceID, account, r.Source)
}
