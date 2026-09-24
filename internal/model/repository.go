package model

// Repository candidate sources: where the server found a repository ->
// product/workspace match. Mirrors api.v1.RepositoryCandidateSource.
const (
	RepositoryCandidateSourceCISync    = "REPOSITORY_CANDIDATE_SOURCE_CI_SYNC"
	RepositoryCandidateSourceCodeLink  = "REPOSITORY_CANDIDATE_SOURCE_CODE_LINK"
	RepositoryCandidateSourceComponent = "REPOSITORY_CANDIDATE_SOURCE_COMPONENT"
)

// RepositoryCandidate is one product/workspace match for a repository URL,
// returned by GET /v1/repositories/resolve.
type RepositoryCandidate struct {
	ProductID     string `json:"productId"`
	ProductName   string `json:"productName"`
	WorkspaceID   string `json:"workspaceId"`
	WorkspaceName string `json:"workspaceName"`
	Source        string `json:"source"`
}

// SourceLabel returns a short human-readable label for Source, for display
// in a disambiguation message. Unknown values pass through unchanged.
func (c RepositoryCandidate) SourceLabel() string {
	switch c.Source {
	case RepositoryCandidateSourceCISync:
		return "ci"
	case RepositoryCandidateSourceCodeLink:
		return "code link"
	case RepositoryCandidateSourceComponent:
		return "component"
	default:
		return c.Source
	}
}
