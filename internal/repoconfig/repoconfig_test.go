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

// TestFind_SkipsGlobalStoreFile_NoContentMarkers isolates the identity guard
// from the content guard: a global store file that (for whatever reason -
// a stub written before first login, a future format change, ...) carries
// none of storeMarkerKeys must still never be read as a repo binding. Only
// the identity check (this IS $HOME/.tiden/config.json) can catch this case;
// the content check alone would not.
func TestFind_SkipsGlobalStoreFile_NoContentMarkers(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.MkdirAll(filepath.Join(home, ".tiden"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Deliberately no apiToken/workspaces/version key anywhere.
	body := []byte(`{"workspaceId": "ws-stray", "timeout": "30s"}`)
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
	if f.Exists || f.WorkspaceID != "" {
		t.Errorf("Find(%q) = %+v, want no binding (identity alone must skip the global store even without a content marker)", cwd, f)
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

// TestFind_SkipsForeignCredentialStoreByContent covers TIDEN-68 ledger rule
// D18.1. The identity check (TestFind_SkipsGlobalStoreFile) only knows this
// process's own HOME. With HOME pointing elsewhere (a sandboxed HOME, an
// agent sandbox, sudo, ...), a credential store sitting at an ancestor of the
// cwd is a *different* file by identity but must still never be read as a
// repo binding: its top-level object carries a marker key (apiToken,
// workspaces, or version) that no repo-local .tiden/config.json ever has.
func TestFind_SkipsForeignCredentialStoreByContent(t *testing.T) {
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome) // this process's own HOME - unrelated store

	realRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(realRoot, ".tiden"), 0o755); err != nil {
		t.Fatal(err)
	}
	body := []byte(`{"version": 2, "workspaceId": "ws-foreign", "workspaces": {"ws-foreign": {"baseUrl": "https://app.tiden.ai", "apiToken": "tok-x"}}}`)
	if err := os.WriteFile(filepath.Join(realRoot, ".tiden", "config.json"), body, 0o644); err != nil {
		t.Fatal(err)
	}

	cwd := filepath.Join(realRoot, "work", "proj")
	if err := os.MkdirAll(cwd, 0o755); err != nil {
		t.Fatal(err)
	}

	f, err := Find(cwd)
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if f.Exists || f.WorkspaceID != "" {
		t.Errorf("Find(%q) = %+v, want no binding (a foreign credential store must never be read as a repo binding, whichever HOME it belongs to)", cwd, f)
	}
}

// TestFind_RealRepoFileBetweenCwdAndForeignStoreStillFound makes sure the
// content-based skip above does not turn into "ignore everything above a
// store-shaped file": a real repo-local binding sitting between the cwd and
// the foreign store must still be found.
func TestFind_RealRepoFileBetweenCwdAndForeignStoreStillFound(t *testing.T) {
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)

	realRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(realRoot, ".tiden"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(realRoot, ".tiden", "config.json"), []byte(`{"version": 2, "workspaceId": "ws-foreign", "workspaces": {}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	repoDir := filepath.Join(realRoot, "work")
	if err := os.MkdirAll(filepath.Join(repoDir, ".tiden"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, ".tiden", "config.json"), []byte(`{"workspaceId": "ws-real"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	cwd := filepath.Join(repoDir, "proj")
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

// TestFind_PlainBindingFileWithOnlyWorkspaceAndProductIDsStillFound is the
// control for the content-based skip: an ordinary repo-local binding file
// (workspaceId/productId, no store marker key) must never be mistaken for a
// credential store.
func TestFind_PlainBindingFileWithOnlyWorkspaceAndProductIDsStillFound(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".tiden"), 0o755); err != nil {
		t.Fatal(err)
	}
	body := []byte(`{"workspaceId": "ws-plain", "productId": "prod-plain", "ciInstallOffered": true}`)
	if err := os.WriteFile(filepath.Join(dir, ".tiden", "config.json"), body, 0o644); err != nil {
		t.Fatal(err)
	}

	f, err := Find(dir)
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if !f.Exists || f.WorkspaceID != "ws-plain" || f.ProductID != "prod-plain" {
		t.Errorf("Find(%q) = %+v, want the plain binding file found (no store marker key present)", dir, f)
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
