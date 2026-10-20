package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBranchDecodesAutoMergeForEachState(t *testing.T) {
	cases := []struct {
		state string
		json  string
		check func(t *testing.T, am *AutoMerge)
	}{
		{"in_progress", `{"state":"in_progress","reason":"round_active","reasonDetail":"a round is active"}`, func(t *testing.T, am *AutoMerge) {
			if am.Reason != "round_active" || am.ReasonDetail != "a round is active" {
				t.Errorf("reason fields = %+v", am)
			}
		}},
		{"countdown", `{"state":"countdown","reason":"quiet_period","nextCheckAt":"2026-10-22T09:00:00Z"}`, func(t *testing.T, am *AutoMerge) {
			if am.NextCheckAt != "2026-10-22T09:00:00Z" || am.Reason != "quiet_period" {
				t.Errorf("countdown fields = %+v", am)
			}
		}},
		{"closing", `{"state":"closing"}`, nil},
		{"partial", `{"state":"partial","openPromises":["a","b"],"partialUntil":"2026-10-29T09:00:00Z"}`, func(t *testing.T, am *AutoMerge) {
			if len(am.OpenPromises) != 2 || am.OpenPromises[1] != "b" || am.PartialUntil != "2026-10-29T09:00:00Z" {
				t.Errorf("partial fields = %+v", am)
			}
		}},
		{"merged", `{"state":"merged"}`, nil},
		{"checkpoint_merged", `{"state":"checkpoint_merged","successorBranchId":"b-next","successorBranchName":"intent/2026-10-30-next"}`, func(t *testing.T, am *AutoMerge) {
			if am.SuccessorBranchID != "b-next" || am.SuccessorBranchName != "intent/2026-10-30-next" {
				t.Errorf("successor fields = %+v", am)
			}
		}},
		{"abandoned", `{"state":"abandoned","reason":"no_activity"}`, nil},
		{"off", `{"state":"off"}`, nil},
		{"held", `{"state":"countdown","holdUntil":"2026-11-01T09:00:00Z","holdReason":"PR 2 next","heldByName":"Ada","holdSource":"person"}`, func(t *testing.T, am *AutoMerge) {
			if am.HoldUntil != "2026-11-01T09:00:00Z" || am.HoldReason != "PR 2 next" || am.HeldByName != "Ada" || am.HoldSource != "person" {
				t.Errorf("hold fields = %+v", am)
			}
		}},
		{"a state added later", `{"state":"review_pending","reason":"x"}`, func(t *testing.T, am *AutoMerge) {
			if am.State != "review_pending" {
				t.Errorf("unknown state = %q, want it kept as a string", am.State)
			}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.state, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(`{"branches":[{"id":"b1","autoMerge":` + tc.json + `}]}`))
			}))
			defer srv.Close()

			resp, err := New(srv.URL, "tok").ListBranches(context.Background(), "p1")
			if err != nil {
				t.Fatal(err)
			}
			am := resp.Branches[0].AutoMerge
			if am == nil {
				t.Fatal("autoMerge not decoded")
			}
			var wantState struct {
				State string `json:"state"`
			}
			if err := json.Unmarshal([]byte(tc.json), &wantState); err != nil {
				t.Fatal(err)
			}
			if am.State != wantState.State {
				t.Errorf("state = %q, want %q", am.State, wantState.State)
			}
			if tc.check != nil {
				tc.check(t, am)
			}
		})
	}
}

func TestBranchDecodesMergeFields(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"branches":[{"id":"b1","status":"merged","mergeMode":"checkpoint","mergedByName":"Tiden auto-merge","continuesBranchId":"b0"}]}`))
	}))
	defer srv.Close()

	resp, err := New(srv.URL, "tok").ListBranches(context.Background(), "p1")
	if err != nil {
		t.Fatal(err)
	}
	b := resp.Branches[0]
	if b.MergeMode != "checkpoint" || b.MergedByName != "Tiden auto-merge" || b.ContinuesBranchID != "b0" {
		t.Errorf("branch = %+v", b)
	}
}

// A server without the feature sends none of the new fields. The branch must
// decode as before and must not grow empty fields on the way out.
func TestBranchFromOlderServerStaysUnchanged(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"branches":[{"id":"b1","productId":"p1","name":"feature/x","status":"open"}]}`))
	}))
	defer srv.Close()

	resp, err := New(srv.URL, "tok").ListBranches(context.Background(), "p1")
	if err != nil {
		t.Fatal(err)
	}
	b := resp.Branches[0]
	if b.AutoMerge != nil || b.MergeMode != "" || b.MergedByName != "" || b.ContinuesBranchID != "" {
		t.Errorf("new fields set from nothing: %+v", b)
	}
	out, err := json.Marshal(b)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"autoMerge", "mergeMode", "mergedByName", "continuesBranchId"} {
		if strings.Contains(string(out), key) {
			t.Errorf("%s appears in output %s", key, out)
		}
	}
}

