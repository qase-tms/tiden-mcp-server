package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
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
	m, err := client.GetTraceability(context.Background(), "p1", "branch", "", "intent/2026-08-05-x")
	if err != nil {
		t.Fatalf("GetTraceability: %v", err)
	}
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
