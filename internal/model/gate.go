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
	// --- release scope ---
	// ReleaseCapture says which capture of the release this verdict is and
	// where it came from; absent on other scopes and from an older server.
	ReleaseCapture *ReleaseCapture `json:"releaseCapture,omitempty"`
}

// ReleaseCapture is the provenance of a release's current Quality Gate
// capture. A release keeps every capture; the highest Seq is current.
type ReleaseCapture struct {
	Seq int `json:"seq,omitempty"`
	// Source is "build_sha" (computed from BuildSHA's test results only) or
	// "main" (a copy of the main verdict: the release had no build SHA).
	Source string `json:"source,omitempty"`
	// Trigger: create, ingest, announce, executions, manual, or legacy.
	Trigger    string `json:"trigger,omitempty"`
	BuildSHA   string `json:"buildSha,omitempty"`
	CapturedAt string `json:"capturedAt,omitempty"`
	CapturedBy string `json:"capturedBy,omitempty"`
	// ReleaseBuildSHA is the release's build SHA now, which can differ from
	// the one this capture was taken from.
	ReleaseBuildSHA string `json:"releaseBuildSha,omitempty"`
	// RecapturePending: the release names a build SHA this capture was not
	// taken from, so a newer capture is still to come.
	RecapturePending bool `json:"recapturePending,omitempty"`
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
	// Source: "sessions" | "branch_delta" | "unmeasured" | "retrieval_only" |
	// "empty". The absences are DISTINCT because they call for different
	// actions — code that resolved to no requirement, a session that read but
	// asserted nothing, and no intent at all are three different problems.
	Source              string   `json:"source,omitempty"`
	SessionIDs          []string `json:"sessionIds,omitempty"`
	TouchedRequirements int      `json:"touchedRequirements,omitempty"`
	TouchedComponents   int      `json:"touchedComponents,omitempty"`
	Empty               bool     `json:"empty,omitempty"`
	// BySource attributes the touched set per provenance — "changed",
	// "anchored", "neighbour" — plus the counts of what was deliberately NOT
	// graded ("retrieval_not_scoped", "directory_not_scoped") and no_evidence.
	BySource map[string]int `json:"bySource,omitempty"`
	// Notes name what the scope left out and the action that brings it in.
	// These MUST survive the round trip: this model is what a tool's answer is
	// decoded into and re-encoded from, so a field missing here is a fact the
	// agent never receives — the same defect the branch verdict exists to fix.
	Notes []string `json:"notes,omitempty"`
	// Diverged: a contributing session's own reconcile found no overlap between
	// what it retrieved and what it changed.
	Diverged bool `json:"diverged,omitempty"`
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
	// Touched is why this requirement is graded: "changed" or "anchored" is
	// this session's own work, "neighbour" is one graph hop away from it. An
	// agent that cannot tell its own work from adjacent debt reads the whole
	// answer as somebody else's — which is what happened on the branch this
	// field exists for. Empty means a server that does not say; treat that as
	// OWN work, never as adjacent.
	Touched string `json:"touched,omitempty"`
	// UncoveredOnMain: this requirement was already uncovered before the
	// branch existed, so its gap is not this session's to close.
	UncoveredOnMain bool `json:"uncoveredOnMain,omitempty"`
	// CoverageExemption is "child_specs" when the row is a parent that is not a
	// gap of its own because its child specs are graded in the same evaluation.
	CoverageExemption string `json:"coverageExemption,omitempty"`
	// ViaChildren, ChildSpecs and ChildSpecsCovered mark a parent spec with no
	// test links of its own whose coverage comes from its child specs:
	// ChildSpecs counts the child specs it was read from, ChildSpecsCovered how
	// many of those are in the good state. Absent on every other row and on a
	// server that predates them.
	ViaChildren       bool `json:"viaChildren,omitempty"`
	ChildSpecs        int  `json:"childSpecs,omitempty"`
	ChildSpecsCovered int  `json:"childSpecsCovered,omitempty"`
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

// TraceabilitySummary is the summary view of the matrix: the same components
// and requirements without per-test cells.
type TraceabilitySummary struct {
	Components []SummaryComponent `json:"components"`
}

type SummaryComponent struct {
	ComponentID  string               `json:"componentId"`
	Name         string               `json:"name"`
	Status       string               `json:"status"`
	ResidualRisk *float64             `json:"residualRisk,omitempty"`
	Ceiling      *int                 `json:"ceiling,omitempty"`
	Repository   string               `json:"repository,omitempty"`
	Requirements []SummaryRequirement `json:"requirements"`
}

// SummaryRequirement is one summary row. The count fields are pointers so an
// older server that does not send them is passed through as absent, not as 0.
type SummaryRequirement struct {
	RequirementID string `json:"requirementId"`
	Display       string `json:"display"`
	Title         string `json:"title,omitempty"`
	ParentID      string `json:"parentId,omitempty"`
	BranchStatus  string `json:"branchStatus,omitempty"`
	CanonicalID   string `json:"canonicalId,omitempty"`
	Coverage      string `json:"coverage"`
	Linked        *int   `json:"linked,omitempty"`
	Passed        *int   `json:"passed,omitempty"`
	Failed        *int   `json:"failed,omitempty"`
	Stale         *int   `json:"stale,omitempty"`
	// CoverageExemption is "child_specs" when the row is a parent that is not a
	// gap of its own because its child specs are graded in the same evaluation.
	CoverageExemption string `json:"coverageExemption,omitempty"`
	// ViaChildren, ChildSpecs and ChildSpecsCovered mark a parent spec with no
	// test links of its own whose coverage comes from its child specs:
	// ChildSpecs counts the child specs it was read from, ChildSpecsCovered how
	// many of those are in the good state. Absent on every other row and on a
	// server that predates them.
	ViaChildren       bool `json:"viaChildren,omitempty"`
	ChildSpecs        int  `json:"childSpecs,omitempty"`
	ChildSpecsCovered int  `json:"childSpecsCovered,omitempty"`
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
	// CoverageExemption is "child_specs" when the row is a parent that is not a
	// gap of its own because its child specs are graded in the same evaluation.
	CoverageExemption string `json:"coverageExemption,omitempty"`
	// ViaChildren, ChildSpecs and ChildSpecsCovered mark a parent spec with no
	// test links of its own whose coverage comes from its child specs:
	// ChildSpecs counts the child specs it was read from, ChildSpecsCovered how
	// many of those are in the good state. Absent on every other row and on a
	// server that predates them.
	ViaChildren       bool `json:"viaChildren,omitempty"`
	ChildSpecs        int  `json:"childSpecs,omitempty"`
	ChildSpecsCovered int  `json:"childSpecsCovered,omitempty"`
}

type MatrixCell struct {
	Display  string `json:"display"`  // e.g. "TC-3"
	TestCase string `json:"testCase"` // title
	Status   string `json:"status"`   // "passed" | "failed" | "stale" | "absent"
}
