// Package changelogcut folds pending changelog entries into a release.
//
// Every PR records its change as its own file under docs/changelog/ instead of
// adding a bullet to CHANGELOG.md, so parallel PRs never conflict in one
// shared section. The release PR runs `make changelog-cut RELEASE=X.Y.Z`,
// which moves every pending entry into a new version section of CHANGELOG.md
// and deletes the entry files. CHANGELOG.md keeps the grammar pinned in
// docs/changelog-spec.md, which the release automation (`/operator release`)
// parses; the entry grammar is pinned in docs/changelog/README.md.
package changelogcut

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// RepoURL is the base of the compare links the cut writes. It is the only
// line that differs between the copies of this package in tiden-cli and
// tiden-mcp-server.
const RepoURL = "https://github.com/qase-tms/tiden-mcp-server"

// EntryDir holds the pending entries, relative to the repository root.
const EntryDir = "docs/changelog"

// Section is a Keep a Changelog subsection.
type Section string

const (
	Added      Section = "Added"
	Changed    Section = "Changed"
	Deprecated Section = "Deprecated"
	Removed    Section = "Removed"
	Fixed      Section = "Fixed"
	Security   Section = "Security"
)

// Sections lists every section in the order a cut writes them.
var Sections = []Section{Added, Changed, Deprecated, Removed, Fixed, Security}

// Entry is one pending change: one bullet of a future version section.
type Entry struct {
	File    string // base name, e.g. "2026-11-07-restart-expired-sign-in.md"
	Title   string
	Date    string // YYYY-MM-DD
	Section Section
	Body    string // the bullet text: LF line endings, trimmed
}

var (
	entryFileRE    = regexp.MustCompile(`^(\d{4}-\d{2}-\d{2})-[a-z0-9]+(?:-[a-z0-9]+)*\.md$`)
	titleRE        = regexp.MustCompile(`^# Changelog — (.*)$`)
	dateLineRE     = regexp.MustCompile(`^\*\*Date:\*\* (\d{4}-\d{2}-\d{2})$`)
	sectionLineRE  = regexp.MustCompile(`^\*\*Section:\*\* (.*)$`)
	versionRE      = regexp.MustCompile(`^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)$`)
	versionHeadRE  = regexp.MustCompile(`^## \[(\d+\.\d+\.\d+)\] - \d{4}-\d{2}-\d{2}$`)
	linkRefLineRE  = regexp.MustCompile(`^\[\d+\.\d+\.\d+\]: `)
	linkRefDefRE   = regexp.MustCompile(`^\s*\[[^\]]+\]:(\s|$)`)
	anyVersionRE   = regexp.MustCompile(`^(\d+)\.(\d+)\.(\d+)$`)
	sectionNameSet = func() map[Section]bool {
		m := make(map[Section]bool, len(Sections))
		for _, s := range Sections {
			m[s] = true
		}
		return m
	}()
)

const unreleasedHeading = "## [Unreleased]"

// ParseEntry parses one entry file. The error names the file and lists every
// grammar rule the entry breaks, not only the first.
func ParseEntry(file, content string) (Entry, error) {
	var problems []string
	e := Entry{File: file}

	fileDate := ""
	if m := entryFileRE.FindStringSubmatch(file); m == nil {
		problems = append(problems, "filename must be YYYY-MM-DD-<kebab-slug>.md")
	} else if _, err := time.Parse(time.DateOnly, m[1]); err != nil {
		problems = append(problems, fmt.Sprintf("filename date %s is not a valid date", m[1]))
	} else {
		fileDate = m[1]
	}

	// A bare CR is a line break to a Markdown renderer, so it is one here too:
	// otherwise a heading could hide behind it inside a bullet.
	content = strings.ReplaceAll(strings.ReplaceAll(content, "\r\n", "\n"), "\r", "\n")
	lines := strings.Split(content, "\n")
	line := func(i int) string {
		if i < len(lines) {
			return lines[i]
		}
		return ""
	}

	if m := titleRE.FindStringSubmatch(line(0)); m == nil || strings.TrimSpace(m[1]) == "" {
		problems = append(problems, `title: line 1 must be "# Changelog — <Title>" (em dash) with a non-empty title`)
	} else {
		e.Title = strings.TrimSpace(m[1])
	}
	if line(1) != "" {
		problems = append(problems, "line 2 must be blank")
	}
	if m := dateLineRE.FindStringSubmatch(line(2)); m == nil {
		problems = append(problems, `Date: line 3 must be "**Date:** YYYY-MM-DD"`)
	} else if fileDate != "" && m[1] != fileDate {
		problems = append(problems, fmt.Sprintf("Date: %s does not match the filename date %s", m[1], fileDate))
	} else {
		e.Date = m[1]
	}
	if m := sectionLineRE.FindStringSubmatch(line(3)); m == nil || !sectionNameSet[Section(m[1])] {
		problems = append(problems, `Section: line 4 must be "**Section:** " followed by Added, Changed, Deprecated, Removed, Fixed or Security`)
	} else {
		e.Section = Section(m[1])
	}
	if line(4) != "" {
		problems = append(problems, "line 5 must be blank")
	}

	var body []string
	if len(lines) > 5 {
		body = lines[5:]
	}
	e.Body = strings.Trim(strings.Join(body, "\n"), "\n")
	if strings.TrimSpace(e.Body) == "" {
		problems = append(problems, "body: the bullet text after line 5 is empty")
	}
	for i, l := range body {
		if strings.HasPrefix(strings.TrimLeft(l, " \t"), "#") {
			problems = append(problems, fmt.Sprintf(
				"body: line %d starts with #, which would become a heading inside CHANGELOG.md", i+6))
		}
		if linkRefDefRE.MatchString(l) {
			problems = append(problems, fmt.Sprintf(
				"body: line %d is a link reference definition, which applies to all of CHANGELOG.md "+
					"and can override a version's compare link; use an inline [text](url) link", i+6))
		}
	}

	if len(problems) > 0 {
		return Entry{}, fmt.Errorf("%s: %s", file, strings.Join(problems, "; "))
	}
	return e, nil
}

