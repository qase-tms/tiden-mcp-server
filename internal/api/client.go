package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"runtime"
	"time"

	"github.com/qase-tms/tiden-mcp-server/internal/version"
)

// pathf builds a request path, escaping each arg with url.PathEscape before
// interpolating. Query strings must not be passed as args - append them
// separately (e.g. pathf(...) + "?" + q.Encode()).
func pathf(format string, args ...string) string {
	escaped := make([]any, len(args))
	for i, a := range args {
		escaped[i] = url.PathEscape(a)
	}
	return fmt.Sprintf(format, escaped...)
}

var (
	ErrUnauthorized = errors.New("API token is invalid or expired")
	ErrForbidden    = errors.New("permission denied")
	ErrNotFound     = errors.New("resource not found")
	ErrRateLimited  = errors.New("rate limited after retries")
	ErrServerError  = errors.New("server error after retries")
)

type APIError struct {
	HTTPStatus int
	// Code is the gRPC status code from the grpc-gateway error envelope
	// ({"code": int, "message": string, "details": [...]}), 0 if absent.
	Code    int
	Message string
	Details []json.RawMessage
	Raw     string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("HTTP %d: %s", e.HTTPStatus, e.Message)
}

type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
	maxRetries int
	backoff    time.Duration
}

var userAgent = "tiden-mcp-server/" + version.Get() + " (" + runtime.GOOS + "/" + runtime.GOARCH + ")"

func New(baseURL, token string) *Client {
	return NewWithTimeout(baseURL, token, 30*time.Second)
}

func NewWithTimeout(baseURL, token string, timeout time.Duration) *Client {
	return &Client{
		baseURL: baseURL,
		token:   token,
		httpClient: &http.Client{
			Timeout: timeout,
		},
		maxRetries: 3,
		backoff:    time.Second,
	}
}

// WithTimeout returns a shallow copy of the client whose HTTP timeout is d.
// Used by long-running endpoints (intent distill runs an LLM server-side).
func (c *Client) WithTimeout(d time.Duration) *Client {
	cp := *c
	cp.httpClient = &http.Client{Timeout: d}
	return &cp
}

// Do executes an HTTP request with auth. It retries with exponential backoff
// on 429/5xx (any method - pre-existing behavior: a 5xx after a committed
// write may re-send a POST) and on transport errors for idempotent methods
// only. Note a retried DELETE whose first attempt succeeded server-side can
// surface ErrNotFound.
func (c *Client) Do(ctx context.Context, method, path string, body any, result any) error {
	var payload []byte
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request: %w", err)
		}
		payload = data
	}

	backoff := c.backoff

	for attempt := 0; ; attempt++ {
		var bodyReader io.Reader
		if payload != nil {
			bodyReader = bytes.NewReader(payload)
		}
		req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bodyReader)
		if err != nil {
			return fmt.Errorf("create request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+c.token)
		req.Header.Set("User-Agent", userAgent)
		if payload != nil {
			req.Header.Set("Content-Type", "application/json")
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if attempt < c.maxRetries && idempotent(method) {
				if serr := sleepCtx(ctx, backoff); serr != nil {
					return serr
				}
				backoff *= 2
				continue
			}
			return fmt.Errorf("request failed: %w", err)
		}

		respBody, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()

		switch {
		case resp.StatusCode >= 200 && resp.StatusCode < 300:
			if readErr != nil {
				return fmt.Errorf("read response: %w", readErr)
			}
			if result != nil && len(respBody) > 0 {
				if err := json.Unmarshal(respBody, result); err != nil {
					return fmt.Errorf("decode response: %w", err)
				}
			}
			return nil

		case resp.StatusCode == http.StatusUnauthorized:
			return ErrUnauthorized

		case resp.StatusCode == http.StatusForbidden:
			return fmt.Errorf("%w: %s", ErrForbidden, extractMessage(respBody))

		case resp.StatusCode == http.StatusNotFound:
			return fmt.Errorf("%w: %s", ErrNotFound, extractMessage(respBody))

		case resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500:
			if attempt == c.maxRetries {
				if resp.StatusCode == http.StatusTooManyRequests {
					return ErrRateLimited
				}
				return fmt.Errorf("%w: HTTP %d", ErrServerError, resp.StatusCode)
			}
			if serr := sleepCtx(ctx, backoff); serr != nil {
				return serr
			}
			backoff *= 2
			continue

		default:
			return newAPIError(resp.StatusCode, respBody)
		}
	}
}

func idempotent(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodPut, http.MethodDelete:
		return true
	}
	return false
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func newAPIError(httpStatus int, body []byte) *APIError {
	msg, code, details := parseErrorBody(body)
	return &APIError{
		HTTPStatus: httpStatus,
		Code:       code,
		Message:    msg,
		Details:    details,
		Raw:        string(body),
	}
}

// parseErrorBody parses the grpc-gateway error envelope. For non-JSON or
// unexpected bodies it falls back to the (truncated) raw body as the message.
func parseErrorBody(body []byte) (msg string, code int, details []json.RawMessage) {
	var parsed struct {
		Code    int               `json:"code"`
		Message string            `json:"message"`
		Details []json.RawMessage `json:"details"`
	}
	if err := json.Unmarshal(body, &parsed); err == nil && parsed.Message != "" {
		return parsed.Message, parsed.Code, parsed.Details
	}
	if len(body) > 200 {
		return string(body[:200]) + "...", 0, nil
	}
	return string(body), 0, nil
}

func extractMessage(body []byte) string {
	msg, _, _ := parseErrorBody(body)
	return msg
}
