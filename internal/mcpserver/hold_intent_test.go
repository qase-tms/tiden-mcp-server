package mcpserver_test

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

// holdRequest is what the fake server saw on the hold endpoint.
type holdRequest struct {
	method string
	path   string
	auth   string
	body   map[string]any
}

// holdRecorder answers every request with the given status and body and keeps
// each request it received.
type holdRecorder struct {
	mu       sync.Mutex
	requests []holdRequest
	status   int
	response string
}

func (h *holdRecorder) handler(t *testing.T) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var body map[string]any
		if len(raw) > 0 {
			if err := json.Unmarshal(raw, &body); err != nil {
				t.Errorf("request body is not JSON: %v: %s", err, raw)
			}
		}
		h.mu.Lock()
		h.requests = append(h.requests, holdRequest{method: r.Method, path: r.URL.Path, auth: r.Header.Get("Authorization"), body: body})
		h.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		status := h.status
		if status == 0 {
			status = http.StatusOK
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(h.response))
	}
}

func (h *holdRecorder) count() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.requests)
}

func (h *holdRecorder) only(t *testing.T) holdRequest {
	t.Helper()
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.requests) != 1 {
		t.Fatalf("server saw %d requests, want 1", len(h.requests))
	}
	return h.requests[0]
}

const heldState = `{"autoMerge":{"state":"countdown","holdUntil":"2026-11-01T09:00:00Z","holdReason":"PR 2 next","heldByName":"Ada","holdSource":"agent"}}`

func autoMergeOf(t *testing.T, body map[string]any) map[string]any {
	t.Helper()
	am, ok := body["autoMerge"].(map[string]any)
	if !ok {
		t.Fatalf("autoMerge missing from result: %v", body)
	}
	return am
}

func TestHoldIntentForSetsHoldThroughTheEndpoint(t *testing.T) {
	rec := &holdRecorder{response: heldState}
	before := time.Now()
	body, isErr := callTool(t, rec.handler(t), "hold_intent", map[string]any{
		"branch_id": "branch-1",
		"for":       "3d",
		"reason":    "PR 2 next",
	})
	after := time.Now()
	if isErr {
		t.Fatalf("tool error: %v", body)
	}

	req := rec.only(t)
	if req.method != http.MethodPost || req.path != "/v1/branches/branch-1/intent-hold" {
		t.Errorf("request = %s %s, want POST /v1/branches/branch-1/intent-hold", req.method, req.path)
	}
	if req.auth != "Bearer token" {
		t.Errorf("auth = %q", req.auth)
	}
	if req.body["reason"] != "PR 2 next" {
		t.Errorf("reason = %v", req.body["reason"])
	}
	if req.body["source"] != "agent" {
		t.Errorf("source = %v, want agent", req.body["source"])
	}
	if _, set := req.body["clear"]; set {
		t.Errorf("clear sent on a set request: %v", req.body)
	}
	untilText, _ := req.body["until"].(string)
	until, err := time.Parse(time.RFC3339, untilText)
	if err != nil {
		t.Fatalf("until %q is not RFC 3339: %v", untilText, err)
	}
	// The wire format drops sub-second precision, so allow one second below.
	low, high := before.Add(72*time.Hour).Add(-time.Second), after.Add(72*time.Hour)
	if until.Before(low) || until.After(high) {
		t.Errorf("until = %s, want between %s and %s (3 days from now)", until, low, high)
	}

	am := autoMergeOf(t, body)
	if am["state"] != "countdown" || am["holdUntil"] != "2026-11-01T09:00:00Z" || am["holdReason"] != "PR 2 next" || am["heldByName"] != "Ada" || am["holdSource"] != "agent" {
		t.Errorf("hold state not returned: %v", am)
	}
}

func TestHoldIntentClearReleasesTheHold(t *testing.T) {
	rec := &holdRecorder{response: `{"autoMerge":{"state":"countdown","nextCheckAt":"2026-10-22T09:00:00Z"}}`}
	body, isErr := callTool(t, rec.handler(t), "hold_intent", map[string]any{
		"branch_id": "branch-1",
		"clear":     true,
	})
	if isErr {
		t.Fatalf("tool error: %v", body)
	}
	req := rec.only(t)
	if req.path != "/v1/branches/branch-1/intent-hold" {
		t.Errorf("path = %s", req.path)
	}
	if req.body["clear"] != true || req.body["source"] != "agent" {
		t.Errorf("body = %v, want clear=true and source=agent", req.body)
	}
	if _, set := req.body["until"]; set {
		t.Errorf("until sent on a clear request: %v", req.body)
	}
	am := autoMergeOf(t, body)
	if am["state"] != "countdown" || am["nextCheckAt"] != "2026-10-22T09:00:00Z" {
		t.Errorf("state after release = %v", am)
	}
	if _, held := am["holdUntil"]; held {
		t.Errorf("released hold still reported: %v", am)
	}
}

