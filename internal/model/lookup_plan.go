package model

// LookupPlan contains search questions, not implementation commitments.
// IDs are stable within a plan and have no relationship to promise IDs.
type LookupPlan struct {
	// Context is the last complete refresh, supplied only when persisting a session.
	Context *LookupBatchResult `json:"context,omitempty"`
	Version int                `json:"version"`
	Items   []LookupItem       `json:"items"`
}

type LookupAnchor struct {
	Repository string `json:"repository"`
	Path       string `json:"path"`
}

type LookupItem struct {
	ID       string         `json:"id"`
	Question string         `json:"question"`
	Anchors  []LookupAnchor `json:"anchors,omitempty"`
}

type LookupMatch struct {
	RequirementID  string         `json:"requirementId"`
	FeatureID      string         `json:"featureId"`
	Roles          []string       `json:"roles"`
	Signals        []string       `json:"signals,omitempty"`
	MatchedAnchors []LookupAnchor `json:"matchedAnchors,omitempty"`
}

type LookupRequirement struct {
	ID          string         `json:"id"`
	CanonicalID string         `json:"canonicalId"`
	SeqNum      int            `json:"seqNum"`
	Title       string         `json:"title"`
	Description string         `json:"description"`
	Anchors     []LookupAnchor `json:"anchors,omitempty"`
}

type LookupItemResult struct {
	ID                      string        `json:"id"`
	Question                string        `json:"question"`
	Status                  string        `json:"status"`
	Matches                 []LookupMatch `json:"matches"`
	CandidateRequirementIDs []string      `json:"candidateRequirementIds"`
	CoverageGapIDs          []string      `json:"coverageGapIds"`
	Truncated               bool          `json:"truncated"`
	Omitted                 int           `json:"omitted"`
	LimitReached            bool          `json:"limitReached"`
	Diagnostics             []string      `json:"diagnostics"`
	Error                   string        `json:"error,omitempty"`
}

type LookupBatchResult struct {
	SessionLookupPlanSupported bool                `json:"sessionLookupPlanSupported"`
	Version                    int                 `json:"version"`
	ProductID                  string              `json:"productId"`
	BranchID                   string              `json:"branchId"`
	ReadAt                     string              `json:"readAt"`
	RetrievalStatus            string              `json:"retrievalStatus"`
	Items                      []LookupItemResult  `json:"items"`
	Requirements               []LookupRequirement `json:"requirements"`
}