// unreleased locates the [Unreleased] section and the newest version heading
// below it.
type unreleased struct {
	headingEnd int    // line index of "## [Unreleased]"
	prevLine   int    // line index of the newest "## [X.Y.Z] - date" heading
	prev       string // its version
	bullets    map[Section]string
}

func parseUnreleased(lines []string) (unreleased, error) {
	u := unreleased{headingEnd: -1, prevLine: -1, bullets: map[Section]string{}}
	for i, l := range lines {
		if l == unreleasedHeading {
			u.headingEnd = i
			break
		}
	}
	if u.headingEnd < 0 {
		return u, errors.New("CHANGELOG.md has no " + unreleasedHeading + " heading")
	}
	for i := u.headingEnd + 1; i < len(lines); i++ {
		if strings.HasPrefix(lines[i], "## ") {
			m := versionHeadRE.FindStringSubmatch(lines[i])
			if m == nil {
				return u, fmt.Errorf("CHANGELOG.md: %q below %s is not a version heading", lines[i], unreleasedHeading)
			}
			u.prevLine, u.prev = i, m[1]
			break
		}
	}
	if u.prevLine < 0 {
		return u, errors.New("CHANGELOG.md has no version heading below " + unreleasedHeading)
	}

	// Bullets an older-style PR added under [Unreleased] are carried into the
	// release with the entries, so nothing merged is ever dropped.
	var current Section
	blocks := map[Section][]string{}
	for _, l := range lines[u.headingEnd+1 : u.prevLine] {
		if strings.HasPrefix(l, "### ") {
			current = Section(strings.TrimPrefix(l, "### "))
			if !sectionNameSet[current] {
				return u, fmt.Errorf("CHANGELOG.md: unsupported subsection %q under %s", l, unreleasedHeading)
			}
			continue
		}
		if current == "" {
			if strings.TrimSpace(l) != "" {
				return u, fmt.Errorf("CHANGELOG.md: %q under %s is outside a ### subsection", l, unreleasedHeading)
			}
			continue
		}
		blocks[current] = append(blocks[current], l)
	}
	for s, b := range blocks {
		if text := strings.Trim(strings.Join(b, "\n"), "\n"); strings.TrimSpace(text) != "" {
			u.bullets[s] = text
		}
	}
	return u, nil
}

// parseVersion parses a version as the cut writes it: X.Y.Z, no leading zeros.
func parseVersion(v string) ([3]int, bool) {
	if !versionRE.MatchString(v) {
		return [3]int{}, false
	}
	return parseNumbers(v)
}

// parseNumbers parses any X.Y.Z, including the leading zeros an older
// heading may carry, so comparisons against it stay numeric.
func parseNumbers(v string) ([3]int, bool) {
	m := anyVersionRE.FindStringSubmatch(v)
	if m == nil {
		return [3]int{}, false
	}
	var out [3]int
	for i := range out {
		n, err := strconv.Atoi(m[i+1])
		if err != nil {
			return [3]int{}, false
		}
		out[i] = n
	}
	return out, true
}