func TestHoldIntentClearMayCarryAReason(t *testing.T) {
	rec := &holdRecorder{response: `{"autoMerge":{"state":"countdown"}}`}
	body, isErr := callTool(t, rec.handler(t), "hold_intent", map[string]any{
		"branch_id": "branch-1",
		"clear":     true,
		"reason":    "  plan changed ",
	})
	if isErr {
		t.Fatalf("tool error: %v", body)
	}
	if got := rec.only(t).body["reason"]; got != "plan changed" {
		t.Errorf("reason = %v, want the trimmed text", got)
	}
}

func TestHoldIntentWithoutForOrUntilHoldsSevenDays(t *testing.T) {
	rec := &holdRecorder{response: heldState}
	before := time.Now()
	body, isErr := callTool(t, rec.handler(t), "hold_intent", map[string]any{"branch_id": "b", "reason": "more PRs"})
	after := time.Now()
	if isErr {
		t.Fatalf("tool error: %v", body)
	}
	until, err := time.Parse(time.RFC3339, rec.only(t).body["until"].(string))
	if err != nil {
		t.Fatal(err)
	}
	if until.Before(before.Add(7*24*time.Hour).Add(-time.Second)) || until.After(after.Add(7*24*time.Hour)) {
		t.Errorf("until = %s, want 7 days from now", until)
	}
}

func TestHoldIntentUntilIsSentAsUTC(t *testing.T) {
	rec := &holdRecorder{response: heldState}
	body, isErr := callTool(t, rec.handler(t), "hold_intent", map[string]any{
		"branch_id": "b",
		"reason":    "release freeze",
		"until":     "2026-11-01T12:00:00+03:00",
	})
	if isErr {
		t.Fatalf("tool error: %v", body)
	}
	if got := rec.only(t).body["until"]; got != "2026-11-01T09:00:00Z" {
		t.Errorf("until = %v, want 2026-11-01T09:00:00Z", got)
	}
}

func TestHoldIntentHourDurations(t *testing.T) {
	rec := &holdRecorder{response: heldState}
	before := time.Now()
	if body, isErr := callTool(t, rec.handler(t), "hold_intent", map[string]any{"branch_id": "b", "reason": "r", "for": "12h"}); isErr {
		t.Fatalf("tool error: %v", body)
	}
	until, _ := time.Parse(time.RFC3339, rec.only(t).body["until"].(string))
	if d := until.Sub(before); d < 12*time.Hour-2*time.Second || d > 12*time.Hour+2*time.Second {
		t.Errorf("hold lasts %s, want about 12h", d)
	}
}

func TestHoldIntentRefusesBadInputWithoutARequest(t *testing.T) {
	cases := []struct {
		name string
		args map[string]any
		want string
	}{
		{"no reason", map[string]any{"branch_id": "b", "for": "3d"}, "reason is required"},
		{"blank reason", map[string]any{"branch_id": "b", "reason": "   ", "for": "3d"}, "reason is required"},
		{"no branch", map[string]any{"branch_id": "", "reason": "r"}, "branch_id is required"},
		{"for and until", map[string]any{"branch_id": "b", "reason": "r", "for": "3d", "until": "2026-11-01T09:00:00Z"}, "either for or until"},
		{"clear with for", map[string]any{"branch_id": "b", "clear": true, "for": "3d"}, "do not pass for or until"},
		{"clear with until", map[string]any{"branch_id": "b", "clear": true, "until": "2026-11-01T09:00:00Z"}, "do not pass for or until"},
		{"zero days", map[string]any{"branch_id": "b", "reason": "r", "for": "0d"}, "positive duration"},
		{"negative hours", map[string]any{"branch_id": "b", "reason": "r", "for": "-2h"}, "positive duration"},
		{"not a duration", map[string]any{"branch_id": "b", "reason": "r", "for": "soon"}, "positive duration"},
		{"fractional days", map[string]any{"branch_id": "b", "reason": "r", "for": "1.5d"}, "positive duration"},
		{"huge days", map[string]any{"branch_id": "b", "reason": "r", "for": "999999999999d"}, "positive duration"},
		{"date only until", map[string]any{"branch_id": "b", "reason": "r", "until": "2026-11-01"}, "RFC 3339"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := &holdRecorder{response: heldState}
			body, isErr := callTool(t, rec.handler(t), "hold_intent", tc.args)
			if !isErr {
				t.Fatalf("want a tool error, got %v", body)
			}
			if msg, _ := body["error"].(string); !strings.Contains(msg, tc.want) {
				t.Errorf("error %q does not contain %q", msg, tc.want)
			}
			if n := rec.count(); n != 0 {
				t.Errorf("%d request(s) reached the server, want none", n)
			}
		})
	}
}

