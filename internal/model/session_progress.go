package model

// Session progress types - mirror the backend protojson for GetSessionProgress
// (POST /v1/products/{pid}/quality-gate:session-progress). Hand-written: the
// server is a pure REST client with no codegen.
//
// Counters are plain JSON numbers (proto int32), not the `,string` int64
// pattern run.go's wire models need.

// SessionProgress is the per-session slice of the quality gate: where each
// requirement in the session's scope stands on the coverage ladder
// no_test -> not_run -> failing -> verified, plus the summary and readiness.
type SessionProgress struct {
	Requirements []SessionProgressRequirement `json:"requirements,omitempty"`
	Summary      SessionProgressSummary       `json:"summary"`
	// Ready is advisory: total > 0 and every requirement verified.
	Ready bool `json:"ready,omitempty"`
	// NextActions are the deterministic "what closes the remaining gap"
	// hints (generate-tests for the no_test group, fix + re-run for failing).
	NextActions []string `json:"nextActions,omitempty"`
}

// SessionProgressSummary counts the requirements per ladder rung.
type SessionProgressSummary struct {
	Total    int `json:"total"`
	Verified int `json:"verified"`
	Failing  int `json:"failing"`
	NotRun   int `json:"notRun"`
	NoTest   int `json:"noTest"`
}

// SessionProgressRequirement is one requirement's rung on the ladder, with the
// tests that decide it.
type SessionProgressRequirement struct {
	RequirementID string `json:"requirementId"`
	Display       string `json:"display,omitempty"` // e.g. "QA-57"
	Title         string `json:"title,omitempty"`
	// Coverage: "no_test" | "not_run" | "failing" | "verified".
	Coverage string `json:"coverage"`
	// ProposedOnly marks a requirement whose only links are intent-branch
	// proposals (nothing durable yet).
	ProposedOnly bool `json:"proposedOnly,omitempty"`
	// MovedThisSession marks a requirement whose deciding execution came from
	// this session (attribution stays visible even when CI green counts).
	MovedThisSession bool                  `json:"movedThisSession,omitempty"`
	Tests            []SessionProgressTest `json:"tests,omitempty"`
}

// SessionProgressTest is one linked test and its latest observed execution.
type SessionProgressTest struct {
	TestID  string `json:"testId,omitempty"`
	Display string `json:"display,omitempty"` // real {CODE}-{seq}, not positional
	Title   string `json:"title,omitempty"`
	Status  string `json:"status,omitempty"` // "passed" | "failed" | ... | "" (never run)
	// FromSession marks an execution reported by this session's own runs.
	FromSession bool `json:"fromSession,omitempty"`
	// RunSeq is the run the deciding execution came from (0 = unknown).
	RunSeq int `json:"runSeq,omitempty"`
	// Proposed marks a link that exists only as an intent-branch proposal.
	Proposed bool `json:"proposed,omitempty"`
}
