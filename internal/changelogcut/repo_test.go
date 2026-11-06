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

// repoEntries reads and parses the pending entries of this repository.
// Malformed ones are reported by TestRepoPendingEntriesConform, so they are
// skipped here.
func repoEntries(t *testing.T) []Entry {
	t.Helper()
	dir := filepath.Join(repoRoot, EntryDir)
	names, err := entryFiles(dir)
	if err != nil {
		t.Fatal(err)
	}
	var entries []Entry
	for _, name := range names {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		if e, err := ParseEntry(name, string(data)); err == nil {
			entries = append(entries, e)
		}
	}
	return entries
}

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

var (
	repoVersionHeadRE = regexp.MustCompile(`(?m)^## \[(\d+)\.(\d+)\.(\d+)\] - \d{4}-\d{2}-\d{2}$`)
	repoAllowedH3     = map[string]bool{}
)

func init() {
	for _, s := range Sections {
		repoAllowedH3["### "+string(s)] = true
	}
}

// TestRepoChangelogIsCuttable cuts the real CHANGELOG.md, with everything
// pending today (entries and any bullets under [Unreleased]), into the next
// patch release in memory, and checks the result still has the structure the
// release automation reads.
func TestRepoChangelogIsCuttable(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(repoRoot, "CHANGELOG.md"))
	if err != nil {
		t.Fatal(err)
	}
	changelog := string(data)
	newest := repoVersionHeadRE.FindStringSubmatch(changelog)
	if newest == nil {
		t.Fatal("CHANGELOG.md has no version heading")
	}
	patch, _ := strconv.Atoi(newest[3])
	next := newest[1] + "." + newest[2] + "." + strconv.Itoa(patch+1)

	entries := append(repoEntries(t),
		Entry{File: "2026-01-01-probe.md", Title: "Probe", Date: "2026-01-01", Section: Fixed, Body: "Probe."})
	out, err := Cut(changelog, entries, next, "2026-01-01", RepoURL)
	if err != nil {
		t.Fatalf("CHANGELOG.md cannot be cut: %v", err)
	}

	before := len(repoVersionHeadRE.FindAllString(changelog, -1))
	if after := len(repoVersionHeadRE.FindAllString(out, -1)); after != before+1 {
		t.Fatalf("cut has %d version headings, want %d", after, before+1)
	}
	if !strings.Contains(out, "## [Unreleased]\n\n## ["+next+"] - 2026-01-01\n") {
		t.Fatalf("[Unreleased] is not directly followed by the new %s heading", next)
	}
	if !strings.Contains(out, "\n["+next+"]: "+RepoURL+"/compare/v") {
		t.Fatalf("cut added no link reference for %s", next)
	}
	for _, line := range strings.Split(out, "\n") {
		switch {
		case strings.HasPrefix(line, "## "):
			if line != "## [Unreleased]" && !versionHeadRE.MatchString(line) {
				t.Errorf("cut produced the heading %q", line)
			}
		case strings.HasPrefix(line, "### "):
			if !repoAllowedH3[line] {
				t.Errorf("cut produced the subsection %q", line)
			}
		}
	}
	for _, e := range entries {
		if !strings.Contains(out, renderBullet(e.Body)) {
			t.Errorf("cut lost entry %s", e.File)
		}
	}
	// Released history, from the previous newest heading to the link block,
	// comes through byte for byte.
	lines := strings.Split(changelog, "\n")
	first, links := -1, -1
	for i, l := range lines {
		if first < 0 && versionHeadRE.MatchString(l) {
			first = i
		}
		if links < 0 && linkRefLineRE.MatchString(l) {
			links = i
		}
	}
	if first < 0 || links < first {
		t.Fatal("cannot locate released history in CHANGELOG.md")
	}
	if history := strings.Join(lines[first:links], "\n"); !strings.Contains(out, history) {
		t.Fatal("cut changed released history")
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
