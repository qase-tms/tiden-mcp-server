// Package changelogcontract falsifies the CHANGELOG.md grammar pinned in
// docs/changelog-spec.md. There is no importable parser in this repo — the
// release automation ("/operator release") reads the file externally — so
// this package is the executable contract: a small reference parser lives
// here, purpose-built to be wrong in the ways a broken CHANGELOG.md would
// break the release automation, and the repo's real CHANGELOG.md is checked
// against it as a sanity subcase.
package changelogcontract

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"
)

// unreleasedHeading is the one heading text the spec allows no variants of
// (file grammar rule 2: "## [Unreleased]" is always present and always first).
const unreleasedHeading = "## [Unreleased]"

// versionHeadingRE is file grammar rule 3, verbatim: "## [X.Y.Z] - YYYY-MM-DD",
// semver without a "v" prefix, one space each side of the hyphen. Anything
// starting with "## [" that does not match this is a malformed heading, not
// a version to silently coerce.
var versionHeadingRE = regexp.MustCompile(`^## \[(\d+\.\d+\.\d+)\] - \d{4}-\d{2}-\d{2}$`)

// heading is one "## "-prefixed line, in file order.
type heading struct {
	line      int // 0-based index into the split-by-"\n" line slice
	raw       string
	version   string // "" unless raw matched versionHeadingRE
	malformed bool   // raw starts with "## [" but is not the Unreleased heading and does not match versionHeadingRE
}

// parseHeadings collects every "## " line without judging file-level
// structure (Unreleased-first, no malformed heading) — that judgment belongs
// to derivePending, which sees the headings in order.
func parseHeadings(lines []string) []heading {
	var hs []heading
	for i, l := range lines {
		switch {
		case l == unreleasedHeading:
			hs = append(hs, heading{line: i, raw: l})
		case strings.HasPrefix(l, "## "):
			if m := versionHeadingRE.FindStringSubmatch(l); m != nil {
				hs = append(hs, heading{line: i, raw: l, version: m[1]})
			} else {
				hs = append(hs, heading{line: i, raw: l, malformed: true})
			}
		}
	}
	return hs
}

var (
	// errMissingUnreleased is file grammar rule 2.
	errMissingUnreleased = errors.New("changelog: `## [Unreleased]` must be present and be the first version heading")
	// errMalformedHeading is file grammar rule 3: a "## [" line that isn't a
	// well-formed version heading must be rejected, never treated as pending.
	errMalformedHeading = errors.New("changelog: malformed version heading")
)

// derivePending implements the "How the automation reads this file" rule:
// pending version = the newest version heading's X.Y.Z (the first heading
// below Unreleased), and only if its vX.Y.Z tag does not exist yet. Older
// untagged headings are never pending — derivePending doesn't even look at
// them, by construction: it only ever inspects hs[1].
//
// tagExists is injected so this stays a pure string→string function with no
// git invocation, per this package's own constraint of not touching git.
func derivePending(content string, tagExists func(version string) bool) (pending string, err error) {
	hs := parseHeadings(strings.Split(content, "\n"))
	if len(hs) == 0 || hs[0].raw != unreleasedHeading {
		return "", errMissingUnreleased
	}
	if len(hs) < 2 {
		return "", nil // no version heading below Unreleased yet: nothing pending, nothing malformed either
	}
	newest := hs[1]
	if newest.malformed {
		return "", fmt.Errorf("%w: %q", errMalformedHeading, newest.raw)
	}
	if tagExists(newest.version) {
		return "", nil // release-flow invariant: tag already cut, nothing left to release
	}
	return newest.version, nil
}

