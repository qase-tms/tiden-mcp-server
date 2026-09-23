// Package repoconfig reads the repo-local .tiden/config.json — the file the
// tiden CLI uses to durably bind a checkout to a Tiden product/workspace. It
// is READ-ONLY: tiden-mcp-server never prompts and never writes; a binding
// is created or changed only by the CLI (`tiden setup`, `tiden workspace
// use`, `tiden product bind`).
package repoconfig

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// maxWalkLevels bounds the upward directory walk in Find, matching the
// CLI's internal/repoconfig.Find (40 levels to the filesystem root).
const maxWalkLevels = 40

// File is the parsed .tiden/config.json at Path.
type File struct {
	Path        string
	Exists      bool
	WorkspaceID string
	ProductID   string
	// Extra holds every other key the file carries (e.g. ciInstallOffered),
	// verbatim. Unused by this read-only package but kept for parity with
	// the CLI's File shape.
	Extra map[string]json.RawMessage
}

// Find walks up from startDir (up to maxWalkLevels, to the filesystem root)
// looking for the nearest .tiden/config.json. It returns Exists=false and
// Path="" when none is found anywhere up the tree.
//
// The walk never returns $HOME/.tiden/config.json itself: that file is the
// CLI's global login/workspace store (internal/config.LoadStore), not a repo
// binding. A directory under HOME with no .tiden of its own used to have its
// walk-up reach that global file and read its top-level workspaceId/productId
// as if they bound the repository — a stray top-level workspaceId there (left
// by an older binary after the v2 migration) then silently became this
// repository's "binding" (TIDEN-68 Finding 2026-09-23, F-HOME). The global
// file is identified the same way internal/config.LoadStore locates it
// (os.UserHomeDir + .tiden/config.json) and compared by file identity
// (os.SameFile), not by path string, so a symlinked spelling of HOME (e.g.
// macOS's /var vs /private/var) is still recognized as the same file.
func Find(startDir string) (*File, error) {
	globalInfo := globalStoreInfo()

	dir := startDir
	for i := 0; i < maxWalkLevels && dir != ""; i++ {
		p := filepath.Join(dir, ".tiden", "config.json")
		if info, err := os.Stat(p); err == nil {
			if globalInfo == nil || !os.SameFile(info, globalInfo) {
				return Read(p)
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return &File{}, nil
}

// globalStoreInfo returns the os.FileInfo of $HOME/.tiden/config.json (the
// CLI's global store), or nil when HOME cannot be resolved or the file does
// not exist there.
func globalStoreInfo() os.FileInfo {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	info, err := os.Stat(filepath.Join(home, ".tiden", "config.json"))
	if err != nil {
		return nil
	}
	return info
}

// Read parses the file at path. A missing file is not an error: it returns
// Exists=false. A malformed file returns an error.
func Read(path string) (*File, error) {
	f := &File{Path: path, Extra: map[string]json.RawMessage{}}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return f, nil
		}
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	f.Exists = true

	if len(data) == 0 {
		return f, nil
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}

	if v, ok := raw["workspaceId"]; ok {
		_ = json.Unmarshal(v, &f.WorkspaceID)
	}
	if v, ok := raw["productId"]; ok {
		_ = json.Unmarshal(v, &f.ProductID)
	}
	for k, v := range raw {
		if k == "workspaceId" || k == "productId" {
			continue
		}
		f.Extra[k] = v
	}
	return f, nil
}
