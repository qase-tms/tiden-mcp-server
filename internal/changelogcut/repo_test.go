package changelogcut

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

const repoRoot = "../.."

// TestRepoPendingEntriesConform fails the PR that adds a malformed entry,
// rather than the release PR that later tries to fold it.
func TestRepoPendingEntriesConform(t *testing.T) {
	dir := filepath.Join(repoRoot, EntryDir)
	names, err := entryFiles(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range names {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := ParseEntry(name, string(data)); err != nil {
			t.Errorf("%v\n(grammar: %s/README.md)", err, EntryDir)
		}
	}
}

// TestRepoChangelogIsCuttable proves the real CHANGELOG.md, with whatever is
// pending under docs/changelog/ and [Unreleased] today, can be cut into the
// next patch release.
func TestRepoChangelogIsCuttable(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(repoRoot, "CHANGELOG.md"))
	if err != nil {
		t.Fatal(err)
	}
	changelog := string(data)
	newest := regexp.MustCompile(`(?m)^## \[(\d+)\.(\d+)\.(\d+)\] - `).FindStringSubmatch(changelog)
	if newest == nil {
		t.Fatal("CHANGELOG.md has no version heading")
	}
	patch, _ := strconv.Atoi(newest[3])
	next := newest[1] + "." + newest[2] + "." + strconv.Itoa(patch+1)

	probe := Entry{File: "2026-01-01-probe.md", Title: "Probe", Date: "2026-01-01", Section: Fixed, Body: "Probe."}
	out, err := Cut(changelog, []Entry{probe}, next, "2026-01-01", RepoURL)
	if err != nil {
		t.Fatalf("CHANGELOG.md cannot be cut: %v", err)
	}
	if !strings.Contains(out, "\n["+next+"]: "+RepoURL+"/compare/v") {
		t.Fatalf("cut of the real CHANGELOG.md added no link reference for %s", next)
	}
}

func TestRepoEntryDirHasReadme(t *testing.T) {
	_, err := os.Stat(filepath.Join(repoRoot, EntryDir, "README.md"))
	if errors.Is(err, os.ErrNotExist) {
		t.Fatalf("%s/README.md is missing: it defines the entry grammar agents follow", EntryDir)
	}
	if err != nil {
		t.Fatal(err)
	}
}
