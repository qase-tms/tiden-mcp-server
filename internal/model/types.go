package model

type User struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatarUrl,omitempty"`
	CreatedAt string `json:"createdAt,omitempty"`
	UpdatedAt string `json:"updatedAt,omitempty"`
}

type Workspace struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Slug      string `json:"slug"`
	CreatedAt string `json:"createdAt,omitempty"`
	UpdatedAt string `json:"updatedAt,omitempty"`
}

type Product struct {
	ID          string `json:"id"`
	WorkspaceID string `json:"workspaceId"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Code        string `json:"code"`
	CreatedAt   string `json:"createdAt,omitempty"`
	UpdatedAt   string `json:"updatedAt,omitempty"`
}

type Component struct {
	ID          string `json:"id"`
	ProductID   string `json:"productId"`
	Name        string `json:"name"`
	Description string `json:"description"`
	// Repository/ComponentPaths/RepositoryAliases are the
	// repository-aware component fields.
	// Repository is the canonical repo id (e.g. "github.com/example-org/checkout");
	// nil/absent means unscoped (the product's main repo). ComponentPaths are
	// subtree path-prefix scopes within that repository (empty = whole repo).
	// RepositoryAliases are local checkout paths mapped to the canonical id.
	Repository        *string  `json:"repository,omitempty"`
	ComponentPaths    []string `json:"componentPaths,omitempty"`
	RepositoryAliases []string `json:"repositoryAliases,omitempty"`
	CreatedAt         string   `json:"createdAt,omitempty"`
	UpdatedAt         string   `json:"updatedAt,omitempty"`
}

type Environment struct {
	ID          string `json:"id"`
	ProductID   string `json:"productId"`
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Description string `json:"description"`
	Host        string `json:"host"`
	Type        string `json:"type"`
	Origin      string `json:"origin"`
	CreatedAt   string `json:"createdAt,omitempty"`
	UpdatedAt   string `json:"updatedAt,omitempty"`
}

type Release struct {
	ID              string `json:"id"`
	ProductID       string `json:"productId"`
	Version         string `json:"version"`
	EnvironmentID   string `json:"environmentId,omitempty"`
	EnvironmentSlug string `json:"environmentSlug,omitempty"`
	Name            string `json:"name"`
	URL             string `json:"url"`
	Description     string `json:"description"`
	ReleasedAt      string `json:"releasedAt,omitempty"`
	CreatedAt       string `json:"createdAt,omitempty"`
	UpdatedAt       string `json:"updatedAt,omitempty"`
}

type Requirement struct {
	ID            string              `json:"id"`
	ProductID     string              `json:"productId"`
	ParentID      *string             `json:"parentId,omitempty"`
	ComponentID   *string             `json:"componentId,omitempty"`
	Title         string              `json:"title"`
	Content       string              `json:"content"`
	Position      int                 `json:"position"`
	CreatedBy     string              `json:"createdBy,omitempty"`
	ChildrenCount int                 `json:"childrenCount"`
	SeqNum        int                 `json:"seqNum"`
	Status        string              `json:"status"`
	Priority      string              `json:"priority"`
	Type          string              `json:"type"`
	AssigneeID    *string             `json:"assigneeId,omitempty"`
	BranchID      string              `json:"branchId,omitempty"`
	SourceID      *string             `json:"sourceId,omitempty"`
	BranchStatus  string              `json:"branchStatus,omitempty"`
	SourceCount   int                 `json:"sourceCount,omitempty"`
	Sources       []RequirementSource `json:"sources,omitempty"`
	CreatedAt     string              `json:"createdAt,omitempty"`
	UpdatedAt     string              `json:"updatedAt,omitempty"`
}

type RequirementSource struct {
	ID                  string         `json:"id"`
	ProductID           string         `json:"productId"`
	BranchID            string         `json:"branchId"`
	RequirementID       string         `json:"requirementId"`
	SourceType          string         `json:"sourceType"`
	Title               string         `json:"title"`
	Locator             string         `json:"locator"`
	URL                 string         `json:"url"`
	RepoPath            string         `json:"repoPath"`
	LineStart           *int           `json:"lineStart,omitempty"`
	LineEnd             *int           `json:"lineEnd,omitempty"`
	Excerpt             string         `json:"excerpt"`
	Metadata            map[string]any `json:"metadata,omitempty"`
	CreatedBy           string         `json:"createdBy,omitempty"`
	CreatedByAgentRunID *string        `json:"createdByAgentRunId,omitempty"`
	CreatedAt           string         `json:"createdAt,omitempty"`
	UpdatedAt           string         `json:"updatedAt,omitempty"`
}

type RequirementSourceInput struct {
	SourceType          string         `json:"sourceType"`
	Title               string         `json:"title,omitempty"`
	Locator             string         `json:"locator,omitempty"`
	URL                 string         `json:"url,omitempty"`
	RepoPath            string         `json:"repoPath,omitempty"`
	LineStart           *int           `json:"lineStart,omitempty"`
	LineEnd             *int           `json:"lineEnd,omitempty"`
	Excerpt             string         `json:"excerpt,omitempty"`
	Metadata            map[string]any `json:"metadata,omitempty"`
	CreatedByAgentRunID *string        `json:"createdByAgentRunId,omitempty"`
}

type RequirementSourcesUpdate struct {
	Sources []RequirementSourceInput `json:"sources"`
}

type Pagination struct {
	NextPageToken string `json:"nextPageToken"`
	TotalCount    int    `json:"totalCount"`
}

type TestRequirementLink struct {
	ID            string `json:"id"`
	TestID        string `json:"testId"`
	RequirementID string `json:"requirementId"`
	CreatedAt     string `json:"createdAt,omitempty"`
}

type BranchRequirementLinkProposal struct {
	ID            string  `json:"id"`
	BranchID      string  `json:"branchId"`
	TestID        string  `json:"testId"`
	RequirementID string  `json:"requirementId"`
	Status        string  `json:"status"`
	CreatedBy     string  `json:"createdBy,omitempty"`
	ReviewedBy    *string `json:"reviewedBy,omitempty"`
	ReviewedAt    string  `json:"reviewedAt,omitempty"`
	ReviewNote    string  `json:"reviewNote,omitempty"`
	CreatedAt     string  `json:"createdAt,omitempty"`
	UpdatedAt     string  `json:"updatedAt,omitempty"`
}

type TestStep struct {
	ID       string     `json:"id,omitempty"`
	Action   string     `json:"action"`
	Expected string     `json:"expected,omitempty"`
	Data     string     `json:"data,omitempty"`
	Children []TestStep `json:"children,omitempty"`
}

type Test struct {
	ID                     string         `json:"id"`
	ProductID              string         `json:"productId"`
	BranchID               string         `json:"branchId"`
	ParentID               *string        `json:"parentId,omitempty"`
	Kind                   string         `json:"kind"`
	Title                  string         `json:"title"`
	Description            string         `json:"description"`
	Position               int            `json:"position"`
	SeqNum                 *int           `json:"seqNum,omitempty"`
	Status                 string         `json:"status,omitempty"`
	Priority               string         `json:"priority,omitempty"`
	Type                   string         `json:"type,omitempty"`
	Layer                  string         `json:"layer,omitempty"`
	Muted                  bool           `json:"muted,omitempty"`
	ComponentID            *string        `json:"componentId,omitempty"`
	AssigneeID             *string        `json:"assigneeId,omitempty"`
	Tags                   []string       `json:"tags,omitempty"`
	CustomFields           map[string]any `json:"customFields,omitempty"`
	Steps                  []TestStep     `json:"steps,omitempty"`
	Origin                 string         `json:"origin,omitempty"`
	Framework              string         `json:"framework,omitempty"`
	ExternalID             string         `json:"externalId,omitempty"`
	ExternalPath           string         `json:"externalPath,omitempty"`
	FilePath               string         `json:"filePath,omitempty"`
	BranchStatus           string         `json:"branchStatus,omitempty"`
	ChildrenCount          int            `json:"childrenCount,omitempty"`
	DescendantCaseCount    int            `json:"descendantCaseCount,omitempty"`
	LinkedRequirementCount int            `json:"linkedRequirementCount,omitempty"`
	CreatedAt              string         `json:"createdAt,omitempty"`
	UpdatedAt              string         `json:"updatedAt,omitempty"`
}