// extractSection returns the body of the version heading matching `version`:
// file grammar rule 4, everything from the heading line down to the next
// "## " heading, exclusive, or EOF if there is none.
//
// Ambiguity note: the bottom-of-file link-reference block (rule 5) is never
// introduced by a "## " line, so a literal reading of rule 4 would fold it
// into the LAST version's body. The spec lists it as a separate structural
// element (rule 5, not part of rule 4's body), so this reference treats the
// two rules as governing disjoint regions rather than resolving the overlap
// inside extractSection — this contract only exercises heading-to-heading /
// heading-to-EOF slicing, and its "last section" subcase deliberately uses an
// input with no trailing link block so that reading is never invoked either
// way. Whether the real file's last section body includes its link block is
// a release-notes-rendering question outside this contract's two case names.
func extractSection(content, version string) (body string, found bool) {
	lines := strings.Split(content, "\n")
	hs := parseHeadings(lines)
	start := -1
	end := len(lines)
	for i, h := range hs {
		if h.version == version {
			start = h.line + 1
			if i+1 < len(hs) {
				end = hs[i+1].line
			}
			break
		}
	}
	if start == -1 {
		return "", false
	}
	return strings.Join(lines[start:end], "\n"), true
}

// slackTruncateBudget is the reference release-notes character budget. The
// spec's "How the automation reads this file" section only says release
// notes are "truncated to Slack message limits" — it names no number. We
// pick Slack's chat.postMessage plain-text cap (4000 chars), the more
// conservative of Slack's several message-size limits, as the constant this
// contract falsifies truncation against.
const slackTruncateBudget = 4000

// truncateBody truncates s to at most budget runes, counting Unicode code
// points rather than bytes (Slack's own limits are character-based) and
// therefore never splitting a multi-byte rune — a byte-slice truncation
// (s[:n]) would not have this property.
func truncateBody(s string, budget int) string {
	if utf8.RuneCountInString(s) <= budget {
		return s
	}
	return string([]rune(s)[:budget])
}

// TestPendingVersionDerivation falsifies the release-flow invariant: only
// the newest version heading (the first below Unreleased) can ever be
// pending, and only when untagged.
func TestPendingVersionDerivation(t *testing.T) {
	cases := []struct {
		name      string
		content   string
		tagExists func(string) bool
		want      string
		wantErr   bool
	}{
		{
			// Encodes: "Pending version = the newest version heading's
			// X.Y.Z, and only if its vX.Y.Z git tag does not exist."
			name: "pending version exists",
			content: "# Changelog\n\n" +
				unreleasedHeading + "\n\n" +
				"## [1.2.0] - 2026-01-01\n\n### Added\n\n- newest, untagged\n\n" +
				"## [1.1.0] - 2025-12-01\n\n### Added\n\n- older, tagged\n",
			tagExists: func(v string) bool { return v == "1.1.0" },
			want:      "1.2.0",
		},
		{
			// Encodes: "if its tag exists, there is nothing to release" —
			// the newest heading being tagged means no pending version,
			// full stop, regardless of older untagged history underneath.
			name: "newest heading already tagged",
			content: "# Changelog\n\n" +
				unreleasedHeading + "\n\n" +
				"## [1.2.0] - 2026-01-01\n\n### Added\n\n- newest, tagged\n\n" +
				"## [1.1.0] - 2025-12-01\n\n### Added\n\n- older\n",
			tagExists: func(v string) bool { return v == "1.2.0" },
			want:      "",
		},
		{
			// Encodes file grammar rule 3 ("X.Y.Z is semver, without a v
			// prefix") together with "Older untagged headings are never
			// considered pending": a malformed newest heading must be
			// rejected outright, not fall through to the next heading or be
			// silently parsed as some other version.
			name: "malformed newest heading",
			content: "# Changelog\n\n" +
				unreleasedHeading + "\n\n" +
				"## [v1.2.0] - 2026-01-01\n\n### Added\n\n- v-prefixed, violates rule 3\n\n" +
				"## [1.1.0] - 2025-12-01\n\n### Added\n\n- older\n",
			tagExists: func(v string) bool { return false },
			wantErr:   true,
		},
		{
			// Encodes file grammar rule 2: Unreleased is always present and
			// always the first version heading. A file that opens straight
			// into a version heading is not a smaller variant of a valid
			// file — it's ungrammatical, and must error rather than treat
			// that heading as pending by default.
			name: "missing Unreleased entirely",
			content: "# Changelog\n\n" +
				"## [1.2.0] - 2026-01-01\n\n### Added\n\n- no Unreleased above this\n",
			tagExists: func(v string) bool { return false },
			wantErr:   true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := derivePending(tc.content, tc.tagExists)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("derivePending(%q) = %q, nil; want an error", tc.name, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("derivePending(%q): unexpected error: %v", tc.name, err)
			}
			if got != tc.want {
				t.Fatalf("derivePending(%q) = %q, want %q", tc.name, got, tc.want)
			}
		})
	}

	t.Run("real CHANGELOG.md", func(t *testing.T) {
		content, err := os.ReadFile("../../CHANGELOG.md")
		if err != nil {
			t.Fatalf("reading repo CHANGELOG.md: %v", err)
		}
		// Only grammar validity is asserted here (no malformed heading, no
		// missing Unreleased) — whether the newest heading is actually
		// tagged in this checkout is a git-state question this package
		// deliberately doesn't ask, so tagExists is a stub that always
		// answers "tagged".
		if _, err := derivePending(string(content), func(string) bool { return true }); err != nil {
			t.Fatalf("real CHANGELOG.md failed grammar-conformant derivation: %v", err)
		}
	})
}