func newer(a, b [3]int) bool {
	for i := range a {
		if a[i] != b[i] {
			return a[i] > b[i]
		}
	}
	return false
}

func renderBullet(body string) string {
	lines := strings.Split(body, "\n")
	for i, l := range lines {
		switch {
		case i == 0:
			lines[i] = "- " + l
		case l != "":
			lines[i] = "  " + l
		}
	}
	return strings.Join(lines, "\n")
}

func sortedEntries(entries []Entry) []Entry {
	out := append([]Entry(nil), entries...)
	sort.SliceStable(out, func(i, j int) bool { return out[i].File < out[j].File })
	return out
}

// Cut returns changelog with a new "## [version] - date" section holding the
// entries and any bullets left under [Unreleased], which ends up empty. It
// adds the version's compare link at the top of the link block. repoURL is
// the repository the compare links point at.
func Cut(changelog string, entries []Entry, version, date, repoURL string) (string, error) {
	want, ok := parseVersion(version)
	if !ok {
		return "", fmt.Errorf("version %q must be X.Y.Z (no v prefix)", version)
	}
	if _, err := time.Parse(time.DateOnly, date); err != nil {
		return "", fmt.Errorf("date %q must be YYYY-MM-DD", date)
	}

	lines := strings.Split(changelog, "\n")
	u, err := parseUnreleased(lines)
	if err != nil {
		return "", err
	}
	prev, ok := parseNumbers(u.prev)
	if !ok {
		return "", fmt.Errorf("CHANGELOG.md: cannot read the newest version %q", u.prev)
	}
	if !newer(want, prev) {
		return "", fmt.Errorf("version %s is not newer than %s, the newest version in CHANGELOG.md", version, u.prev)
	}
	if len(entries) == 0 && len(u.bullets) == 0 {
		return "", fmt.Errorf("nothing to release: %s/ holds no entries and %s is empty", EntryDir, unreleasedHeading)
	}

	linkAt := -1
	for i, l := range lines {
		if linkRefLineRE.MatchString(l) {
			linkAt = i
			break
		}
	}
	if linkAt < 0 {
		return "", errors.New("CHANGELOG.md has no link reference block to add the new version's link to")
	}

	bySection := map[Section][]string{}
	for _, s := range Sections {
		if b, ok := u.bullets[s]; ok {
			bySection[s] = append(bySection[s], b)
		}
	}
	for _, e := range sortedEntries(entries) {
		bySection[e.Section] = append(bySection[e.Section], renderBullet(e.Body))
	}

	var block []string
	block = append(block, fmt.Sprintf("## [%s] - %s", version, date), "")
	for _, s := range Sections {
		if len(bySection[s]) == 0 {
			continue
		}
		block = append(block, "### "+string(s), "", strings.Join(bySection[s], "\n\n"), "")
	}

	var out []string
	out = append(out, lines[:u.headingEnd+1]...)
	out = append(out, "")
	out = append(out, block...)
	out = append(out, lines[u.prevLine:linkAt]...)
	out = append(out, fmt.Sprintf("[%s]: %s/compare/v%s...v%s", version, repoURL, u.prev, version))
	out = append(out, lines[linkAt:]...)
	return strings.Join(out, "\n"), nil
}

func entryFiles(dir string) ([]string, error) {
	des, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var names []string
	for _, de := range des {
		name := de.Name()
		if de.IsDir() || name == "README.md" || !strings.HasSuffix(name, ".md") {
			continue
		}
		if de.Type()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("%s is a symlink; entries must be regular files", filepath.Join(dir, name))
		}
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}

// TagExists reports whether a git tag exists on the remote.
type TagExists func(tag string) (bool, error)

