package model

// RequirementIdentity is read-only. Body, source excerpts and metadata are
// absent, not empty values to copy into an update request.
type RequirementIdentity struct {
	ID                   string                     `json:"id"`
	ProductID            string                     `json:"productId"`
	ParentID             *string                    `json:"parentId,omitempty"`
	ComponentID          *string                    `json:"componentId,omitempty"`
	Title                string                     `json:"title"`
	SeqNum               int                        `json:"seqNum"`
	SourceID             *string                    `json:"sourceId,omitempty"`
	BranchStatus         string                     `json:"branchStatus,omitempty"`
	ContentSHA256        string                     `json:"contentSha256"`
	TrimmedContentSHA256 string                     `json:"trimmedContentSha256"`
	Sources              []RequirementSourceLocator `json:"sources"`
}

type RequirementSourceLocator struct {
	SourceType string `json:"sourceType"`
	Locator    string `json:"locator"`
	URL        string `json:"url"`
	Repository string `json:"repository"`
	RepoPath   string `json:"repoPath"`
	LineStart  *int   `json:"lineStart,omitempty"`
	LineEnd    *int   `json:"lineEnd,omitempty"`
}
