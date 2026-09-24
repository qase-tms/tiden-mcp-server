// Package gitremote gives the resolution ladder (internal/resolve) the
// canonical identity of the git repository above the server's working
// directory, for the "repository lookup" resolution step
// (GET /v1/repositories/resolve). It is READ-ONLY: it only ever shells out
// to `git remote get-url origin`, never writes.
package gitremote

import (
	"os/exec"
	"strings"
)

// OriginIdentity returns the canonicalized "host/path" identity of the
// `origin` remote of the git repository containing cwd (or any of its
// parents — git itself resolves the nearest .git upward). It returns "" when
// cwd is not inside a git work tree, or the repository has no `origin`
// remote: the caller (internal/resolve) treats that as "skip the repository
// lookup step", not as an error.
func OriginIdentity(cwd string) string {
	cmd := exec.Command("git", "remote", "get-url", "origin")
	cmd.Dir = cwd
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return NormalizeRemoteURL(strings.TrimSpace(string(out)))
}

// NormalizeRemoteURL canonicalizes a git remote URL to "host/path" so the
// same repository yields one identity across clone-URL forms: the scheme
// (ssh://, git://, http://, https://) and credentials (user[:pass]@) are
// stripped, the scp-like "user@host:path" form is rewritten to "host/path",
// the host is lowercased (a port, if present, is kept as-is), and trailing
// ".git" and slashes are removed. An empty string passes through unchanged.
//
// Credentials NEVER survive: the authority is truncated at its last '@'
// before the key is built, so an embedded token in a remote URL cannot leak
// into diagnostics or error messages.
//
// This must stay in agreement with tiden-cli's
// internal/intent.NormalizeRemoteURL — the two mean "the same repository"
// the same way (see TIDEN-68 ladder specification).
func NormalizeRemoteURL(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}

	hasScheme := false
	if i := strings.Index(s, "://"); i >= 0 {
		s = s[i+3:]
		hasScheme = true
	}

	// Strip credentials: drop everything up to the last '@' before the path.
	hostEnd := strings.IndexByte(s, '/')
	if hostEnd < 0 {
		hostEnd = len(s)
	}
	if at := strings.LastIndexByte(s[:hostEnd], '@'); at >= 0 {
		s = s[at+1:]
	}

	// scp-like form (no scheme): "host:path" -> "host/path". With a scheme,
	// a colon in the authority is a port and is kept.
	if !hasScheme {
		if c := strings.IndexByte(s, ':'); c >= 0 {
			if slash := strings.IndexByte(s, '/'); slash < 0 || c < slash {
				s = s[:c] + "/" + strings.TrimPrefix(s[c+1:], "/")
			}
		}
	}

	// Lowercase only the host (the path stays case-sensitive).
	if slash := strings.IndexByte(s, '/'); slash >= 0 {
		s = strings.ToLower(s[:slash]) + s[slash:]
	} else {
		s = strings.ToLower(s)
	}

	s = strings.TrimRight(s, "/")
	s = strings.TrimSuffix(s, ".git")
	return strings.TrimRight(s, "/")
}
