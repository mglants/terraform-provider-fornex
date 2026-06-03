package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestListDomains(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/dns/domain/" {
			t.Errorf("Expected to request '/dns/domain/', got: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Api-Key test-key" {
			t.Errorf("Expected Authorization header 'Api-Key test-key', got: %s", r.Header.Get("Authorization"))
		}

		w.WriteHeader(http.StatusOK)
		page := paginatedDomains{
			Results: []Domain{
				{Name: "example.com", Created: "2024-01-01", Updated: "2024-01-01", Tags: []string{"test"}},
			},
		}
		_ = json.NewEncoder(w).Encode(page)
	}))
	defer server.Close()

	client := NewClient("test-key", server.URL, 0)
	domains, err := client.ListDomains(context.Background())

	if err != nil {
		t.Fatalf("Expected no error, got: %s", err)
	}

	if len(domains) != 1 {
		t.Fatalf("Expected 1 domain, got: %d", len(domains))
	}

	if domains[0].Name != "example.com" {
		t.Errorf("Expected domain name 'example.com', got: %s", domains[0].Name)
	}
}

func TestGetDomainPaginated(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		switch r.URL.Query().Get("page") {
		case "", "1":
			// Page 1: target domain is not here, but `next` points at page 2.
			next := "http://" + r.Host + r.URL.Path + "?page=2&q=" + r.URL.Query().Get("q")
			_ = json.NewEncoder(w).Encode(paginatedDomains{
				Next: next,
				Results: []Domain{
					{Name: "other.com"},
				},
			})
		case "2":
			_ = json.NewEncoder(w).Encode(paginatedDomains{
				Results: []Domain{
					{Name: "wanted.com", Tags: []string{"test"}},
				},
			})
		}
	}))
	defer server.Close()

	client := NewClient("test-key", server.URL, 0)
	domain, err := client.GetDomain(context.Background(), "wanted.com")
	if err != nil {
		t.Fatalf("Expected no error, got: %s", err)
	}
	if domain.Name != "wanted.com" {
		t.Errorf("Expected wanted.com, got: %s", domain.Name)
	}
}

func TestGetDomainCached(t *testing.T) {
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusOK)
		// Each q-walk returns the same two domains for test simplicity.
		// Production Fornex returns *different* sets depending on q.
		_ = json.NewEncoder(w).Encode(paginatedDomains{
			Results: []Domain{
				{Name: "cached.com"},
				{Name: "sibling.com"},
			},
		})
	}))
	defer server.Close()

	client := NewClient("test-key", server.URL, 0)

	for i := 0; i < 3; i++ {
		domain, err := client.GetDomain(context.Background(), "cached.com")
		if err != nil {
			t.Fatalf("GetDomain(cached.com) #%d: %s", i, err)
		}
		if domain.Name != "cached.com" {
			t.Errorf("Expected cached.com, got: %s", domain.Name)
		}
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Errorf("Repeated lookups of same name should share 1 walk, got: %d", got)
	}

	// `sibling.com` was returned as part of the cached.com walk, so it's
	// already cached and should NOT trigger a fresh HTTP call.
	if _, err := client.GetDomain(context.Background(), "sibling.com"); err != nil {
		t.Fatalf("GetDomain(sibling.com): %s", err)
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Errorf("Cached sibling lookup should not trigger a new walk, got: %d", got)
	}

	// A genuinely missing name forces a fresh walk (since the cache cannot
	// rule out absence — Fornex's `q=` is fuzzy and a previous walk's
	// non-results do NOT prove non-existence).
	if _, err := client.GetDomain(context.Background(), "missing.com"); err == nil {
		t.Fatal("expected not-found error for missing.com")
	}
	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Errorf("Cache miss for missing.com should force a new walk (total=2), got: %d", got)
	}
}

