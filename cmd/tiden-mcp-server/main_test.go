package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/qase-tms/tiden-mcp-server/internal/resolve"
)

func writeHomeConfig(t *testing.T, home string, body []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(home, ".tiden"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".tiden", "config.json"), body, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestResolveConfig_V2FileWithRepoBinding_FastPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	writeHomeConfig(t, home, []byte(`{
		"version": 2,
		"workspaces": {
			"ws-1111": {"baseUrl": "https://app.tiden.ai", "apiToken": "tdn_tok", "account": "me@example.dev", "name": "Personal"}
		}
	}`))

	cwd := t.TempDir()
	if err := os.MkdirAll(filepath.Join(cwd, ".tiden"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cwd, ".tiden", "config.json"), []byte(`{"workspaceId": "ws-1111"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	resolved, _, err := resolveConfig(context.Background(), cwd, "", "", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved.WorkspaceID != "ws-1111" || resolved.APIToken != "tdn_tok" || resolved.Source != resolve.SourceRepoBinding {
		t.Errorf("resolved = {BaseURL:%q WorkspaceID:%q Source:%q token:%s}", resolved.BaseURL, resolved.WorkspaceID, resolved.Source, redactToken(resolved.APIToken))
	}
}

func TestResolveConfig_UnresolvedV2File_ReturnsTypedErrorNeverMissingConfigMessage(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	// Two workspace entries, no repo binding anywhere: ambiguous (E3), not
	// the old "missing required config" message a v2 file used to trigger
	// (baseUrl/apiToken/workspaceId all live inside workspaces, not at the
	// top level, so the pre-TIDEN-68 flat Check() always misfired on a v2 file).
	writeHomeConfig(t, home, []byte(`{
		"version": 2,
		"workspaces": {
			"ws-1111": {"baseUrl": "https://app.tiden.ai", "apiToken": "tok-a", "name": "A"},
			"ws-2222": {"baseUrl": "https://app.tiden.ai", "apiToken": "tok-b", "name": "B"}
		}
	}`))

	cwd := t.TempDir()

	_, _, err := resolveConfig(context.Background(), cwd, "", "", "", "")
	if err == nil {
		t.Fatal("expected an error")
	}
	var rerr *resolve.Error
	if !errors.As(err, &rerr) {
		t.Fatalf("err = %T(%v), want *resolve.Error", err, err)
	}
	if rerr.Code != "E3" {
		t.Errorf("Code = %q, want E3", rerr.Code)
	}
	if strings.Contains(err.Error(), "missing required config") {
		t.Errorf("err = %q, must never repeat the old flat-config message for a v2 file", err.Error())
	}
}

func TestResolveConfig_NoLoginFile_ReturnsSetupHint(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home) // no ~/.tiden/config.json at all

	cwd := t.TempDir()

	_, _, err := resolveConfig(context.Background(), cwd, "", "", "", "")
	var rerr *resolve.Error
	if !errors.As(err, &rerr) {
		t.Fatalf("err = %T(%v), want *resolve.Error", err, err)
	}
	if rerr.Code != "NO_LOGIN" {
		t.Errorf("Code = %q, want NO_LOGIN", rerr.Code)
	}
	if rerr.Message != "Not logged in. Run `tiden setup`." {
		t.Errorf("Message = %q", rerr.Message)
	}
}

func TestResolveConfig_FlagOverridesFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	writeHomeConfig(t, home, []byte(`{
		"version": 2,
		"workspaces": {
			"ws-1111": {"baseUrl": "https://app.tiden.ai", "apiToken": "tok-file", "name": "File"}
		}
	}`))
	cwd := t.TempDir()

	resolved, _, err := resolveConfig(context.Background(), cwd, "https://flag.example", "tok-flag", "ws-flag", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved.BaseURL != "https://flag.example" || resolved.APIToken != "tok-flag" || resolved.WorkspaceID != "ws-flag" || resolved.Source != resolve.SourceFlag {
		t.Errorf("resolved = {BaseURL:%q WorkspaceID:%q Source:%q token:%s}", resolved.BaseURL, resolved.WorkspaceID, resolved.Source, redactToken(resolved.APIToken))
	}
}

// TestResolveConfig_TokenOnlyEnv_StartsWithEmptyWorkspace is the F6m ladder
// amendment: an explicitly given token (TIDEN_API_TOKEN, mirroring the
// pre-TIDEN-68 token-only-env deployment) must start the server even when
// no workspace can be chosen for it - never an E5/refusal at startup.
func TestResolveConfig_TokenOnlyEnv_StartsWithEmptyWorkspace(t *testing.T) {
	// HOME is already an empty temp dir (TestMain): no ~/.tiden/config.json,
	// so the only login the ladder knows about is the env override.
	t.Setenv("TIDEN_API_TOKEN", "tok-env")
	t.Setenv("TIDEN_BASE_URL", "https://app.tiden.ai")

	cwd := t.TempDir()

	resolved, _, err := resolveConfig(context.Background(), cwd, "", "", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v (a token-only env must never be refused)", err)
	}
	if resolved.WorkspaceID != "" {
		t.Errorf("WorkspaceID = %q, want empty (no repo binding, no product/repository match, no store to probe)", resolved.WorkspaceID)
	}
	if resolved.BaseURL != "https://app.tiden.ai" || resolved.APIToken != "tok-env" || resolved.Source != resolve.SourceEnv {
		t.Errorf("resolved = {BaseURL:%q Source:%q token:%s}", resolved.BaseURL, resolved.Source, redactToken(resolved.APIToken))
	}
}

func TestPrintDiagnostic_EmptyWorkspaceRendersSensibly(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	printDiagnostic(w, &resolve.Resolved{
		BaseURL:  "https://app.tiden.ai",
		APIToken: "tok-env",
		Source:   resolve.SourceEnv,
	})
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	out := string(buf[:n])
	if !strings.Contains(out, "<none>") {
		t.Errorf("out = %q, want it to name the missing workspace sensibly (e.g. <none>), not an empty pair of parens", out)
	}
	if strings.Contains(out, "()") {
		t.Errorf("out = %q, must not print an empty (id) pair", out)
	}
}

func TestPrintResolutionError_PrintsMessageAndHints(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	printResolutionError(w, &resolve.Error{
		Message: "This repository is bound to workspace ws-1 (Team), and none of your logged-in accounts is a member.",
		Hints:   []string{"tiden setup --workspace-id ws-1"},
	})
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	out := string(buf[:n])
	if !strings.Contains(out, "This repository is bound to workspace ws-1") {
		t.Errorf("out = %q, want the message", out)
	}
	if !strings.Contains(out, "tiden setup --workspace-id ws-1") {
		t.Errorf("out = %q, want the hint", out)
	}
}
