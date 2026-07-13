package model

import "encoding/json"

// Test-run wire models (protojson). int64 fields arrive as JSON strings —
// the `,string` tags mirror that. int32 fields (seqNum, testSeqNum, attempt)
// are plain JSON numbers. Unset message fields arrive as explicit null
// (EmitUnpopulated), hence the pointer fields.

type RunStats struct {
	Total      int64 `json:"total,string"`
	Passed     int64 `json:"passed,string"`
	Failed     int64 `json:"failed,string"`
	Blocked    int64 `json:"blocked,string"`
	Skipped    int64 `json:"skipped,string"`
	Invalid    int64 `json:"invalid,string"`
	Muted      int64 `json:"muted,string"`
	Attempts   int64 `json:"attempts,string"`
	DurationMs int64 `json:"durationMs,string"`
}

type LiveDocStats struct {
	SuitesCreated     int64 `json:"suitesCreated,string"`
	TestsCreated      int64 `json:"testsCreated,string"`
	TestsUpdated      int64 `json:"testsUpdated,string"`
	TestsUnchanged    int64 `json:"testsUnchanged,string"`
	ResultsBackfilled int64 `json:"resultsBackfilled,string"`
}

type TestRun struct {
	ID                 string            `json:"id"`
	ProductID          string            `json:"productId"`
	SeqNum             int               `json:"seqNum"`
	Title              string            `json:"title"`
	Description        string            `json:"description"`
	Status             string            `json:"status"`
	EnvironmentID      string            `json:"environmentId"`
	EnvironmentSlug    string            `json:"environmentSlug"`
	EnvironmentName    string            `json:"environmentName"`
	BranchName         string            `json:"branchName"`
	BranchID           string            `json:"branchId"`
	Configurations     map[string]string `json:"configurations"`
	BuildSha           string            `json:"buildSha"`
	ClientMeta         map[string]string `json:"clientMeta"`
	Stats              *RunStats         `json:"stats"`
	StartedAt          string            `json:"startedAt"`
	CompletedAt        *string           `json:"completedAt"`
	LiveDocStatus      string            `json:"liveDocStatus"`
	LiveDocOperationID string            `json:"liveDocOperationId"`
	LiveDocStats       *LiveDocStats     `json:"liveDocStats"`
	LiveDocError       string            `json:"liveDocError"`
	CreatedBy          string            `json:"createdBy"`
	CreatedAt          string            `json:"createdAt"`
	UpdatedAt          string            `json:"updatedAt"`
}

type RunParamGroup struct {
	Names []string `json:"names"`
}

type RunSuiteSegment struct {
	Title      string `json:"title"`
	ExternalID string `json:"externalId"`
}

type TestRunResult struct {
	ID              string            `json:"id"`
	RunID           string            `json:"runId"`
	TestID          string            `json:"testId"`
	TestSeqNum      int               `json:"testSeqNum"`
	EventSeq        int64             `json:"eventSeq,string"`
	Title           string            `json:"title"`
	Signature       string            `json:"signature"`
	ExternalID      string            `json:"externalId"`
	IdentityKey     string            `json:"identityKey"`
	ExecutionKey    string            `json:"executionKey"`
	Status          string            `json:"status"`
	DurationMs      int64             `json:"durationMs,string"`
	StartedAt       *string           `json:"startedAt"`
	EndedAt         *string           `json:"endedAt"`
	Thread          string            `json:"thread"`
	Message         string            `json:"message"`
	Stacktrace      string            `json:"stacktrace"`
	Params          map[string]string `json:"params"`
	ParamGroups     []RunParamGroup   `json:"paramGroups"`
	Fields          map[string]string `json:"fields"`
	Steps           json.RawMessage   `json:"steps"`
	SuitePath       []RunSuiteSegment `json:"suitePath"`
	Attachments     []string          `json:"attachments"`
	Muted           bool              `json:"muted"`
	Defect          bool              `json:"defect"`
	IsLatestAttempt bool              `json:"isLatestAttempt"`
	Attempt         int               `json:"attempt"`
	CreatedAt       string            `json:"createdAt"`
}

type RunSuiteSummary struct {
	Path  []string  `json:"path"`
	Stats *RunStats `json:"stats"`
}

type RunParamCombo struct {
	ExecutionKey string            `json:"executionKey"`
	Params       map[string]string `json:"params"`
	Status       string            `json:"status"`
	DurationMs   int64             `json:"durationMs,string"`
	Attempts     int               `json:"attempts"`
	ResultID     string            `json:"resultId"`
}

type RunCaseSummary struct {
	IdentityKey string          `json:"identityKey"`
	Title       string          `json:"title"`
	SuitePath   []string        `json:"suitePath"`
	TestID      string          `json:"testId"`
	TestSeqNum  int             `json:"testSeqNum"`
	Status      string          `json:"status"`
	DurationMs  int64           `json:"durationMs,string"`
	Attempts    int             `json:"attempts"`
	Muted       bool            `json:"muted"`
	Combos      []RunParamCombo `json:"combos"`
}