func TestGetEntryCached(t *testing.T) {
	var listCalls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "GET" && strings.HasSuffix(r.URL.Path, "/entry_set/"):
			atomic.AddInt32(&listCalls, 1)
			_ = json.NewEncoder(w).Encode([]Entry{
				{ID: 1, Host: "@", Type: "A", Value: "1.1.1.1"},
				{ID: 2, Host: "www", Type: "A", Value: "2.2.2.2"},
			})
		case r.Method == "DELETE":
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("Unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	client := NewClient("test-key", server.URL, 0)

	for i := 0; i < 3; i++ {
		entry, err := client.GetEntry(context.Background(), "example.com", 1)
		if err != nil {
			t.Fatalf("GetEntry #%d: %s", i, err)
		}
		if entry.ID != 1 {
			t.Errorf("Expected entry 1, got: %d", entry.ID)
		}
	}
	if got := atomic.LoadInt32(&listCalls); got != 1 {
		t.Errorf("Expected exactly 1 ListEntries call across 3 GetEntry calls, got: %d", got)
	}

	if err := client.DeleteEntry(context.Background(), "example.com", 2); err != nil {
		t.Fatalf("DeleteEntry: %s", err)
	}
	if _, err := client.GetEntry(context.Background(), "example.com", 1); err != nil {
		t.Fatalf("GetEntry after DeleteEntry: %s", err)
	}
	if got := atomic.LoadInt32(&listCalls); got != 2 {
		t.Errorf("Expected ListEntries to be re-fetched after DeleteEntry (total=2), got: %d", got)
	}
}

func TestCreateDomain(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Expected POST request, got: %s", r.Method)
		}

		var req DomainRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.Name != "new.com" || req.IP != createDomainStubIP {
			t.Errorf("Unexpected request body: %+v", req)
		}

		w.WriteHeader(http.StatusCreated)
		domain := Domain{Name: "new.com"}
		_ = json.NewEncoder(w).Encode(domain)
	}))
	defer server.Close()

	client := NewClient("test-key", server.URL, 0)
	domain, err := client.CreateDomain(context.Background(), "new.com")

	if err != nil {
		t.Fatalf("Expected no error, got: %s", err)
	}

	if domain.Name != "new.com" {
		t.Errorf("Expected domain name 'new.com', got: %s", domain.Name)
	}
}

func TestDeleteDomain(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "DELETE" {
			t.Errorf("Expected DELETE request, got: %s", r.Method)
		}
		if r.URL.Path != "/dns/domain/example.com/" {
			t.Errorf("Expected path '/dns/domain/example.com/', got: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := NewClient("test-key", server.URL, 0)
	err := client.DeleteDomain(context.Background(), "example.com")

	if err != nil {
		t.Fatalf("Expected no error, got: %s", err)
	}
}

func TestErrorHandling(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("invalid request"))
	}))
	defer server.Close()

	client := NewClient("test-key", server.URL, 0)
	_, err := client.ListDomains(context.Background())

	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	if !strings.Contains(err.Error(), "status: 400") {
		t.Errorf("Expected error to contain status 400, got: %s", err)
	}
}

func TestNoRetryOnPerRequestTimeout(t *testing.T) {
	var count int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&count, 1)
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient("test-key", server.URL, 50*time.Millisecond)
	_, err := client.ListDomains(context.Background())

	if err == nil {
		t.Fatal("Expected timeout error, got nil")
	}

	if got := atomic.LoadInt32(&count); got != 1 {
		t.Errorf("Expected exactly 1 request (no retry on timeout), got: %d", got)
	}
}

func TestRetryOnServerError(t *testing.T) {
	var count int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&count, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewClient("test-key", server.URL, time.Second)
	client.HTTPClient.RetryWaitMin = time.Millisecond
	client.HTTPClient.RetryWaitMax = 5 * time.Millisecond

	_, err := client.ListDomains(context.Background())

	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	if got := atomic.LoadInt32(&count); got != 5 {
		t.Errorf("Expected 5 requests (1 + RetryMax=4 retries), got: %d", got)
	}
}
