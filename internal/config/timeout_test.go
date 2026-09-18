package config

import (
	"testing"
	"time"
)

func TestParseTimeout(t *testing.T) {
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
		d, err := ParseTimeout(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Errorf("ParseTimeout(%q): expected error", tc.in)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseTimeout(%q): unexpected error: %v", tc.in, err)
		}
		if d != tc.want {
			t.Errorf("ParseTimeout(%q): got %v, want %v", tc.in, d, tc.want)
		}
	}
}
