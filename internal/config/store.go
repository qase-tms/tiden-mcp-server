// Package config reads the tiden CLI's ~/.tiden/config.json — both the
// legacy v1 flat shape and the v2 workspace-keyed shape — and the top-level
// per-request Timeout setting. It is READ-ONLY: unlike the CLI's own
// internal/config, this package never writes the file. tiden-mcp-server is a
// public binary installed with `go install ...@latest`, so it cannot depend
// on the internal tiden-cli module; this is a deliberate, minimal duplicate
// of the CLI's parsing logic, kept in lockstep via shared golden fixtures
// (see testdata/shared/README.md).
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// Entry is one workspace's credentials in the v2 global config file.
type Entry struct {
	BaseURL  string `json:"baseUrl"`
	APIToken string `json:"apiToken"`
	Account  string `json:"account,omitempty"`
	Name     string `json:"name,omitempty"`
}

// Login is a (baseUrl, apiToken) pair not yet placed under any workspace
// entry — the v1-with-empty-workspaceId case ("loose").
type Login struct {
	BaseURL  string
	APIToken string
	Account  string
}

// Store is the raw, version-aware view of ~/.tiden/config.json: the global
// login/workspace data plus everything else the file carries. It is
// read-only — this package never persists a Store back to disk (that stays
// the CLI's job; see tiden-cli's internal/config.Store.Save).
type Store struct {
	Path        string
	Loaded      bool
	FileVersion int // 0 no file, 1 flat (v1), 2 workspace-keyed (v2)

	// Timeout is the raw global timeout string ("" = default 30s). Present
	// as a top-level key in both file versions.
	Timeout string

	Workspaces map[string]Entry
	Loose      []Login

	// Extra holds every other top-level key the file carries, verbatim.
	// Unused by this read-only package but kept for parity with the CLI's
	// Store shape and so future fields don't need re-deriving this parse.
	Extra map[string]json.RawMessage
}

// LoadStore reads $HOME/.tiden/config.json. A missing file (or an
// unresolvable home directory) is not an error: it returns an empty Store
// with FileVersion 0.
func LoadStore() (*Store, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return &Store{}, nil
	}
	return LoadStoreFrom(filepath.Join(home, ".tiden", "config.json"))
}

// LoadStoreFrom reads and parses the config file at path. A missing file is
// not an error: it returns an empty Store{Path: path, FileVersion: 0}. A
// malformed file returns an error alongside an empty, usable Store.
func LoadStoreFrom(path string) (*Store, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return &Store{Path: path}, nil
	}
	s, perr := parseStore(data)
	if perr != nil {
		return &Store{Path: path}, fmt.Errorf("parse %s: %w", path, perr)
	}
	s.Path = path
	s.Loaded = true
	return s, nil
}

// parseStore parses the raw file bytes into a Store. Detection: a top-level
// "workspaces" key means v2 (even alongside stray legacy flat keys — v2
// wins and those flat keys are dropped); otherwise it's the v1 flat shape.
// This must stay byte-for-byte in agreement with tiden-cli's
// internal/config.parseStore — see testdata/shared/README.md.
func parseStore(data []byte) (*Store, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	s := &Store{Extra: map[string]json.RawMessage{}}

	if wsRaw, ok := raw["workspaces"]; ok {
		s.FileVersion = 2
		var workspaces map[string]Entry
		if err := json.Unmarshal(wsRaw, &workspaces); err != nil {
			return nil, fmt.Errorf("parse workspaces: %w", err)
		}
		if workspaces == nil {
			workspaces = map[string]Entry{}
		}
		s.Workspaces = workspaces
	} else {
		s.FileVersion = 1
		s.Workspaces = map[string]Entry{}

		var baseURL, apiToken, workspaceID string
		if v, ok := raw["baseUrl"]; ok {
			_ = json.Unmarshal(v, &baseURL)
		}
		if v, ok := raw["apiToken"]; ok {
			_ = json.Unmarshal(v, &apiToken)
		}
		if v, ok := raw["workspaceId"]; ok {
			_ = json.Unmarshal(v, &workspaceID)
		}
		if apiToken != "" {
			if workspaceID != "" {
				s.Workspaces[workspaceID] = Entry{BaseURL: baseURL, APIToken: apiToken}
			} else {
				s.Loose = append(s.Loose, Login{BaseURL: baseURL, APIToken: apiToken})
			}
		}
	}

	if v, ok := raw["timeout"]; ok {
		var t string
		_ = json.Unmarshal(v, &t)
		s.Timeout = t
	}

	known := map[string]bool{
		"version": true, "timeout": true, "workspaces": true,
		"baseUrl": true, "apiToken": true, "workspaceId": true,
	}
	for k, v := range raw {
		if known[k] {
			continue
		}
		s.Extra[k] = v
	}

	return s, nil
}

// DistinctLogins returns the unique (BaseURL, APIToken) pairs found across
// Workspaces and Loose, each with its Account when known. Order is
// deterministic: workspace-derived logins first (by ascending workspace
// id), then any remaining Loose logins in their existing order. This
// ordering is significant: the resolution ladder (internal/resolve) tries
// logins in this order.
func (s *Store) DistinctLogins() []Login {
	type key struct{ baseURL, apiToken string }
	seen := map[key]int{}
	var result []Login

	add := func(baseURL, apiToken, account string) {
		k := key{baseURL, apiToken}
		if idx, ok := seen[k]; ok {
			if result[idx].Account == "" && account != "" {
				result[idx].Account = account
			}
			return
		}
		seen[k] = len(result)
		result = append(result, Login{BaseURL: baseURL, APIToken: apiToken, Account: account})
	}

	ids := make([]string, 0, len(s.Workspaces))
	for id := range s.Workspaces {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		e := s.Workspaces[id]
		add(e.BaseURL, e.APIToken, e.Account)
	}
	for _, l := range s.Loose {
		add(l.BaseURL, l.APIToken, l.Account)
	}
	return result
}

// HasAnyLogin reports whether the store carries any credential at all
// (a workspace entry or a loose login).
func (s *Store) HasAnyLogin() bool {
	return len(s.Workspaces) > 0 || len(s.Loose) > 0
}
