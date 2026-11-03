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

const verdictJSON = `{
  "id": "v1",
  "productId": "p1",
  "scope": "VERDICT_SCOPE_BRANCH",
  "branchId": "b1",
  "status": "VERDICT_STATUS_BLOCKED",
  "subjects": [{
    "subjectType": "feature",
    "subjectId": "req-root",
    "name": "Checkout",
    "status": "VERDICT_STATUS_BLOCKED",
    "riskSource": "backend",
    "issueSources": ["backend"]
  }]
}`

func TestComputeVerdictBranchBody(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/products/p1/quality-gate:compute" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.String())
		}
		data, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(data, &gotBody); err != nil {
			t.Errorf("body invalid JSON: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"verdict": ` + verdictJSON + `}`))
	}))
	defer srv.Close()

	client := newTestClient(srv.URL)
	v, err := client.ComputeVerdict(context.Background(), "p1", "branch", "", "intent/2026-08-05-x")
	if err != nil {
		t.Fatalf("ComputeVerdict: %v", err)
	}
	if gotBody["scope"] != "VERDICT_SCOPE_BRANCH" || gotBody["branch"] != "intent/2026-08-05-x" {
		t.Errorf("request body = %#v", gotBody)
	}
	if _, present := gotBody["releaseId"]; present {
		t.Errorf("releaseId must be absent for branch scope: %#v", gotBody)
	}
	if len(v.Subjects) != 1 || v.Subjects[0].SubjectType != "feature" || v.Subjects[0].RiskSource != "backend" {
		t.Errorf("subjects decode = %+v", v.Subjects)
	}
}

func TestComputeVerdictMainOmitsBranch(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(data, &gotBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"verdict": ` + verdictJSON + `}`))
	}))
	defer srv.Close()

	client := newTestClient(srv.URL)
	if _, err := client.ComputeVerdict(context.Background(), "p1", "main", "", ""); err != nil {
		t.Fatalf("ComputeVerdict: %v", err)
	}
	if gotBody["scope"] != "VERDICT_SCOPE_MAIN" {
		t.Errorf("scope = %v", gotBody["scope"])
	}
	if _, present := gotBody["branch"]; present {
		t.Errorf("branch must be absent for main scope: %#v", gotBody)
	}
}

func TestGetVerdictBranchQuery(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v1/products/p1/quality-gate" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.String())
		}
		q := r.URL.Query()
		if q.Get("scope") != "VERDICT_SCOPE_BRANCH" || q.Get("branch") != "intent/2026-08-05-x" {
			t.Errorf("query = %v", q)
		}
		if q.Has("releaseId") {
			t.Errorf("releaseId must be absent for branch scope: %v", q)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"verdict": ` + verdictJSON + `}`))
	}))
	defer srv.Close()

	client := newTestClient(srv.URL)
	if _, err := client.GetVerdict(context.Background(), "p1", "branch", "", "intent/2026-08-05-x"); err != nil {
		t.Fatalf("GetVerdict: %v", err)
	}
}

func TestGetTraceabilityBranchQueryAndDecode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v1/products/p1/quality-gate/traceability" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.String())
		}
		q := r.URL.Query()
		if q.Get("scope") != "VERDICT_SCOPE_BRANCH" || q.Get("branch") != "intent/2026-08-05-x" {
			t.Errorf("query = %v", q)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"matrix": {"components": [{
		  "componentId": "c1", "name": "backend", "status": "VERDICT_STATUS_PASS",
		  "repository": "example-org/checkout",
		  "requirements": [{
		    "requirementId": "r1", "display": "QA-57", "title": "Session progress",
		    "parentId": "r0", "branchStatus": "modified", "coverage": "verified",
		    "cells": [{"display": "TC-3", "testCase": "reports progress", "status": "passed"}]
		  }]
		}]}}`))
	}))
	defer srv.Close()

	client := newTestClient(srv.URL)
	res, err := client.GetTraceabilityView(context.Background(), "p1", "branch", "", "intent/2026-08-05-x", TraceabilityOptions{})
	if err != nil {
		t.Fatalf("GetTraceabilityView: %v", err)
	}
	m := res.Matrix
	if len(m.Components) != 1 || m.Components[0].Repository != "example-org/checkout" {
		t.Errorf("components = %+v", m.Components)
	}
	req := m.Components[0].Requirements[0]
	if req.Title != "Session progress" || req.ParentID != "r0" || req.BranchStatus != "modified" {
		t.Errorf("requirement decode = %+v", req)
	}
}

func TestGetVerdictReleaseQuery(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("scope") != "VERDICT_SCOPE_RELEASE" || q.Get("releaseId") != "rel-1" {
			t.Errorf("query = %v", q)
		}
		if q.Has("branch") {
			t.Errorf("branch must be absent for release scope: %v", q)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"verdict": ` + verdictJSON + `}`))
	}))
	defer srv.Close()

	client := newTestClient(srv.URL)
	if _, err := client.GetVerdict(context.Background(), "p1", "release", "rel-1", ""); err != nil {
		t.Fatalf("GetVerdict: %v", err)
	}
}

