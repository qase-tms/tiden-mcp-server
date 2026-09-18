package gitremote

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestNormalizeRemoteURL(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"", ""},
		{"https://github.com/org/repo.git", "github.com/org/repo"},
		{"https://github.com/org/repo", "github.com/org/repo"},
		{"git@github.com:org/repo.git", "github.com/org/repo"},
		{"ssh://git@github.com/org/repo", "github.com/org/repo"},
		{"ssh://git@github.com:2222/org/repo.git", "github.com:2222/org/repo"},
		{"https://user:token@github.com/org/repo.git", "github.com/org/repo"},
		{"https://GitHub.com/Org/Repo.git", "github.com/Org/Repo"},
		{"https://github.com/org/repo///", "github.com/org/repo"},
		{"git@gitlab.example.com:group/sub/repo.git", "gitlab.example.com/group/sub/repo"},
	}
	for _, tc := range cases {
		got := NormalizeRemoteURL(tc.in)
		if got != tc.want {
			t.Errorf("NormalizeRemoteURL(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestOriginIdentity_NoGitReturnsEmpty(t *testing.T) {
	dir := t.TempDir()
	if got := OriginIdentity(dir); got != "" {
		t.Errorf("OriginIdentity(%q) = %q, want empty (no git repo)", dir, got)
	}
}

func TestOriginIdentity_NoOriginRemoteReturnsEmpty(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init", "-q")
	if got := OriginIdentity(dir); got != "" {
		t.Errorf("OriginIdentity(%q) = %q, want empty (no origin remote)", dir, got)
	}
}

func TestOriginIdentity_ReturnsNormalizedOrigin(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init", "-q")
	runGit(t, dir, "remote", "add", "origin", "git@github.com:qase-tms/tiden-mcp-server.git")

	nested := filepath.Join(dir, "a", "b")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}

	got := OriginIdentity(nested)
	want := "github.com/qase-tms/tiden-mcp-server"
	if got != want {
		t.Errorf("OriginIdentity(%q) = %q, want %q", nested, got, want)
	}
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}
