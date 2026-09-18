package config

import (
	"fmt"
	"time"
)

// ParseTimeout parses a Timeout setting (empty, a flag, an env var, or
// Store.Timeout). Empty means the 30s default.
func ParseTimeout(s string) (time.Duration, error) {
	if s == "" {
		return 30 * time.Second, nil
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("invalid timeout %q: %w", s, err)
	}
	if d <= 0 {
		return 0, fmt.Errorf("invalid timeout %q: must be positive", s)
	}
	return d, nil
}
