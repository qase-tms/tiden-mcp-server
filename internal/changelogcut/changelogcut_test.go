package changelogcut

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const exampleURL = "https://github.com/qase-tms/example"

const validEntry = "# Changelog — Restart an expired resumed sign-in\n" +
	"\n" +
	"**Date:** 2026-11-07\n" +
	"**Section:** Fixed\n" +
	"\n" +
	"A re-run that resumed a saved sign-in in its last minute no longer exits 1\n" +
	"with no link: `tiden setup --agent` starts a new sign-in. (#271)\n"

func readFixture(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "cut", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestParseEntry(t *testing.T) {
	got, err := ParseEntry("2026-11-07-restart-expired-sign-in.md", validEntry)
	if err != nil {
		t.Fatalf("ParseEntry: %v", err)
	}
	want := Entry{
		File:    "2026-11-07-restart-expired-sign-in.md",
		Title:   "Restart an expired resumed sign-in",
		Date:    "2026-11-07",
		Section: Fixed,
		Body: "A re-run that resumed a saved sign-in in its last minute no longer exits 1\n" +
			"with no link: `tiden setup --agent` starts a new sign-in. (#271)",
	}
	if got != want {
		t.Fatalf("ParseEntry = %+v\nwant %+v", got, want)
	}
}

func TestParseEntryRejects(t *testing.T) {
	tests := []struct {
		name    string
		file    string
		content string
		want    []string
	}{
		{"filename", "notes.md", validEntry, []string{"filename"}},
		{"filename date", "2026-13-07-x.md", validEntry, []string{"filename"}},
		{"hyphen title", "2026-11-07-x.md",
			strings.Replace(validEntry, "# Changelog — ", "# Changelog - ", 1), []string{"title"}},
		{"empty title", "2026-11-07-x.md",
			strings.Replace(validEntry, "Restart an expired resumed sign-in", " ", 1), []string{"title"}},
		{"date mismatch", "2026-11-08-x.md", validEntry, []string{"Date"}},
		{"unknown section", "2026-11-07-x.md",
			strings.Replace(validEntry, "**Section:** Fixed", "**Section:** Improved", 1), []string{"Section"}},
		{"empty body", "2026-11-07-x.md",
			strings.SplitAfter(validEntry, "**Section:** Fixed\n")[0] + "\n\n", []string{"body"}},
		{"heading in body", "2026-11-07-x.md", validEntry + "\n### Notes\n\nMore.\n", []string{"heading"}},
		{"indented heading in body", "2026-11-07-x.md", validEntry + "\n  ## Why\n", []string{"heading"}},
		{"heading after a bare CR", "2026-11-07-x.md",
			validEntry + "Fixed command.\r## [9.9.9] - 2026-11-07\n", []string{"heading"}},
		{"link reference definition", "2026-11-07-x.md",
			validEntry + "\n[0.53.0]: https://example.test/wrong\n", []string{"reference"}},
		{"indented link reference definition", "2026-11-07-x.md",
			validEntry + "\n  [docs]: https://example.test/\n", []string{"reference"}},
		{"link reference definition with the URL on the next line", "2026-11-07-x.md",
			validEntry + "\n[0.53.0]:\n  https://example.test/wrong\n", []string{"reference"}},
		{"every rule reported", "2026-11-07-x.md",
			strings.SplitAfter(strings.Replace(validEntry, "**Section:** Fixed", "**Section:** Improved", 1),
				"**Section:** Improved\n")[0],
			[]string{"Section", "body"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseEntry(tt.file, tt.content)
			if err == nil {
				t.Fatal("ParseEntry accepted an invalid entry")
			}
			msg := err.Error()
			if !strings.Contains(msg, tt.file) {
				t.Errorf("error %q does not name the file %s", msg, tt.file)
			}
			for _, w := range tt.want {
				if !strings.Contains(msg, w) {
					t.Errorf("error %q does not mention %q", msg, w)
				}
			}
		})
	}
}

func TestParseEntryCRLF(t *testing.T) {
	crlf := strings.ReplaceAll(validEntry, "\n", "\r\n")
	got, err := ParseEntry("2026-11-07-restart-expired-sign-in.md", crlf)
	if err != nil {
		t.Fatalf("ParseEntry: %v", err)
	}
	want, _ := ParseEntry("2026-11-07-restart-expired-sign-in.md", validEntry)
	if got != want {
		t.Fatalf("CRLF entry = %+v\nwant %+v", got, want)
	}
	if strings.Contains(got.Title+got.Body, "\r") {
		t.Fatal("parsed entry still contains \\r")
	}
}

func goldenEntries() []Entry {
	return []Entry{
		{File: "2026-11-07-b.md", Title: "B", Date: "2026-11-07", Section: Fixed, Body: "Fix b."},
		{File: "2026-11-07-a.md", Title: "A", Date: "2026-11-07", Section: Fixed,
			Body: "Fix a, first paragraph\nwrapped onto a second line.\n\nSecond paragraph:\n\n- nested item"},
		{File: "2026-11-06-c.md", Title: "C", Date: "2026-11-06", Section: Added, Body: "Feature c."},
	}
}

func TestCutGolden(t *testing.T) {
	got, err := Cut(readFixture(t, "CHANGELOG.in.md"), goldenEntries(), "0.53.0", "2026-11-07", exampleURL)
	if err != nil {
		t.Fatalf("Cut: %v", err)
	}
	if want := readFixture(t, "CHANGELOG.want.md"); got != want {
		t.Fatalf("Cut output differs from testdata/cut/CHANGELOG.want.md\n--- got ---\n%s", got)
	}
}

func TestCutFoldsUnreleasedBullets(t *testing.T) {
	in := strings.Replace(readFixture(t, "CHANGELOG.in.md"), "## [Unreleased]\n\n",
		"## [Unreleased]\n\n### Fixed\n\n- Legacy fix\n  wrapped.\n\n### Security\n\n- Legacy security note.\n\n", 1)

	got, err := Cut(in, goldenEntries()[:1], "0.53.0", "2026-11-07", exampleURL)
	if err != nil {
		t.Fatalf("Cut: %v", err)
	}
	for _, want := range []string{
		"## [Unreleased]\n\n## [0.53.0] - 2026-11-07\n\n### Fixed\n\n",
		"### Fixed\n\n- Legacy fix\n  wrapped.\n\n- Fix b.\n\n### Security\n\n- Legacy security note.\n\n## [0.52.0] - 2026-11-06",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("Cut output lacks %q\n--- got ---\n%s", want, got)
		}
	}

	// Bullets alone, with no entry files, are still a release.
	if _, err := Cut(in, nil, "0.53.0", "2026-11-07", exampleURL); err != nil {
		t.Fatalf("Cut with only [Unreleased] bullets: %v", err)
	}
}

