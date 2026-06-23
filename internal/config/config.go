package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type Config struct {
	BaseURL     string `json:"baseUrl"`
	APIToken    string `json:"apiToken"`
	WorkspaceID string `json:"workspaceId,omitempty"`

	// Timeout is the per-request API timeout as a Go duration string
	// (e.g. "60s"). Empty means the 30s default. Settable via the config
	// file, TIDEN_TIMEOUT, or --timeout.
	Timeout string `json:"timeout,omitempty"`
}

// Load merges config from: file (~/.tiden/config.json) < env vars < explicit overrides.
//
// A malformed config file is reported as an error, but the returned Config is
// always non-nil with env vars and flag overrides applied.
func Load(flagBaseURL, flagAPIToken, flagWorkspaceID string) (*Config, error) {
	cfg := &Config{}
	var loadErr error

	// 1. Read config file
	home, err := os.UserHomeDir()
	if err == nil {
		path := filepath.Join(home, ".tiden", "config.json")
		if data, err := os.ReadFile(path); err == nil {
			if uerr := json.Unmarshal(data, cfg); uerr != nil {
				*cfg = Config{} // discard any partial parse
				loadErr = fmt.Errorf("parse %s: %w", path, uerr)
			}
		}
	}

	// 2. Env vars override file
	if v := os.Getenv("TIDEN_BASE_URL"); v != "" {
		cfg.BaseURL = v
	}
	if v := os.Getenv("TIDEN_API_TOKEN"); v != "" {
		cfg.APIToken = v
	}
	if v := os.Getenv("TIDEN_WORKSPACE_ID"); v != "" {
		cfg.WorkspaceID = v
	}
	if v := os.Getenv("TIDEN_TIMEOUT"); v != "" {
		cfg.Timeout = v
	}

	// 3. Flags override everything
	if flagBaseURL != "" {
		cfg.BaseURL = flagBaseURL
	}
	if flagAPIToken != "" {
		cfg.APIToken = flagAPIToken
	}
	if flagWorkspaceID != "" {
		cfg.WorkspaceID = flagWorkspaceID
	}

	return cfg, loadErr
}

// RequestTimeout parses the Timeout setting. Empty means 30s.
func (c *Config) RequestTimeout() (time.Duration, error) {
	if c.Timeout == "" {
		return 30 * time.Second, nil
	}
	d, err := time.ParseDuration(c.Timeout)
	if err != nil {
		return 0, fmt.Errorf("invalid timeout %q: %w", c.Timeout, err)
	}
	if d <= 0 {
		return 0, fmt.Errorf("invalid timeout %q: must be positive", c.Timeout)
	}
	return d, nil
}

// Check returns a list of missing required fields.
func (c *Config) Check() []string {
	var missing []string
	if c.BaseURL == "" {
		missing = append(missing, "baseUrl")
	}
	if c.APIToken == "" {
		missing = append(missing, "apiToken")
	}
	return missing
}
