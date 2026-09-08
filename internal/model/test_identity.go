package model

// TestIdentity is an inventory-only type. It has no description, steps or
// execution payload and is not accepted by test create/update methods.
type TestIdentity struct {
	ID           string   `json:"id"`
	ProductID    string   `json:"productId"`
	ParentID     *string  `json:"parentId,omitempty"`
	SourceID     *string  `json:"sourceId,omitempty"`
	BranchStatus string   `json:"branchStatus"`
	Kind         string   `json:"kind"`
	Title        string   `json:"title"`
	SeqNum       *int     `json:"seqNum,omitempty"`
	Tags         []string `json:"tags"`
	Signature    string   `json:"signature"`
	Framework    string   `json:"framework"`
	FilePath     string   `json:"filePath"`
	Layer        string   `json:"layer"`
	IsAutomated  bool     `json:"isAutomated"`
}
