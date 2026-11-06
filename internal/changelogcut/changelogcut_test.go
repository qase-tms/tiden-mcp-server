package changelogcut

import (
	"os"
	"path/filepath"
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

	summary, err := Run(root, "0.53.0", "2026-11-07")
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
	if _, err := Run(root, "0.54.0", "2026-11-08"); err == nil || !strings.Contains(err.Error(), "nothing to release") {
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

	_, err := Run(root, "0.53.0", "2026-11-07")
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
	if _, err := Run(root, "0.53.0", "2026-11-07"); err == nil || !strings.Contains(err.Error(), "nothing to release") {
		t.Fatalf("Run error = %v, want nothing to release", err)
	}
}
