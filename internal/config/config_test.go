package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoadMalformedFileReturnsErrorAndUsableConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.MkdirAll(filepath.Join(home, ".tiden"), 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(home, ".tiden", "config.json")
	if err := os.WriteFile(path, []byte(`{"baseUrl": "https://x",`), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TIDEN_BASE_URL", "https://from-env")

	cfg, err := Load("", "tok-from-flag", "")
	if err == nil {
		t.Fatal("expected parse error for malformed config file")
	}
	if !strings.Contains(err.Error(), path) {
		t.Errorf("err = %v, want it to name %s", err, path)
	}
	if cfg == nil {
		t.Fatal("cfg must be non-nil even on parse error")
	}
	if cfg.BaseURL != "https://from-env" || cfg.APIToken != "tok-from-flag" {
		t.Errorf("env/flag overrides not applied: %+v", cfg)
	}
}

func TestLoadValidFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("TIDEN_BASE_URL", "")
	t.Setenv("TIDEN_API_TOKEN", "")
	if err := os.MkdirAll(filepath.Join(home, ".tiden"), 0o755); err != nil {
		t.Fatal(err)
	}
	body := []byte(`{"baseUrl": "https://api.example.com", "apiToken": "tok"}`)
	if err := os.WriteFile(filepath.Join(home, ".tiden", "config.json"), body, 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load("", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.BaseURL != "https://api.example.com" || cfg.APIToken != "tok" {
		t.Errorf("cfg = %+v", cfg)
	}
}

func TestRequestTimeout(t *testing.T) {
	cases := []struct {
		in      string
		want    time.Duration
		wantErr bool
	}{
		{"", 30 * time.Second, false},
		{"60s", time.Minute, false},
		{"2m", 2 * time.Minute, false},
		{"banana", 0, true},
		{"-5s", 0, true},
		{"0", 0, true},
	}
	for _, tc := range cases {
		d, err := (&Config{Timeout: tc.in}).RequestTimeout()
		if tc.wantErr {
			if err == nil {
				t.Errorf("Timeout=%q: expected error", tc.in)
			}
			continue
		}
		if err != nil {
			t.Errorf("Timeout=%q: unexpected error: %v", tc.in, err)
		}
		if d != tc.want {
			t.Errorf("Timeout=%q: got %v, want %v", tc.in, d, tc.want)
		}
	}
}
