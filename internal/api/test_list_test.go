package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestListTestsFollowsPages(t *testing.T) {
	var requests []url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		requests = append(requests, q)
		w.Header().Set("Content-Type", "application/json")
		switch q.Get("pagination.pageToken") {
		case "":
			_, _ = w.Write([]byte(`{"tests":[{"id":"t1","kind":"case"},{"id":"t2","kind":"case"}],"pagination":{"nextPageToken":"1","totalCount":5}}`))
		case "1":
			_, _ = w.Write([]byte(`{"tests":[{"id":"t3","kind":"case"},{"id":"t4","kind":"case"}],"pagination":{"nextPageToken":"2","totalCount":5}}`))
		case "2":
			_, _ = w.Write([]byte(`{"tests":[{"id":"t5","kind":"case"}],"pagination":{"nextPageToken":"","totalCount":5}}`))
		default:
			t.Errorf("unexpected pagination.pageToken %q", q.Get("pagination.pageToken"))
		}
	}))
	defer srv.Close()

	client := newTestClient(srv.URL)
	resp, err := client.ListTests(context.Background(), "p1", "feature/x")
	if err != nil {
		t.Fatalf("ListTests: %v", err)
	}

	if len(requests) != 3 {
		t.Fatalf("requests = %d, want 3", len(requests))
	}
	for i, q := range requests {
		if q.Get("pagination.pageSize") != "200" {
			t.Errorf("request %d: pagination.pageSize = %q, want 200", i, q.Get("pagination.pageSize"))
		}
		if q.Get("branch") != "feature/x" {
			t.Errorf("request %d: branch = %q, want feature/x", i, q.Get("branch"))
		}
	}

	wantIDs := []string{"t1", "t2", "t3", "t4", "t5"}
	if len(resp.Tests) != len(wantIDs) {
		t.Fatalf("tests = %d, want %d", len(resp.Tests), len(wantIDs))
	}
	for i, want := range wantIDs {
		if resp.Tests[i].ID != want {
			t.Errorf("tests[%d].ID = %q, want %q", i, resp.Tests[i].ID, want)
		}
	}
	if resp.Pagination.NextPageToken != "" {
		t.Errorf("NextPageToken = %q, want empty", resp.Pagination.NextPageToken)
	}
	if resp.Pagination.TotalCount != 5 {
		t.Errorf("TotalCount = %d, want 5", resp.Pagination.TotalCount)
	}
}

func TestListRequirementsFollowsPages(t *testing.T) {
	var requests []url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		requests = append(requests, q)
		w.Header().Set("Content-Type", "application/json")
		switch q.Get("pagination.pageToken") {
		case "":
			_, _ = w.Write([]byte(`{"requirements":[{"id":"r1","title":"R1"},{"id":"r2","title":"R2"}],"pagination":{"nextPageToken":"1","totalCount":3}}`))
		case "1":
			_, _ = w.Write([]byte(`{"requirements":[{"id":"r3","title":"R3"}],"pagination":{"nextPageToken":"","totalCount":3}}`))
		default:
			t.Errorf("unexpected pagination.pageToken %q", q.Get("pagination.pageToken"))
		}
	}))
	defer srv.Close()

	client := newTestClient(srv.URL)
	resp, err := client.ListRequirements(context.Background(), "p1", "feature/x")
	if err != nil {
		t.Fatalf("ListRequirements: %v", err)
	}

	if len(requests) != 2 {
		t.Fatalf("requests = %d, want 2", len(requests))
	}
	for i, q := range requests {
		if q.Get("pagination.pageSize") != "200" {
			t.Errorf("request %d: pagination.pageSize = %q, want 200", i, q.Get("pagination.pageSize"))
		}
		if q.Get("branch") != "feature/x" {
			t.Errorf("request %d: branch = %q, want feature/x", i, q.Get("branch"))
		}
	}

	wantIDs := []string{"r1", "r2", "r3"}
	if len(resp.Requirements) != len(wantIDs) {
		t.Fatalf("requirements = %d, want %d", len(resp.Requirements), len(wantIDs))
	}
	for i, want := range wantIDs {
		if resp.Requirements[i].ID != want {
			t.Errorf("requirements[%d].ID = %q, want %q", i, resp.Requirements[i].ID, want)
		}
	}
	if resp.Pagination.NextPageToken != "" {
		t.Errorf("NextPageToken = %q, want empty", resp.Pagination.NextPageToken)
	}
	if resp.Pagination.TotalCount != 3 {
		t.Errorf("TotalCount = %d, want 3", resp.Pagination.TotalCount)
	}
}
