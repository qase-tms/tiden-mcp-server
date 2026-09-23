package repoconfig

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestFind_WalksUpNestedDir(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".tiden"), 0o755); err != nil {
		t.Fatal(err)
	}
	body := []byte(`{"workspaceId": "ws-1234", "productId": "prod-5678"}`)
	if err := os.WriteFile(filepath.Join(root, ".tiden", "config.json"), body, 0o644); err != nil {
		t.Fatal(err)
	}

	nested := filepath.Join(root, "a", "b", "c")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}

	f, err := Find(nested)
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if !f.Exists {
		t.Fatalf("f.Exists = false, want true")
	}
	if f.WorkspaceID != "ws-1234" || f.ProductID != "prod-5678" {
		t.Errorf("f = %+v", f)
	}
}

func TestFind_NearestFileWins(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".tiden"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".tiden", "config.json"), []byte(`{"workspaceId": "ws-far"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	nested := filepath.Join(root, "nested")
	if err := os.MkdirAll(filepath.Join(nested, ".tiden"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, ".tiden", "config.json"), []byte(`{"workspaceId": "ws-near"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	f, err := Find(nested)
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if f.WorkspaceID != "ws-near" {
		t.Errorf("WorkspaceID = %q, want ws-near (nearest file must win)", f.WorkspaceID)
	}
}

// TestFind_SkipsGlobalStoreFile is TIDEN-68's F-HOME fix (D17 rule 1): the
// walk-up must never return $HOME/.tiden/config.json itself — that is the
// CLI's global login/workspace store, not a repo binding. A release that
// shipped before this fix left a stray top-level workspaceId/productId in
// that file (an older binary writing after the v2 migration), which this
// package would otherwise read as if it were the repository's own binding.
func TestFind_SkipsGlobalStoreFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.MkdirAll(filepath.Join(home, ".tiden"), 0o755); err != nil {
		t.Fatal(err)
	}
	body := []byte(`{"version": 2, "workspaceId": "ws-stray", "productId": "prod-stray", "workspaces": {}}`)
	if err := os.WriteFile(filepath.Join(home, ".tiden", "config.json"), body, 0o644); err != nil {
		t.Fatal(err)
	}

	cwd := filepath.Join(home, "work", "repo")
	if err := os.MkdirAll(cwd, 0o755); err != nil {
		t.Fatal(err)
	}

	f, err := Find(cwd)
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if f.Exists || f.WorkspaceID != "" || f.ProductID != "" {
		t.Errorf("Find(%q) = %+v, want no binding (the global store must never be read as a repo binding)", cwd, f)
	}
}

// TestFind_StillFindsRealFileUnderHome makes sure the global-store skip in
// TestFind_SkipsGlobalStoreFile does not turn into "ignore every file under
// HOME": a real repo-local file elsewhere under HOME must still be found.
func TestFind_StillFindsRealFileUnderHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.MkdirAll(filepath.Join(home, ".tiden"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".tiden", "config.json"), []byte(`{"version": 2, "workspaceId": "ws-stray", "workspaces": {}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	repoDir := filepath.Join(home, "work")
	if err := os.MkdirAll(filepath.Join(repoDir, ".tiden"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, ".tiden", "config.json"), []byte(`{"workspaceId": "ws-real"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	cwd := filepath.Join(repoDir, "repo")
	if err := os.MkdirAll(cwd, 0o755); err != nil {
		t.Fatal(err)
	}

	f, err := Find(cwd)
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if !f.Exists || f.WorkspaceID != "ws-real" {
		t.Errorf("Find(%q) = %+v, want the real repo-local file at %s", cwd, f, filepath.Join(repoDir, ".tiden", "config.json"))
	}
}

// TestFind_SkipsGlobalStoreFile_SymlinkedPathSpelling covers D17 rule 1's
// "compare by file identity" requirement: on macOS, a path reached through
// os.Getwd() after a chdir is typically returned in its resolved
// (non-symlinked, e.g. /private/var/...) form, while HOME stays whatever
// spelling was set (e.g. /var/...). A string comparison of paths would miss
// that these name the same file; os.SameFile must not.
func TestFind_SkipsGlobalStoreFile_SymlinkedPathSpelling(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.MkdirAll(filepath.Join(home, ".tiden"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".tiden", "config.json"), []byte(`{"version": 2, "workspaceId": "ws-stray", "workspaces": {}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	resolvedHome, err := filepath.EvalSymlinks(home)
	if err != nil {
		t.Fatalf("EvalSymlinks: %v", err)
	}
	cwd := filepath.Join(resolvedHome, "work", "repo")
	if err := os.MkdirAll(cwd, 0o755); err != nil {
		t.Fatal(err)
	}

	f, err := Find(cwd)
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if f.Exists || f.WorkspaceID != "" {
		t.Errorf("Find(%q) = %+v, want no binding even when cwd is spelled via the resolved form of HOME", cwd, f)
	}
}

func TestFind_NoFileReturnsEmpty(t *testing.T) {
	dir := t.TempDir()
	f, err := Find(dir)
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if f.Exists {
		t.Errorf("f.Exists = true, want false")
	}
	if f.WorkspaceID != "" || f.ProductID != "" {
		t.Errorf("f = %+v, want empty", f)
	}
}

func TestRead_MalformedFileReturnsError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte(`{not json`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Read(path); err == nil {
		t.Fatal("expected error for malformed file")
	}
}

func TestRead_PreservesExtraKeys(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	body := []byte(`{"productId": "p1", "ciInstallOffered": true}`)
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatal(err)
	}
	f, err := Read(path)
	if err != nil {
		t.Fatal(err)
	}
	raw, ok := f.Extra["ciInstallOffered"]
	if !ok {
		t.Fatalf("Extra = %+v, want ciInstallOffered present", f.Extra)
	}
	var b bool
	if err := json.Unmarshal(raw, &b); err != nil || !b {
		t.Errorf("ciInstallOffered = %s, want true", raw)
	}
}
