package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"

	"github.com/qase-tms/tiden-mcp-server/internal/api"
	"github.com/qase-tms/tiden-mcp-server/internal/config"
	"github.com/qase-tms/tiden-mcp-server/internal/mcpserver"
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
		if errors.Is(err, api.ErrUnauthorized) {
			os.Exit(2)
		}
		fmt.Fprintf(os.Stderr, "error: %s\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, flagBaseURL, flagAPIToken, flagWorkspaceID, flagTimeout string) error {
	cfg, err := config.Load(flagBaseURL, flagAPIToken, flagWorkspaceID)
	if err != nil {
		return err
	}
	if flagTimeout != "" {
		cfg.Timeout = flagTimeout
	}
	if missing := cfg.Check(); len(missing) > 0 {
		return fmt.Errorf("missing required config: %v. Set baseUrl and apiToken in ~/.tiden/config.json or via flags/env vars", missing)
	}

	timeout, err := cfg.RequestTimeout()
	if err != nil {
		return err
	}
	client := api.NewWithTimeout(cfg.BaseURL, cfg.APIToken, timeout)
	srv := mcpserver.New(client, cfg.WorkspaceID)
	return mcpserver.Run(ctx, srv)
}
