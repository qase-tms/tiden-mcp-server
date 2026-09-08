package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestListRequirementIdentitiesPage(t *testing.T) {
	hash1 := "a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1"
	hash2 := "b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2"
	rowA := `{"id":"r1","productId":"p","sources":[],"contentSha256":"` + hash1 + `","trimmedContentSha256":"` + hash2 + `"}`
	rowB := `{"id":"r2","productId":"p","sources":[],"contentSha256":"` + hash2 + `","trimmedContentSha256":"` + hash1 + `"}`

	for _, tc := range []struct {
		name, token, body string
		size              int
		wantError         bool
	}{
		{
			name: "happy_path_no_continuation",
			body: `{"view":"identity","identities":[` + rowA + `],"pagination":{"totalCount":1}}`,
		},
		{
			name: "happy_path_progresses",
			body: `{"view":"identity","identities":[` + rowA + `],"pagination":{"totalCount":2,"nextPageToken":"cursor-2"}}`,
		},
		{
			name:      "repeated_next_token",
			token:     "cursor-1",
			body:      `{"view":"identity","identities":[` + rowA + `],"pagination":{"totalCount":2,"nextPageToken":"cursor-1"}}`,
			wantError: true,
		},
		{
			name:      "next_token_with_zero_rows",
			body:      `{"view":"identity","identities":[],"pagination":{"totalCount":1,"nextPageToken":"cursor-1"}}`,
			wantError: true,
		},
		{
			name:      "oversized_page",
			size:      1,
			body:      `{"view":"identity","identities":[` + rowA + `,` + rowB + `],"pagination":{"totalCount":2}}`,
			wantError: true,
		},
		{
			name:      "duplicate_requirement_id",
			body:      `{"view":"identity","identities":[` + rowA + `,` + rowA + `],"pagination":{"totalCount":2}}`,
			wantError: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Method != "GET" || r.URL.Path != "/v1/products/p/requirements" {
					t.Errorf("request: %s %s", r.Method, r.URL)
				}
				q := r.URL.Query()
				if q.Get("view") != "identity" || q.Get("pagination.pageToken") != tc.token {
					t.Errorf("query: %v", q)
				}
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()

			got, err := New(server.URL, "token").ListRequirementIdentitiesPage(context.Background(), "p", "", tc.size, tc.token)
			if (err != nil) != tc.wantError {
				t.Fatalf("got=%+v err=%v, wantError=%v", got, err, tc.wantError)
			}
			if tc.wantError && got != nil {
				t.Fatal("partial response on failure")
			}
			if calls != 1 {
				t.Fatalf("calls = %d, want 1", calls)
			}
		})
	}
}
