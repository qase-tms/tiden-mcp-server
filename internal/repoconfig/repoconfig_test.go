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
