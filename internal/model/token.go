package model

// Token kinds and scopes as the API spells them (api.v1.ApiTokenKind /
// ApiTokenScope enum names, protojson).
const (
	TokenKindPersonal = "API_TOKEN_KIND_PERSONAL"
	TokenKindProduct  = "API_TOKEN_KIND_PRODUCT"

	TokenScopeFull        = "API_TOKEN_SCOPE_FULL"
	TokenScopePublishRuns = "API_TOKEN_SCOPE_PUBLISH_RUNS"
)

// TokenInfo describes the API token a call was made with (GET /v1/auth/token).
// ProductID and WorkspaceID are set only for a product token: the one product
// it may reach, and that product's workspace.
type TokenInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Kind        string `json:"kind"`
	Scope       string `json:"scope"`
	ProductID   string `json:"productId,omitempty"`
	WorkspaceID string `json:"workspaceId,omitempty"`
	IntentOnly  bool   `json:"intentOnly,omitempty"`
	ExpiresAt   string `json:"expiresAt,omitempty"`
}

// IsProduct reports a product token: one that belongs to a single product and
// is refused on every other one.
func (t *TokenInfo) IsProduct() bool {
	return t != nil && t.Kind == TokenKindProduct
}

// IsPublishOnly reports a token that may only publish test runs.
func (t *TokenInfo) IsPublishOnly() bool {
	return t != nil && t.Scope == TokenScopePublishRuns
}

// KindLabel is the token kind in words: "product token", "personal token",
// or "API token" when the server did not say.
func (t *TokenInfo) KindLabel() string {
	switch {
	case t == nil:
		return "API token"
	case t.Kind == TokenKindProduct:
		return "product token"
	case t.Kind == TokenKindPersonal:
		return "personal token"
	default:
		return "API token"
	}
}

// ScopeLabel is the token scope in words.
func (t *TokenInfo) ScopeLabel() string {
	switch {
	case t == nil:
		return ""
	case t.Scope == TokenScopePublishRuns:
		return "publish-only"
	case t.Scope == TokenScopeFull:
		return "full access"
	default:
		return ""
	}
}