func TestHoldIntentServerRefusalReachesTheAgent(t *testing.T) {
	cases := []struct {
		name    string
		status  int
		body    string
		wantMsg string
	}{
		{
			"hold invalid",
			http.StatusBadRequest,
			`{"code":3,"message":"INTENT_HOLD_INVALID: until is in the past","details":[]}`,
			"INTENT_HOLD_INVALID: until is in the past",
		},
		{
			"branch not open",
			http.StatusBadRequest,
			`{"code":9,"message":"BRANCH_NOT_OPEN: branch is already merged","details":[]}`,
			"BRANCH_NOT_OPEN: branch is already merged",
		},
		{
			"branch not found",
			http.StatusNotFound,
			`{"code":5,"message":"branch not found","details":[]}`,
			"branch not found",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := &holdRecorder{status: tc.status, response: tc.body}
			body, isErr := callTool(t, rec.handler(t), "hold_intent", map[string]any{"branch_id": "b", "reason": "r", "for": "3d"})
			if !isErr {
				t.Fatalf("want a tool error, got %v", body)
			}
			if msg, _ := body["error"].(string); !strings.Contains(msg, tc.wantMsg) {
				t.Errorf("error %q does not carry the server message %q", msg, tc.wantMsg)
			}
			if n := rec.count(); n != 1 {
				t.Errorf("server saw %d requests, want 1", n)
			}
		})
	}
}

func TestHoldIntentEscapesTheBranchID(t *testing.T) {
	rec := &holdRecorder{response: heldState}
	if body, isErr := callTool(t, rec.handler(t), "hold_intent", map[string]any{"branch_id": "a/b c", "reason": "r"}); isErr {
		t.Fatalf("tool error: %v", body)
	}
	// r.URL.Path is the decoded form; the slash stays inside one segment only
	// when it was escaped, so the decoded path still carries it.
	if got := rec.only(t).path; got != "/v1/branches/a/b c/intent-hold" {
		t.Errorf("path = %q", got)
	}
}

func TestListBranchesAndMergePreviewPassAutoMergeThrough(t *testing.T) {
	list, isErr := callTool(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/products/p1/branches" {
			t.Errorf("path = %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"branches":[
		  {"id":"b1","name":"intent/2026-10-20-x","status":"open","autoMerge":{"state":"partial","openPromises":["export works offline"],"partialUntil":"2026-10-28T09:00:00Z"}},
		  {"id":"b2","name":"intent/2026-10-10-y","status":"merged","mergeMode":"auto","mergedByName":"Tiden auto-merge","continuesBranchId":"b0"}]}`))
	}, "list_branches", map[string]any{"product_id": "p1"})
	if isErr {
		t.Fatalf("tool error: %v", list)
	}
	branches := list["branches"].([]any)
	first := branches[0].(map[string]any)["autoMerge"].(map[string]any)
	if first["state"] != "partial" || first["partialUntil"] != "2026-10-28T09:00:00Z" {
		t.Errorf("autoMerge = %v", first)
	}
	if promises, _ := first["openPromises"].([]any); len(promises) != 1 || promises[0] != "export works offline" {
		t.Errorf("openPromises = %v", first["openPromises"])
	}
	second := branches[1].(map[string]any)
	if second["mergeMode"] != "auto" || second["mergedByName"] != "Tiden auto-merge" || second["continuesBranchId"] != "b0" {
		t.Errorf("merged branch fields = %v", second)
	}

	preview, isErr := callTool(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"additions":[],"modifications":[],"deletions":[],"stats":{"additions":0,"modifications":0,"deletions":0,"conflicts":0},"autoMerge":{"state":"countdown","nextCheckAt":"2026-10-22T09:00:00Z","reason":"waiting_for_quiet_period"}}`))
	}, "get_merge_preview", map[string]any{"branch_id": "b1"})
	if isErr {
		t.Fatalf("tool error: %v", preview)
	}
	am := autoMergeOf(t, preview)
	if am["state"] != "countdown" || am["nextCheckAt"] != "2026-10-22T09:00:00Z" || am["reason"] != "waiting_for_quiet_period" {
		t.Errorf("merge preview autoMerge = %v", am)
	}
}
