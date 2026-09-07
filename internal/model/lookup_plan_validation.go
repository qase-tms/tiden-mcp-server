package model

import (
	"fmt"
	"path"
	"strings"
	"unicode/utf8"
)

func ValidateLookupPlan(plan LookupPlan, maxItems int) error {
	invalid := func(message string) error { return fmt.Errorf("%s", message) }
	if plan.Version != 1 {
		return invalid("lookup plan version must be 1")
	}
	if len(plan.Items) == 0 || len(plan.Items) > maxItems {
		return invalid(fmt.Sprintf("lookup plan requires 1..%d items", maxItems))
	}
	seen := map[string]bool{}
	for _, item := range plan.Items {
		if strings.TrimSpace(item.ID) == "" || len(item.ID) > 64 || seen[item.ID] {
			return invalid("lookup item IDs must be nonempty, unique and at most 64 bytes")
		}
		seen[item.ID] = true
		if strings.TrimSpace(item.Question) == "" || !utf8.ValidString(item.Question) || utf8.RuneCountInString(item.Question) > 2048 {
			return invalid("lookup question must contain 1..2048 characters")
		}
		if len(item.Anchors) > 16 {
			return invalid("lookup item permits at most 16 anchors")
		}
		for _, anchor := range item.Anchors {
			if !validLookupRepository(anchor.Repository) {
				return invalid("lookup anchor requires canonical repository host/owner/repo")
			}
			if anchor.Path == "" || len(anchor.Path) > 1024 || path.IsAbs(anchor.Path) || path.Clean(anchor.Path) != anchor.Path || anchor.Path == "." || anchor.Path == ".." || strings.HasPrefix(anchor.Path, "../") || strings.ContainsAny(anchor.Path, "\\\x00\r\n") {
				return invalid("lookup anchor path must be a clean repository-relative file path")
			}
		}
	}
	return nil
}
func validLookupRepository(repo string) bool {
	parts := strings.Split(repo, "/")
	return len(repo) <= 500 && len(parts) == 3 && strings.Contains(parts[0], ".") && parts[1] != "" && parts[2] != "" && parts[1] != ".." && parts[2] != ".." && !strings.HasSuffix(repo, ".git") && !strings.ContainsAny(repo, " :@\\\t\n\r\x00")
}