func TestCutRejects(t *testing.T) {
	in := readFixture(t, "CHANGELOG.in.md")
	tests := []struct {
		name      string
		changelog string
		entries   []Entry
		version   string
		date      string
		want      string
	}{
		{"nothing to release", in, nil, "0.53.0", "2026-11-07", "nothing to release"},
		{"same version", in, goldenEntries(), "0.52.0", "2026-11-07", "not newer than 0.52.0"},
		{"older version", in, goldenEntries(), "0.51.9", "2026-11-07", "not newer than 0.52.0"},
		{"v prefix", in, goldenEntries(), "v0.53.0", "2026-11-07", "X.Y.Z"},
		{"leading zero", in, goldenEntries(), "0.053.0", "2026-11-07", "X.Y.Z"},
		{"older than a legacy leading-zero heading",
			strings.Replace(in, "## [0.52.0] - 2026-11-06", "## [0.053.0] - 2026-11-06", 1),
			goldenEntries(), "0.52.1", "2026-11-07", "not newer than 0.053.0"},
		{"bad date", in, goldenEntries(), "0.53.0", "07.11.2026", "YYYY-MM-DD"},
		{"text outside a subsection",
			strings.Replace(in, "## [Unreleased]\n\n", "## [Unreleased]\n\n- stray bullet\n\n", 1),
			goldenEntries(), "0.53.0", "2026-11-07", "subsection"},
		{"unknown subsection",
			strings.Replace(in, "## [Unreleased]\n\n", "## [Unreleased]\n\n### Improved\n\n- x\n\n", 1),
			goldenEntries(), "0.53.0", "2026-11-07", "Improved"},
		{"no unreleased heading",
			strings.Replace(in, "## [Unreleased]\n\n", "", 1),
			goldenEntries(), "0.53.0", "2026-11-07", "[Unreleased]"},
		{"no link block",
			in[:strings.Index(in, "[0.52.0]: ")],
			goldenEntries(), "0.53.0", "2026-11-07", "link"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Cut(tt.changelog, tt.entries, tt.version, tt.date, exampleURL)
			if err == nil {
				t.Fatal("Cut succeeded, want an error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error %q does not mention %q", err, tt.want)
			}
		})
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func entryText(title, date string, section Section, body string) string {
	return "# Changelog — " + title + "\n\n**Date:** " + date + "\n**Section:** " + string(section) + "\n\n" + body + "\n"
}

func TestRun(t *testing.T) {
	root := t.TempDir()
	in := readFixture(t, "CHANGELOG.in.md")
	writeFile(t, filepath.Join(root, "CHANGELOG.md"), in)
	dir := filepath.Join(root, "docs", "changelog")
	writeFile(t, filepath.Join(dir, "README.md"), "# docs/changelog\n")
	writeFile(t, filepath.Join(dir, "notes.txt"), "not an entry\n")
	writeFile(t, filepath.Join(dir, "2026-11-07-a.md"), entryText("Entry A", "2026-11-07", Fixed, "Fix a."))
	writeFile(t, filepath.Join(dir, "2026-11-07-b.md"), entryText("Entry B", "2026-11-07", Added, "Feature b."))

	summary, err := Run(root, "0.53.0", "2026-11-07", tags("v0.52.0"))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	for _, want := range []string{
		"## [0.53.0] - 2026-11-07",
		"### Added\n- Entry B (docs/changelog/2026-11-07-b.md)",
		"### Fixed\n- Entry A (docs/changelog/2026-11-07-a.md)",
	} {
		if !strings.Contains(summary, want) {
			t.Errorf("summary lacks %q\n--- summary ---\n%s", want, summary)
		}
	}
	changelog, err := os.ReadFile(filepath.Join(root, "CHANGELOG.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(changelog), "## [0.53.0] - 2026-11-07\n\n### Added\n\n- Feature b.\n\n### Fixed\n\n- Fix a.\n") {
		t.Fatalf("CHANGELOG.md lacks the new section:\n%s", changelog)
	}
	for _, gone := range []string{"2026-11-07-a.md", "2026-11-07-b.md"} {
		if _, err := os.Stat(filepath.Join(dir, gone)); !os.IsNotExist(err) {
			t.Errorf("%s was not deleted (stat err: %v)", gone, err)
		}
	}
	for _, kept := range []string{"README.md", "notes.txt"} {
		if _, err := os.Stat(filepath.Join(dir, kept)); err != nil {
			t.Errorf("%s was removed: %v", kept, err)
		}
	}

	// A second cut has nothing left and changes nothing.
	if _, err := Run(root, "0.54.0", "2026-11-08", tags("v0.52.0", "v0.53.0")); err == nil || !strings.Contains(err.Error(), "nothing to release") {
		t.Fatalf("second Run error = %v, want nothing to release", err)
	}
	again, _ := os.ReadFile(filepath.Join(root, "CHANGELOG.md"))
	if string(again) != string(changelog) {
		t.Fatal("a failed Run rewrote CHANGELOG.md")
	}
}

func TestRunMalformedEntryChangesNothing(t *testing.T) {
	root := t.TempDir()
	in := readFixture(t, "CHANGELOG.in.md")
	writeFile(t, filepath.Join(root, "CHANGELOG.md"), in)
	dir := filepath.Join(root, "docs", "changelog")
	good := entryText("Good", "2026-11-07", Fixed, "Fine.")
	bad := entryText("Bad", "2026-11-07", "Improved", "Not fine.")
	writeFile(t, filepath.Join(dir, "2026-11-07-good.md"), good)
	writeFile(t, filepath.Join(dir, "2026-11-07-bad.md"), bad)

	_, err := Run(root, "0.53.0", "2026-11-07", tags("v0.52.0"))
	if err == nil || !strings.Contains(err.Error(), "2026-11-07-bad.md") {
		t.Fatalf("Run error = %v, want one naming 2026-11-07-bad.md", err)
	}
	if got, _ := os.ReadFile(filepath.Join(root, "CHANGELOG.md")); string(got) != in {
		t.Fatal("Run rewrote CHANGELOG.md despite a malformed entry")
	}
	for name, want := range map[string]string{"2026-11-07-good.md": good, "2026-11-07-bad.md": bad} {
		if got, err := os.ReadFile(filepath.Join(dir, name)); err != nil || string(got) != want {
			t.Errorf("%s changed or vanished (err %v)", name, err)
		}
	}
}

func TestRunWithoutEntryDirectory(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "CHANGELOG.md"), readFixture(t, "CHANGELOG.in.md"))
	if _, err := Run(root, "0.53.0", "2026-11-07", tags("v0.52.0")); err == nil || !strings.Contains(err.Error(), "nothing to release") {
		t.Fatalf("Run error = %v, want nothing to release", err)
	}
}

