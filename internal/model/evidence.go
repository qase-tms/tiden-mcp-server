package model

// ResultAnchor is an anchor as the server reports it: repository, path and an
// optional line range (0 or absent = the whole file). It is output-only: the
// tool's input anchors (LookupAnchor) stay {repository, path}.
type ResultAnchor struct {
	Repository string `json:"repository"`
	Path       string `json:"path"`
	LineStart  int    `json:"lineStart,omitempty"`
	LineEnd    int    `json:"lineEnd,omitempty"`
}

// Evidence is one structured reason a spec is related to an item, strongest
// first, at most one entry per kind: anchor, seed, co_anchored, ancestor.
// Absent on a server that predates it.
type Evidence struct {
	Kind             string        `json:"kind"`
	Signals          []string      `json:"signals,omitempty"`
	SeedRank         int           `json:"seedRank,omitempty"`
	Anchor           *ResultAnchor `json:"anchor,omitempty"`
	ViaRequirementID string        `json:"viaRequirementId,omitempty"`
	ViaSeqNum        int           `json:"viaSeqNum,omitempty"`
	Hops             int           `json:"hops,omitempty"`
	Confidence       float64       `json:"confidence,omitempty"`
}
