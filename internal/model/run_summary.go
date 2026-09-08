package model

// RunCaseOverview is a read-only case rollup without combination payloads.
type RunCaseOverview struct {
	IdentityKey string   `json:"identityKey"`
	Title       string   `json:"title"`
	SuitePath   []string `json:"suitePath"`
	TestID      string   `json:"testId"`
	TestSeqNum  int      `json:"testSeqNum"`
	Status      string   `json:"status"`
	DurationMs  int64    `json:"durationMs,string"`
	Attempts    int      `json:"attempts"`
	Muted       bool     `json:"muted"`
	ComboCount  int64    `json:"comboCount,string"`
}