// tags fakes the remote tag lookup: only the given tags exist.
func tags(existing ...string) TagExists {
	return func(tag string) (bool, error) { return slices.Contains(existing, tag), nil }
}

// newRunRepo writes the fixture CHANGELOG.md and one valid Fixed entry.
func newRunRepo(t *testing.T) (root, changelog, entryPath string) {
	t.Helper()
	root = t.TempDir()
	changelog = readFixture(t, "CHANGELOG.in.md")
	writeFile(t, filepath.Join(root, "CHANGELOG.md"), changelog)
	entryPath = filepath.Join(root, "docs", "changelog", "2026-11-07-a.md")
	writeFile(t, entryPath, entryText("Entry A", "2026-11-07", Fixed, "Fix a."))
	return root, changelog, entryPath
}

func assertUntouched(t *testing.T, root, changelog, entryPath string) {
	t.Helper()
	if got, _ := os.ReadFile(filepath.Join(root, "CHANGELOG.md")); string(got) != changelog {
		t.Error("CHANGELOG.md was rewritten")
	}
	if _, err := os.Stat(entryPath); err != nil {
		t.Errorf("entry was removed: %v", err)
	}
}

func TestRunRefusesUntaggedPrevious(t *testing.T) {
	root, changelog, entry := newRunRepo(t)
	_, err := Run(root, "0.53.0", "2026-11-07", tags())
	if err == nil || !strings.Contains(err.Error(), "v0.52.0") || !strings.Contains(err.Error(), "not tagged") {
		t.Fatalf("Run error = %v, want one saying v0.52.0 is not tagged", err)
	}
	assertUntouched(t, root, changelog, entry)
}