func TestMergePreviewDecodesAutoMerge(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/branches/b1/merge-preview" {
			t.Errorf("path = %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"additions":[],"modifications":[],"deletions":[],"stats":{"conflicts":2},"autoMerge":{"state":"partial","openPromises":["x"],"partialUntil":"2026-10-29T09:00:00Z"}}`))
	}))
	defer srv.Close()

	resp, err := New(srv.URL, "tok").GetMergePreview(context.Background(), "b1")
	if err != nil {
		t.Fatal(err)
	}
	if resp.Stats == nil || resp.Stats.Conflicts != 2 {
		t.Errorf("stats = %+v", resp.Stats)
	}
	if resp.AutoMerge == nil || resp.AutoMerge.State != "partial" || len(resp.AutoMerge.OpenPromises) != 1 || resp.AutoMerge.PartialUntil != "2026-10-29T09:00:00Z" {
		t.Errorf("autoMerge = %+v", resp.AutoMerge)
	}
}

func TestMergePreviewFromOlderServerHasNoAutoMerge(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"additions":[],"modifications":[],"deletions":[],"stats":{}}`))
	}))
	defer srv.Close()

	resp, err := New(srv.URL, "tok").GetMergePreview(context.Background(), "b1")
	if err != nil {
		t.Fatal(err)
	}
	if resp.AutoMerge != nil {
		t.Errorf("autoMerge = %+v, want nil", resp.AutoMerge)
	}
	out, _ := json.Marshal(resp)
	if strings.Contains(string(out), "autoMerge") {
		t.Errorf("autoMerge appears in output %s", out)
	}
}

func TestSetIntentHold_PostsToHoldPath(t *testing.T) {
	var gotMethod, gotPath string
	var rawBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.EscapedPath()
		rawBody, _ = io.ReadAll(r.Body)
		_, _ = w.Write([]byte(`{"autoMerge":{"state":"countdown","holdUntil":"2026-11-01T09:00:00Z","holdReason":"PR 2 next","heldByName":"Ada","holdSource":"agent"}}`))
	}))
	defer srv.Close()

	resp, err := New(srv.URL, "tok").SetIntentHold(context.Background(), "b/1", SetIntentHoldRequest{
		Until:  "2026-11-01T09:00:00Z",
		Reason: "PR 2 next",
		Source: "agent",
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotMethod != http.MethodPost || gotPath != "/v1/branches/b%2F1/intent-hold" {
		t.Errorf("request = %s %s", gotMethod, gotPath)
	}
	var sent map[string]any
	if err := json.Unmarshal(rawBody, &sent); err != nil {
		t.Fatal(err)
	}
	if sent["until"] != "2026-11-01T09:00:00Z" || sent["reason"] != "PR 2 next" || sent["source"] != "agent" {
		t.Errorf("body = %s", rawBody)
	}
	if _, set := sent["clear"]; set {
		t.Errorf("clear sent when false: %s", rawBody)
	}
	if resp.AutoMerge == nil || resp.AutoMerge.HoldUntil != "2026-11-01T09:00:00Z" || resp.AutoMerge.HeldByName != "Ada" {
		t.Errorf("resp = %+v", resp.AutoMerge)
	}
}

func TestSetIntentHold_ClearBody(t *testing.T) {
	var rawBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rawBody, _ = io.ReadAll(r.Body)
		_, _ = w.Write([]byte(`{"autoMerge":{"state":"countdown"}}`))
	}))
	defer srv.Close()

	if _, err := New(srv.URL, "tok").SetIntentHold(context.Background(), "b1", SetIntentHoldRequest{Clear: true, Source: "agent"}); err != nil {
		t.Fatal(err)
	}
	var sent map[string]any
	if err := json.Unmarshal(rawBody, &sent); err != nil {
		t.Fatal(err)
	}
	if sent["clear"] != true || sent["source"] != "agent" {
		t.Errorf("body = %s", rawBody)
	}
	if _, set := sent["until"]; set {
		t.Errorf("until sent on clear: %s", rawBody)
	}
}

func TestSetIntentHold_ServerRefusalKeepsMessage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"code":3,"message":"INTENT_HOLD_INVALID: until is more than 30 days ahead","details":[]}`))
	}))
	defer srv.Close()

	_, err := New(srv.URL, "tok").SetIntentHold(context.Background(), "b1", SetIntentHoldRequest{Until: "2027-01-01T00:00:00Z", Reason: "r"})
	if err == nil || !strings.Contains(err.Error(), "INTENT_HOLD_INVALID: until is more than 30 days ahead") {
		t.Fatalf("err = %v, want the server message", err)
	}
}
