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
func Find(startDir string) (*File, error) {
	dir := startDir
	for i := 0; i < maxWalkLevels && dir != ""; i++ {
		p := filepath.Join(dir, ".tiden", "config.json")
		if _, err := os.Stat(p); err == nil {
			return Read(p)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return &File{}, nil
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