func TestRunRefusesExistingTag(t *testing.T) {
	root, changelog, entry := newRunRepo(t)
	_, err := Run(root, "0.53.0", "2026-11-07", tags("v0.52.0", "v0.53.0"))
	if err == nil || !strings.Contains(err.Error(), "v0.53.0") || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("Run error = %v, want one saying v0.53.0 already exists", err)
	}
	assertUntouched(t, root, changelog, entry)
}

func TestRunTagLookupError(t *testing.T) {
	root, changelog, entry := newRunRepo(t)
	failing := func(string) (bool, error) { return false, errors.New("network down") }
	if _, err := Run(root, "0.53.0", "2026-11-07", failing); err == nil || !strings.Contains(err.Error(), "network down") {
		t.Fatalf("Run error = %v, want the lookup error", err)
	}
	assertUntouched(t, root, changelog, entry)
}

func TestRunThatCannotMoveEntriesChangesNothing(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	root, changelog, entry := newRunRepo(t)
	untracked := filepath.Join(filepath.Dir(entry), "2026-11-07-b.md")
	writeFile(t, untracked, entryText("Entry B", "2026-11-07", Added, "Feature b."))
	dir := filepath.Dir(entry)
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })

	if _, err := Run(root, "0.53.0", "2026-11-07", tags("v0.52.0")); err == nil {
		t.Fatal("Run succeeded in a read-only entry directory")
	}
	assertUntouched(t, root, changelog, entry)
	if _, err := os.Stat(untracked); err != nil {
		t.Fatalf("second entry was removed: %v", err)
	}
}

