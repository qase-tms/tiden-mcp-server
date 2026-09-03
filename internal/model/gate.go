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
	Subjects           []GateSubjectResult   `json:"subjects,omitempty"`
	FixHints           []GateFixHint         `json:"fixHints,omitempty"`
	AcceptanceRequired bool                  `json:"acceptanceRequired,omitempty"`
	// --- branch scope ---
	// NextAction is the one move the verdict asks for. An agent handed a status
	// with nothing after it learns to ignore the status.
	NextAction string `json:"nextAction,omitempty"`
	// Annotation names what a green verdict was bought with, e.g.
	// "with accepted risks: R2 QA-14 (...)".
	Annotation string `json:"annotation,omitempty"`
	// AcceptedRisks are the risk acceptances the branch's sessions recorded and
	// the verdict folded in. The risks stay visible: the failing criterion keeps
	// its status.
	AcceptedRisks []GateAcceptedRisk `json:"acceptedRisks,omitempty"`
	// ScopeInfo says how the branch's touched scope was derived; absent from a
	// server that still computes the whole product on a branch.
	ScopeInfo *GateScopeInfo `json:"scopeInfo,omitempty"`
}

// GateAcceptedRisk is one recorded risk acceptance carried on a verdict.
type GateAcceptedRisk struct {
	Criterion       string   `json:"criterion,omitempty"`
	RequirementRefs []string `json:"requirementRefs,omitempty"`
	Evidence        string   `json:"evidence,omitempty"`
	FollowUp        string   `json:"followUp,omitempty"`
	SessionID       string   `json:"sessionId,omitempty"`
}

// GateScopeInfo is the branch verdict's provenance: which sessions defined the
// touched set, or that there was none.
type GateScopeInfo struct {
	Source              string   `json:"source,omitempty"` // "sessions" | "branch_delta" | "empty"
	SessionIDs          []string `json:"sessionIds,omitempty"`
	TouchedRequirements int      `json:"touchedRequirements,omitempty"`
	TouchedComponents   int      `json:"touchedComponents,omitempty"`
	Empty               bool     `json:"empty,omitempty"`
}

// GateTestFact is one non-passing case of a subject, named so an agent can act
// without a second query.
type GateTestFact struct {
	TestID       string   `json:"testId,omitempty"`
	Display      string   `json:"display,omitempty"`
	Status       string   `json:"status,omitempty"`
	Requirements []string `json:"requirements,omitempty"`
	// WasRedOnMain is a pointer because "unknown" (a server that does not send
	// it) is a different claim from "no".
	WasRedOnMain *bool  `json:"wasRedOnMain,omitempty"`
	Message      string `json:"message,omitempty"`
	Accepted     bool   `json:"accepted,omitempty"`
}

// GateRequirementFact places one touched requirement on the coverage ladder and
// names the move that closes it.
type GateRequirementFact struct {
	RequirementID string `json:"requirementId,omitempty"`
	Display       string `json:"display,omitempty"`
	State         string `json:"state,omitempty"` // "verified" | "not_run" | "no_test"
	Accepted      bool   `json:"accepted,omitempty"`
	Deferred      bool   `json:"deferred,omitempty"`
	NextAction    string `json:"nextAction,omitempty"`
}

// GateSubjectResult is the per-subject breakdown of a verdict - the
// generalization of GateComponentResult across subject types
// (component | feature | product). Features are first-class gate subjects;
// `subjects` supersedes the component-only `components` list.
type GateSubjectResult struct {
	SubjectType  string                `json:"subjectType"`
	SubjectID    string                `json:"subjectId"`
	Name         string                `json:"name"`
	Status       string                `json:"status"`
	ResidualRisk *float64              `json:"residualRisk,omitempty"`
	Ceiling      *int                  `json:"ceiling,omitempty"`
	Criteria     []GateCriterionResult `json:"criteria,omitempty"`
	// RiskSource names the component whose risk profile drove a feature's risk
	// fan-in; IssueSources names the components that contributed open issues.
	// Both empty for component/product subjects.
	RiskSource   string   `json:"riskSource,omitempty"`
	IssueSources []string `json:"issueSources,omitempty"`
	// --- branch scope: the facts behind the status, and the move. ---
	Tests         []GateTestFact        `json:"tests,omitempty"`
	Requirements  []GateRequirementFact `json:"requirements,omitempty"`
	NextAction    string                `json:"nextAction,omitempty"`
	AcceptedRisks []GateAcceptedRisk    `json:"acceptedRisks,omitempty"`
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
	Repository   string              `json:"repository,omitempty"` // source repo this component maps to
	Requirements []MatrixRequirement `json:"requirements"`
}

type MatrixRequirement struct {
	RequirementID string       `json:"requirementId"`
	Display       string       `json:"display"`
	Title         string       `json:"title,omitempty"`        // requirement title
	ParentID      string       `json:"parentId,omitempty"`     // parent requirement id (feature-tree grouping)
	BranchStatus  string       `json:"branchStatus,omitempty"` // "added" | "modified" | "unchanged" (branch scope only)
	Coverage      string       `json:"coverage"`               // "verified" | "not_run" | "no_test"
	Cells         []MatrixCell `json:"cells"`
}

type MatrixCell struct {
	Display  string `json:"display"`  // e.g. "TC-3"
	TestCase string `json:"testCase"` // title
	Status   string `json:"status"`   // "passed" | "failed" | "stale" | "absent"
}
