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
		domains := []Domain{
			{Name: "example.com", Created: "2024-01-01", Updated: "2024-01-01", Tags: []string{"test"}},
		}
		_ = json.NewEncoder(w).Encode(domains)
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

func TestCreateDomain(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Expected POST request, got: %s", r.Method)
		}

		var req DomainRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.Name != "new.com" || req.IP != "1.1.1.1" {
			t.Errorf("Unexpected request body: %+v", req)
		}

		w.WriteHeader(http.StatusCreated)
		domain := Domain{Name: "new.com"}
		_ = json.NewEncoder(w).Encode(domain)
	}))
	defer server.Close()

	client := NewClient("test-key", server.URL, 0)
	domain, err := client.CreateDomain(context.Background(), "new.com", "1.1.1.1")

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
