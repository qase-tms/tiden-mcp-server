package config

import (
	"os"
	"testing"
)

// TestMain isolates HOME and clears TIDEN_* for the whole package: no test may
// read the developer's real ~/.tiden or a real token from the environment
// (a reporter run once leaked one into a failure message).
func TestMain(m *testing.M) {
	home, err := os.MkdirTemp("", "tiden-mcp-test-home-")
	if err != nil {
		panic(err)
	}
	_ = os.Setenv("HOME", home)
	for _, k := range []string{"TIDEN_API_TOKEN", "TIDEN_WORKSPACE_ID", "TIDEN_BASE_URL", "TIDEN_PRODUCT_ID", "TIDEN_TIMEOUT"} {
		_ = os.Unsetenv(k)
	}
	code := m.Run()
	_ = os.RemoveAll(home)
	os.Exit(code)
}
