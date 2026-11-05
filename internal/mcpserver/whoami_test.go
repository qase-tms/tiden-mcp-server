package mcpserver_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/qase-tms/tiden-mcp-server/internal/api"
	"github.com/qase-tms/tiden-mcp-server/internal/mcpserver"
)

func callWhoami(t *testing.T, handler http.HandlerFunc) (map[string]any, bool) {
	t.Helper()
	httpServer := httptest.NewServer(handler)
	defer httpServer.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	server := mcpserver.New(api.New(httpServer.URL, "test-token"), "")
	go func() { _ = server.Run(ctx, serverTransport) }()

	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "v0"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = session.Close() }()

	res, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "whoami", Arguments: map[string]any{}})
	if err != nil {
		t.Fatal(err)
	}
	text := res.Content[0].(*mcp.TextContent).Text
	var out map[string]any
	if !res.IsError {
		if err := json.Unmarshal([]byte(text), &out); err != nil {
			t.Fatalf("whoami result is not JSON: %v\n%s", err, text)
		}
	}
	return out, res.IsError
}

func TestWhoami_ProductToken(t *testing.T) {
	out, isErr := callWhoami(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/auth/token":
			_, _ = w.Write([]byte(`{"id":"t1","name":"CI","kind":"API_TOKEN_KIND_PRODUCT","scope":"API_TOKEN_SCOPE_FULL","productId":"P1","workspaceId":"ws-1"}`))
		case "/v1/auth/me":
			_, _ = w.Write([]byte(`{"user":{"id":"t1","email":"api-token+t1@tokens.invalid","name":"CI"}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})
	if isErr {
		t.Fatal("whoami returned an error")
	}
	token, _ := out["token"].(map[string]any)
	if out["name"] != "CI" || token["kind"] != "API_TOKEN_KIND_PRODUCT" || token["productId"] != "P1" || token["workspaceId"] != "ws-1" {
		t.Fatalf("whoami = %v, want the user plus the product token", out)
	}
}

func TestWhoami_PublishOnlyTokenReturnsTokenInfo(t *testing.T) {
	out, isErr := callWhoami(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/v1/auth/token" {
			_, _ = w.Write([]byte(`{"id":"t2","name":"Reporter","kind":"API_TOKEN_KIND_PRODUCT","scope":"API_TOKEN_SCOPE_PUBLISH_RUNS","productId":"P1","workspaceId":"ws-1"}`))
			return
		}
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"code":7,"message":"this API token can only publish test runs for product P1"}`))
	})
	if isErr {
		t.Fatal("whoami returned an error for a publish-only token")
	}
	token, _ := out["token"].(map[string]any)
	if _, hasEmail := out["email"]; hasEmail || token["scope"] != "API_TOKEN_SCOPE_PUBLISH_RUNS" {
		t.Fatalf("whoami = %v, want token info only", out)
	}
}

func TestWhoami_OldServerReturnsUserOnly(t *testing.T) {
	out, isErr := callWhoami(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/v1/auth/me" {
			_, _ = w.Write([]byte(`{"user":{"id":"u1","email":"a@example.dev","name":"A"}}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"code":5,"message":"Not Found"}`))
	})
	if isErr {
		t.Fatal("whoami returned an error against an older server")
	}
	if out["email"] != "a@example.dev" || out["token"] != nil {
		t.Fatalf("whoami = %v, want the user alone", out)
	}
}

// Only a publish-only token's 403 on Me is expected. A full-access token
// refused on Me is a real error, not an empty user.
func TestWhoami_FullTokenRefusedOnMeIsAnError(t *testing.T) {
	_, isErr := callWhoami(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/v1/auth/token" {
			_, _ = w.Write([]byte(`{"id":"t1","name":"CI","kind":"API_TOKEN_KIND_PRODUCT","scope":"API_TOKEN_SCOPE_FULL","productId":"P1","workspaceId":"ws-1"}`))
			return
		}
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"code":7,"message":"permission denied"}`))
	})
	if !isErr {
		t.Fatal("whoami succeeded, want the 403 reported")
	}
}