// TestVersionSectionExtraction falsifies file grammar rule 4 (section body =
// heading to next "## " heading, exclusive, or EOF) and the Slack-budget
// truncation rule in "How the automation reads this file".
func TestVersionSectionExtraction(t *testing.T) {
	const multiVersion = "# Changelog\n\n" +
		unreleasedHeading + "\n\n" +
		"## [2.0.0] - 2026-02-01\n\n### Added\n\n- v2 line one\n- v2 line two\n\n" +
		"## [1.5.0] - 2026-01-15\n\n### Fixed\n\n- mid-file line one\n- mid-file line two\n\n" +
		"## [1.0.0] - 2026-01-01\n\n### Added\n\n- last section line one\n- last section line two\n"

	t.Run("mid-file section bounded by next heading", func(t *testing.T) {
		body, found := extractSection(multiVersion, "1.5.0")
		if !found {
			t.Fatal("extractSection: version 1.5.0 not found")
		}
		want := "\n### Fixed\n\n- mid-file line one\n- mid-file line two\n"
		if body != want {
			t.Fatalf("extractSection(1.5.0) = %q, want %q", body, want)
		}
		if strings.Contains(body, "1.0.0") || strings.Contains(body, "last section") {
			t.Fatalf("extractSection(1.5.0) leaked the next section: %q", body)
		}
	})

	t.Run("last section bounded by EOF", func(t *testing.T) {
		// No trailing link-reference block in this input — see the
		// ambiguity note on extractSection for why that's deliberate here.
		body, found := extractSection(multiVersion, "1.0.0")
		if !found {
			t.Fatal("extractSection: version 1.0.0 not found")
		}
		want := "\n### Added\n\n- last section line one\n- last section line two\n"
		if body != want {
			t.Fatalf("extractSection(1.0.0) = %q, want %q", body, want)
		}
	})

	t.Run("Slack-budget truncation never splits a UTF-8 rune", func(t *testing.T) {
		// "日" and "🔥" are 3- and 4-byte runes; placing them straddling the
		// budget boundary is what would break a byte-slice truncation
		// (s[:slackTruncateBudget]) but must survive a rune-safe one.
		prefix := strings.Repeat("a", slackTruncateBudget-1)
		body := prefix + "日🔥" + strings.Repeat("b", 50)

		got := truncateBody(body, slackTruncateBudget)

		if !utf8.ValidString(got) {
			t.Fatalf("truncateBody produced invalid UTF-8: %q", got)
		}
		if n := utf8.RuneCountInString(got); n != slackTruncateBudget {
			t.Fatalf("truncateBody: got %d runes, want exactly the %d-rune budget", n, slackTruncateBudget)
		}
		if !strings.HasPrefix(got, prefix) {
			t.Fatal("truncateBody dropped or altered content before the boundary")
		}
		if !strings.HasSuffix(got, "日") {
			t.Fatalf("truncateBody: want the cut to land after the whole 日 rune, got tail %q", got[len(prefix):])
		}
	})
}
