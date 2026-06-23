package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func newTestClient(baseURL string) *Client {
	c := New(baseURL, "tok")
	c.backoff = time.Millisecond
	return c
}

func TestDo_Accepts2xx(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   string
	}{
		{"201 created with body", http.StatusCreated, `{"id":"x"}`},
		{"204 no content empty body", http.StatusNoContent, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				if tc.body != "" {
					_, _ = w.Write([]byte(tc.body))
				}
			}))
			defer srv.Close()

			var result struct {
				ID string `json:"id"`
			}
			if err := newTestClient(srv.URL).Do(context.Background(), "POST", "/v1/things", map[string]string{"a": "b"}, &result); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.body != "" && result.ID != "x" {
				t.Errorf("result = %+v", result)
			}
		})
	}
}

func TestDo_RetriesOn429And500ThenSucceeds(t *testing.T) {
	for _, failStatus := range []int{http.StatusTooManyRequests, http.StatusInternalServerError} {
		var calls atomic.Int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if calls.Add(1) <= 2 {
				w.WriteHeader(failStatus)
				return
			}
			_, _ = w.Write([]byte(`{}`))
		}))

		err := newTestClient(srv.URL).Do(context.Background(), "GET", "/v1/things", nil, nil)
		srv.Close()
		if err != nil {
			t.Errorf("status %d: unexpected error: %v", failStatus, err)
		}
		if got := calls.Load(); got != 3 {
			t.Errorf("status %d: calls = %d, want 3", failStatus, got)
		}
	}
}

func TestDo_RetriesExhausted(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	err := newTestClient(srv.URL).Do(context.Background(), "GET", "/v1/things", nil, nil)
	if !errors.Is(err, ErrServerError) {
		t.Fatalf("err = %v, want ErrServerError", err)
	}
	if got := calls.Load(); got != 4 {
		t.Errorf("calls = %d, want 4 (initial + 3 retries)", got)
	}
}

// flakyConnServer closes the TCP connection (before writing a response) for
// the first n requests, then serves normally.
func flakyConnServer(t *testing.T, n int32, calls *atomic.Int32) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) <= n {
			hj, ok := w.(http.Hijacker)
			if !ok {
				t.Fatal("server does not support hijacking")
			}
			conn, _, err := hj.Hijack()
			if err != nil {
				t.Fatalf("hijack: %v", err)
			}
			_ = conn.Close()
			return
		}
		_, _ = w.Write([]byte(`{}`))
	}))
}

func TestDo_TransportErrorRetriedForGET(t *testing.T) {
	var calls atomic.Int32
	srv := flakyConnServer(t, 2, &calls)
	defer srv.Close()

	if err := newTestClient(srv.URL).Do(context.Background(), "GET", "/v1/things", nil, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := calls.Load(); got != 3 {
		t.Errorf("calls = %d, want 3", got)
	}
}

func TestDo_TransportErrorNotRetriedForPOST(t *testing.T) {
	var calls atomic.Int32
	srv := flakyConnServer(t, 100, &calls)
	defer srv.Close()

	err := newTestClient(srv.URL).Do(context.Background(), "POST", "/v1/things", map[string]string{"a": "b"}, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	// Go's transport may transparently retry a request once on a connection
	// that was closed before any response bytes arrived, so allow 1 or 2
	// underlying calls - the client itself must not add retry attempts.
	if got := calls.Load(); got > 2 {
		t.Errorf("calls = %d, want <= 2 (no client-level retries for POST)", got)
	}
}

func TestDo_ContextCancelDuringBackoffReturnsPromptly(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := New(srv.URL, "tok") // real 1s backoff: cancellation must beat it
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	err := c.Do(ctx, "GET", "/v1/things", nil, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Errorf("returned after %v, want prompt return on cancel", elapsed)
	}
}

func TestDo_SetsUserAgent(t *testing.T) {
	var gotUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	if err := newTestClient(srv.URL).Do(context.Background(), "GET", "/v1/things", nil, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(gotUA, "tiden-mcp-server/") {
		t.Errorf("User-Agent = %q, want tiden-mcp-server/ prefix", gotUA)
	}
}

func TestDo_ParsesGatewayErrorEnvelope(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"code":3,"message":"title is required","details":[{"@type":"type.example/Foo","field":"title"}]}`))
	}))
	defer srv.Close()

	err := newTestClient(srv.URL).Do(context.Background(), "POST", "/v1/things", map[string]string{}, nil)
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("err = %v, want *APIError", err)
	}
	if apiErr.HTTPStatus != http.StatusBadRequest || apiErr.Code != 3 || apiErr.Message != "title is required" {
		t.Errorf("apiErr = %+v", apiErr)
	}
	if len(apiErr.Details) != 1 {
		t.Errorf("details = %v, want 1 entry", apiErr.Details)
	}
}

func TestDo_BodyReadErrorSurfaces(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Promise more bytes than we send, then return: client sees an
		// unexpected EOF while reading the body.
		w.Header().Set("Content-Length", "1000")
		_, _ = w.Write([]byte(`{"par`))
	}))
	defer srv.Close()

	var result map[string]any
	// POST so the transport error path doesn't retry.
	err := newTestClient(srv.URL).Do(context.Background(), "POST", "/v1/things", map[string]string{}, &result)
	if err == nil {
		t.Fatal("expected error from truncated body")
	}
	if !strings.Contains(err.Error(), "read response") {
		t.Errorf("err = %v, want read response error", err)
	}
}

func TestDo_BodyResentOnRetry(t *testing.T) {
	var calls atomic.Int32
	var bodies []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(b)
		bodies = append(bodies, string(b))
		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	if err := newTestClient(srv.URL).Do(context.Background(), "POST", "/v1/things", map[string]string{"a": "b"}, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(bodies) != 2 || bodies[0] != bodies[1] || bodies[0] != `{"a":"b"}` {
		t.Errorf("bodies = %q, want identical %q twice", bodies, `{"a":"b"}`)
	}
}
