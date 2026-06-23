package model

// Quality Gate types - mirror the backend protojson (camelCase fields; enums as
// their proto names, e.g. "VERDICT_STATUS_BLOCKED"). Hand-written: the CLI is a
// pure REST client with no codegen.

type Verdict struct {
	ID                 string                `json:"id"`
	ProductID          string                `json:"productId"`
	Scope              string                `json:"scope"`
	ReleaseID          string                `json:"releaseId,omitempty"`
	BranchID           string                `json:"branchId,omitempty"`
	BuildSHA           string                `json:"buildSha,omitempty"`
	Status             string                `json:"status"`
	ComputedAt         string                `json:"computedAt,omitempty"`
	Components         []GateComponentResult `json:"components,omitempty"`
	FixHints           []GateFixHint         `json:"fixHints,omitempty"`
	AcceptanceRequired bool                  `json:"acceptanceRequired,omitempty"`
}

type GateComponentResult struct {
	ComponentID  string                `json:"componentId"`
	Name         string                `json:"name"`
	Status       string                `json:"status"`
	ResidualRisk *float64              `json:"residualRisk,omitempty"`
	Ceiling      *int                  `json:"ceiling,omitempty"`
	Criteria     []GateCriterionResult `json:"criteria,omitempty"`
}

type GateCriterionResult struct {
	Criterion string               `json:"criterion"`
	Status    string               `json:"status"`
	Soft      bool                 `json:"soft,omitempty"`
	Accepted  bool                 `json:"accepted,omitempty"`
	Detail    *GateCriterionDetail `json:"detail,omitempty"`
}

type GateCriterionDetail struct {
	CoverageGap  string            `json:"coverageGap,omitempty"`
	FailingTests []GateFailingTest `json:"failingTests,omitempty"`
}

type GateFailingTest struct {
	Requirement string `json:"requirement"`
	TestCase    string `json:"testCase"`
}

type GateFixHint struct {
	Component string `json:"component"`
	Action    string `json:"action"`
}

type TraceabilityMatrix struct {
	Components []MatrixComponent `json:"components"`
}

type MatrixComponent struct {
	ComponentID  string              `json:"componentId"`
	Name         string              `json:"name"`
	Status       string              `json:"status"`
	ResidualRisk *float64            `json:"residualRisk,omitempty"`
	Ceiling      *int                `json:"ceiling,omitempty"`
	Requirements []MatrixRequirement `json:"requirements"`
}

type MatrixRequirement struct {
	RequirementID string       `json:"requirementId"`
	Display       string       `json:"display"`
	Coverage      string       `json:"coverage"` // "verified" | "not_run" | "no_test"
	Cells         []MatrixCell `json:"cells"`
}

type MatrixCell struct {
	Display  string `json:"display"`  // e.g. "TC-3"
	TestCase string `json:"testCase"` // title
	Status   string `json:"status"`   // "passed" | "failed" | "stale" | "absent"
}