// Run cuts the release in the repository at root: it folds every entry under
// docs/changelog/ (README.md and non-.md files excepted) and any bullets
// under [Unreleased] into CHANGELOG.md, deletes the folded entry files, and
// returns a summary for the release PR body.
//
// It refuses to cut when the target tag already exists or the previous
// version is not tagged yet: a new heading over an untagged one would hide
// that release's notes from the release automation, which only reads the
// newest heading. Every check runs before anything is written. If an entry
// cannot be deleted after CHANGELOG.md is written, the error says so.
func Run(root, version, date string, tagExists TagExists) (string, error) {
	path := filepath.Join(root, "CHANGELOG.md")
	dir := filepath.Join(root, EntryDir)
	for _, p := range []string{path, dir} {
		info, err := os.Lstat(p)
		if err == nil && info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("%s is a symlink; the cut only works on regular files in the repository", p)
		}
	}

	if err := insideRoot(root, dir); err != nil {
		return "", err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}

	names, err := entryFiles(dir)
	if err != nil {
		return "", err
	}
	var entries []Entry
	var errs []error
	for _, name := range names {
		content, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return "", err
		}
		e, err := ParseEntry(name, string(content))
		if err != nil {
			errs = append(errs, err)
			continue
		}
		entries = append(entries, e)
	}
	if len(errs) > 0 {
		return "", fmt.Errorf("malformed entries (grammar: %s/README.md):\n%w", EntryDir, errors.Join(errs...))
	}

	changelog := string(data)
	out, err := Cut(changelog, entries, version, date, RepoURL)
	if err != nil {
		return "", err
	}
	u, err := parseUnreleased(strings.Split(changelog, "\n"))
	if err != nil {
		return "", err
	}

	if exists, err := tagExists("v" + version); err != nil {
		return "", fmt.Errorf("checking tag v%s: %w", version, err)
	} else if exists {
		return "", fmt.Errorf("tag v%s already exists: pick the next version", version)
	}
	if exists, err := tagExists("v" + u.prev); err != nil {
		return "", fmt.Errorf("checking tag v%s: %w", u.prev, err)
	} else if !exists {
		return "", fmt.Errorf("%s is not tagged yet: release it (/operator release) before cutting %s, "+
			"or its notes stop being the ones the release automation reads", "v"+u.prev, version)
	}

	// Move the folded entries aside first: if any move fails, the others go
	// back and nothing is written, so a failed cut never needs a git undo
	// (which would also lose an entry that was never committed).
	staging, err := os.MkdirTemp(dir, ".cut-")
	if err != nil {
		return "", fmt.Errorf("cannot stage the entries for removal: %w", err)
	}
	var moved []string
	restore := func() {
		for _, name := range moved {
			_ = os.Rename(filepath.Join(staging, name), filepath.Join(dir, name))
		}
		_ = os.Remove(staging)
	}
	for _, e := range entries {
		if err := os.Rename(filepath.Join(dir, e.File), filepath.Join(staging, e.File)); err != nil {
			restore()
			return "", fmt.Errorf("cannot stage %s for removal, nothing changed: %w", e.File, err)
		}
		moved = append(moved, e.File)
	}
	if err := writeAtomic(path, []byte(out), info.Mode().Perm()); err != nil {
		restore()
		return "", fmt.Errorf("writing CHANGELOG.md, nothing changed: %w", err)
	}
	if err := os.RemoveAll(staging); err != nil {
		return "", fmt.Errorf("CHANGELOG.md now holds %s, but the folded entries are still in %s: delete that directory (%w)",
			version, staging, err)
	}
	return summary(version, date, entries, u.bullets), nil
}

// insideRoot refuses an entry directory that resolves outside the
// repository through a symlinked ancestor.
func insideRoot(root, dir string) error {
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return err
	}
	realDir, err := filepath.EvalSymlinks(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if rel, err := filepath.Rel(realRoot, realDir); err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("%s resolves outside the repository (%s): the cut only works on files in the repository", dir, realDir)
	}
	return nil
}

// LsRemoteHasTag reports whether `git ls-remote --tags` output lists exactly
// refs/tags/<tag>. Git matches the pattern by suffix, so the output can also
// list a tag like refs/tags/archive/refs/tags/<tag>.
func LsRemoteHasTag(output, tag string) bool {
	want := "refs/tags/" + tag
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && (fields[1] == want || fields[1] == want+"^{}") {
			return true
		}
	}
	return false
}

// writeAtomic replaces path through a temporary file in the same directory,
// so a failed write never leaves a truncated CHANGELOG.md.
func writeAtomic(path string, data []byte, perm os.FileMode) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer func() { _ = os.Remove(name) }()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(perm); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}

func summary(version, date string, entries []Entry, bullets map[Section]string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "## [%s] - %s\n", version, date)
	sorted := sortedEntries(entries)
	for _, s := range Sections {
		var items []string
		for _, l := range strings.Split(bullets[s], "\n") {
			if strings.HasPrefix(l, "- ") {
				items = append(items, fmt.Sprintf("- %s (carried over from %s)", strings.TrimPrefix(l, "- "), unreleasedHeading))
			}
		}
		for _, e := range sorted {
			if e.Section == s {
				items = append(items, fmt.Sprintf("- %s (%s/%s)", e.Title, EntryDir, e.File))
			}
		}
		if len(items) == 0 {
			continue
		}
		fmt.Fprintf(&b, "### %s\n%s\n", s, strings.Join(items, "\n"))
	}
	return b.String()
}
