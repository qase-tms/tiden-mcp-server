package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

// --- golden fixtures (shared verbatim with tiden-cli/internal/config/testdata/shared) ---

type expectedEntry struct {
	BaseURL  string `json:"baseUrl"`
	APIToken string `json:"apiToken"`
	Account  string `json:"account"`
	Name     string `json:"name"`
}

type expectedLoose struct {
	BaseURL  string `json:"baseUrl"`
	APIToken string `json:"apiToken"`
	Account  string `json:"account"`
}

type expectedFixture struct {
	FileVersion int                      `json:"fileVersion"`
	Entries     map[string]expectedEntry `json:"entries"`
	Loose       []expectedLoose          `json:"loose"`
	ExtraKeys   []string                 `json:"extraKeys"`
}

func loadExpectedFixtures(t *testing.T) map[string]expectedFixture {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "shared", "expected.json"))
	if err != nil {
		t.Fatalf("read expected.json: %v", err)
	}
	var out map[string]expectedFixture
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("parse expected.json: %v", err)
	}
	return out
}

func TestLoadStoreFromGoldenFixtures(t *testing.T) {
	expected := loadExpectedFixtures(t)

	for name, want := range expected {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join("testdata", "shared", name+".json")
			got, err := LoadStoreFrom(path)
			if err != nil {
				t.Fatalf("LoadStoreFrom(%s): %v", path, err)
			}

			if got.FileVersion != want.FileVersion {
				t.Errorf("FileVersion = %d, want %d", got.FileVersion, want.FileVersion)
			}

			if len(got.Workspaces) != len(want.Entries) {
				t.Fatalf("Workspaces = %+v, want %+v", got.Workspaces, want.Entries)
			}
			for id, wantEntry := range want.Entries {
				gotEntry, ok := got.Workspaces[id]
				if !ok {
					t.Errorf("missing workspace entry %q", id)
					continue
				}
				if gotEntry.BaseURL != wantEntry.BaseURL || gotEntry.APIToken != wantEntry.APIToken ||
					gotEntry.Account != wantEntry.Account || gotEntry.Name != wantEntry.Name {
					t.Errorf("entry %q = %+v, want %+v", id, gotEntry, wantEntry)
				}
			}

			if len(got.Loose) != len(want.Loose) {
				t.Fatalf("Loose = %+v, want %+v", got.Loose, want.Loose)
			}
			for i, wantLoose := range want.Loose {
				gotLoose := got.Loose[i]
				if gotLoose.BaseURL != wantLoose.BaseURL || gotLoose.APIToken != wantLoose.APIToken || gotLoose.Account != wantLoose.Account {
					t.Errorf("loose[%d] = %+v, want %+v", i, gotLoose, wantLoose)
				}
			}

			gotExtraKeys := make([]string, 0, len(got.Extra))
			for k := range got.Extra {
				gotExtraKeys = append(gotExtraKeys, k)
			}
			sort.Strings(gotExtraKeys)
			wantExtraKeys := append([]string(nil), want.ExtraKeys...)
			sort.Strings(wantExtraKeys)
			if len(gotExtraKeys) != len(wantExtraKeys) {
				t.Fatalf("extraKeys = %v, want %v", gotExtraKeys, wantExtraKeys)
			}
			for i := range gotExtraKeys {
				if gotExtraKeys[i] != wantExtraKeys[i] {
					t.Errorf("extraKeys = %v, want %v", gotExtraKeys, wantExtraKeys)
				}
			}
		})
	}
}

func TestLoadStoreMissingFileIsEmptyStore(t *testing.T) {
	path := filepath.Join(t.TempDir(), "does-not-exist", "config.json")
	s, err := LoadStoreFrom(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.FileVersion != 0 || s.Loaded {
		t.Errorf("s = %+v, want zero-value FileVersion/Loaded", s)
	}
	if len(s.Workspaces) != 0 || len(s.Loose) != 0 {
		t.Errorf("s = %+v, want empty Workspaces/Loose", s)
	}
}

func TestLoadStoreHomeEnv(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if err := os.MkdirAll(filepath.Join(home, ".tiden"), 0o755); err != nil {
		t.Fatal(err)
	}
	src, err := os.ReadFile(filepath.Join("testdata", "shared", "v2.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".tiden", "config.json"), src, 0o600); err != nil {
		t.Fatal(err)
	}

	s, err := LoadStore()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.FileVersion != 2 {
		t.Errorf("FileVersion = %d, want 2", s.FileVersion)
	}
	if len(s.Workspaces) != 3 {
		t.Errorf("Workspaces = %+v, want 3 entries", s.Workspaces)
	}
}

func TestDistinctLogins_OrderedByAscendingWorkspaceIDThenLoose(t *testing.T) {
	path := filepath.Join("testdata", "shared", "v2.json")
	s, err := LoadStoreFrom(path)
	if err != nil {
		t.Fatal(err)
	}
	logins := s.DistinctLogins()
	// v2.json: ws-1111 and ws-2222 share tdn_shared_token (me@example.dev),
	// ws-3333 has tdn_other_token (work@example.dev). Ascending ws id means
	// ws-1111's login (shared token) comes first, then ws-3333's.
	if len(logins) != 2 {
		t.Fatalf("DistinctLogins() = %+v, want 2 distinct logins", logins)
	}
	if logins[0].APIToken != "tdn_shared_token" || logins[0].Account != "me@example.dev" {
		t.Errorf("logins[0] = %+v", logins[0])
	}
	if logins[1].APIToken != "tdn_other_token" || logins[1].Account != "work@example.dev" {
		t.Errorf("logins[1] = %+v", logins[1])
	}
}