func TestRunLeavesNoStagingDirectory(t *testing.T) {
	root, _, entry := newRunRepo(t)
	if _, err := Run(root, "0.53.0", "2026-11-07", tags("v0.52.0")); err != nil {
		t.Fatal(err)
	}
	des, err := os.ReadDir(filepath.Dir(entry))
	if err != nil {
		t.Fatal(err)
	}
	if len(des) != 0 {
		t.Fatalf("entry directory still holds %v after the cut", des)
	}
}

func TestRunRefusesSymlinks(t *testing.T) {
	t.Run("entry directory", func(t *testing.T) {
		root := t.TempDir()
		changelog := readFixture(t, "CHANGELOG.in.md")
		writeFile(t, filepath.Join(root, "CHANGELOG.md"), changelog)
		outside := filepath.Join(t.TempDir(), "elsewhere")
		external := filepath.Join(outside, "2026-11-07-a.md")
		writeFile(t, external, entryText("Outside", "2026-11-07", Fixed, "Outside."))
		if err := os.MkdirAll(filepath.Join(root, "docs"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(outside, filepath.Join(root, "docs", "changelog")); err != nil {
			t.Fatal(err)
		}
		if _, err := Run(root, "0.53.0", "2026-11-07", tags("v0.52.0")); err == nil || !strings.Contains(err.Error(), "symlink") {
			t.Fatalf("Run error = %v, want a symlink refusal", err)
		}
		assertUntouched(t, root, changelog, external)
	})
	t.Run("ancestor directory", func(t *testing.T) {
		root := t.TempDir()
		changelog := readFixture(t, "CHANGELOG.in.md")
		writeFile(t, filepath.Join(root, "CHANGELOG.md"), changelog)
		outside := filepath.Join(t.TempDir(), "docs")
		external := filepath.Join(outside, "changelog", "2026-11-07-a.md")
		writeFile(t, external, entryText("Outside", "2026-11-07", Fixed, "Outside."))
		if err := os.Symlink(outside, filepath.Join(root, "docs")); err != nil {
			t.Fatal(err)
		}
		if _, err := Run(root, "0.53.0", "2026-11-07", tags("v0.52.0")); err == nil || !strings.Contains(err.Error(), "outside") {
			t.Fatalf("Run error = %v, want a refusal of an entry directory outside the repository", err)
		}
		assertUntouched(t, root, changelog, external)
	})
	t.Run("entry file", func(t *testing.T) {
		root, changelog, entry := newRunRepo(t)
		target := filepath.Join(t.TempDir(), "target.md")
		writeFile(t, target, entryText("Outside", "2026-11-07", Fixed, "Outside."))
		link := filepath.Join(filepath.Dir(entry), "2026-11-07-link.md")
		if err := os.Symlink(target, link); err != nil {
			t.Fatal(err)
		}
		if _, err := Run(root, "0.53.0", "2026-11-07", tags("v0.52.0")); err == nil || !strings.Contains(err.Error(), "symlink") {
			t.Fatalf("Run error = %v, want a symlink refusal", err)
		}
		assertUntouched(t, root, changelog, entry)
		if _, err := os.Stat(target); err != nil {
			t.Fatalf("symlink target was removed: %v", err)
		}
	})
}

func TestLsRemoteHasTag(t *testing.T) {
	tests := []struct {
		name, output string
		want         bool
	}{
		{"exact", "1111\trefs/tags/v0.52.0\n", true},
		{"annotated", "1111\trefs/tags/v0.52.0\n2222\trefs/tags/v0.52.0^{}\n", true},
		{"suffix match only", "1111\trefs/tags/archive/refs/tags/v0.52.0\n", false},
		{"other tag", "1111\trefs/tags/v0.52.01\n", false},
		{"empty", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := LsRemoteHasTag(tt.output, "v0.52.0"); got != tt.want {
				t.Fatalf("LsRemoteHasTag(%q) = %v, want %v", tt.output, got, tt.want)
			}
		})
	}
}