// TestGetVerdict_ScopeAndProvenanceSurviveTheRoundTrip pins that a field the
// server publishes reaches the agent. This model is what a tool answer is
// decoded into and re-encoded from, so a field absent here is silently dropped:
// the exclusion counts, the sentences that name them, and the per-requirement
// provenance would all vanish between the server and the agent, which is
// exactly the "the verdict never reached the reader" failure the branch scope
// work exists to fix — one layer further down.
func TestGetVerdict_ScopeAndProvenanceSurviveTheRoundTrip(t *testing.T) {
	const body = `{"verdict":{"id":"v1","productId":"p1","scope":"VERDICT_SCOPE_BRANCH",
		"status":"VERDICT_STATUS_NOT_VERIFIED",
		"scopeInfo":{"source":"unmeasured","touchedRequirements":0,"touchedComponents":0,
			"bySource":{"changed":1,"directory_not_scoped":37,"retrieval_not_scoped":28},
			"notes":["37 requirement(s) are anchored in the same folders as this branch's changes but were not graded."],
			"diverged":true},
		"subjects":[{"subjectType":"component","subjectId":"c1","name":"Workspace","status":"VERDICT_STATUS_NOT_VERIFIED",
			"requirements":[{"requirementId":"r1","display":"QASE-716","state":"no_test","touched":"neighbour","uncoveredOnMain":true}]}]}}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, body)
	}))
	defer srv.Close()

	c := New(srv.URL, "tok")
	v, err := c.GetVerdict(context.Background(), "p1", "branch", "", "intent/x")
	if err != nil {
		t.Fatalf("GetVerdict: %v", err)
	}
	si := v.ScopeInfo
	if si == nil {
		t.Fatal("scopeInfo was dropped entirely")
	}
	if si.Source != "unmeasured" {
		t.Errorf("source = %q, want the named absence", si.Source)
	}
	if si.BySource["directory_not_scoped"] != 37 || si.BySource["retrieval_not_scoped"] != 28 {
		t.Errorf("by_source = %v, want the exclusion counts", si.BySource)
	}
	if len(si.Notes) != 1 {
		t.Errorf("notes = %v, want the sentence that names the exclusion", si.Notes)
	}
	if !si.Diverged {
		t.Error("diverged was dropped, so the session's own reconcile disagreement stays invisible")
	}
	if len(v.Subjects) != 1 || len(v.Subjects[0].Requirements) != 1 {
		t.Fatalf("subjects = %+v", v.Subjects)
	}
	req := v.Subjects[0].Requirements[0]
	if req.Touched != "neighbour" {
		t.Errorf("touched = %q, want the provenance that tells own work from adjacent", req.Touched)
	}
	if !req.UncoveredOnMain {
		t.Error("uncoveredOnMain was dropped, so pre-existing debt reads as this session's failure")
	}
	// And it survives being handed on: a tool re-encodes what it decoded.
	out, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, want := range []string{"directory_not_scoped", "notes", "diverged", `"touched":"neighbour"`, "uncoveredOnMain"} {
		if !strings.Contains(string(out), want) {
			t.Errorf("re-encoded verdict lost %q: %s", want, out)
		}
	}
}

// A branch verdict's inherited rows (AICP-107) and its change record (AICP-108)
// must survive the decode and the re-encode: the model is the only shape a tool
// hands on, so a field missing here never reaches the agent.
func TestGetVerdict_InheritedRowsAndMainChangeSurviveTheRoundTrip(t *testing.T) {
	const body = `{"verdict":{"id":"v1","productId":"p1","scope":"VERDICT_SCOPE_BRANCH",
		"status":"VERDICT_STATUS_NOT_VERIFIED",
		"inheritedAfterPass":[{"requirementId":"r9","display":"TIDEN-2210","state":"no_test",
			"viaFiles":["app/frontend/src/types/qualityGate.ts"],"addedOnMainAt":"2026-10-31T16:44:10Z",
			"mergedIntent":"intent/2026-10-27-a-spec-in-a-planned-status"}],
		"mainChange":{"fromStatus":"VERDICT_STATUS_PASS","toStatus":"VERDICT_STATUS_NOT_VERIFIED",
			"intentBranch":"intent/2026-10-27-a-spec-in-a-planned-status","intentBranchId":"b2",
			"mergedAt":"2026-10-31T16:45:21Z",
			"rows":[{"requirementId":"r1","display":"TIDEN-147","touched":"changed","state":"failing",
				"failingTests":["TC-1 Gate rows"]}]}}}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, body)
	}))
	defer srv.Close()

	v, err := New(srv.URL, "tok").GetVerdict(context.Background(), "p1", "branch", "", "intent/x")
	if err != nil {
		t.Fatalf("GetVerdict: %v", err)
	}
	if len(v.InheritedAfterPass) != 1 || v.InheritedAfterPass[0].ViaFiles[0] != "app/frontend/src/types/qualityGate.ts" ||
		v.InheritedAfterPass[0].MergedIntent == "" || v.InheritedAfterPass[0].AddedOnMainAt == "" {
		t.Fatalf("inheritedAfterPass = %+v, want the row with its file, time and intent", v.InheritedAfterPass)
	}
	mc := v.MainChange
	if mc == nil {
		t.Fatal("mainChange was dropped entirely")
	}
	if mc.FromStatus != "VERDICT_STATUS_PASS" || mc.ToStatus != "VERDICT_STATUS_NOT_VERIFIED" ||
		mc.IntentBranch == "" || mc.IntentBranchID != "b2" || mc.MergedAt == "" {
		t.Errorf("mainChange = %+v, want the status move, the intent and the time", mc)
	}
	if len(mc.Rows) != 1 || mc.Rows[0].State != "failing" || len(mc.Rows[0].FailingTests) != 1 || mc.Rows[0].Touched != "changed" {
		t.Errorf("mainChange.rows = %+v, want the failing row", mc.Rows)
	}
	out, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, want := range []string{`"inheritedAfterPass"`, `"viaFiles"`, `"mergedIntent"`, `"mainChange"`, `"fromStatus"`, `"failingTests"`} {
		if !strings.Contains(string(out), want) {
			t.Errorf("re-encoded verdict lost %q: %s", want, out)
		}
	}
}
