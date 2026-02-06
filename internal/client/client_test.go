package client

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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
		json.NewEncoder(w).Encode(domains)
	}))
	defer server.Close()

	client := NewClient("test-key", server.URL)
	domains, err := client.ListDomains()

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
		json.NewDecoder(r.Body).Decode(&req)
		if req.Name != "new.com" || req.IP != "1.1.1.1" {
			t.Errorf("Unexpected request body: %+v", req)
		}

		w.WriteHeader(http.StatusCreated)
		domain := Domain{Name: "new.com"}
		json.NewEncoder(w).Encode(domain)
	}))
	defer server.Close()

	client := NewClient("test-key", server.URL)
	domain, err := client.CreateDomain("new.com", "1.1.1.1")

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

	client := NewClient("test-key", server.URL)
	err := client.DeleteDomain("example.com")

	if err != nil {
		t.Fatalf("Expected no error, got: %s", err)
	}
}

func TestErrorHandling(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("invalid request"))
	}))
	defer server.Close()

	client := NewClient("test-key", server.URL)
	_, err := client.ListDomains()

	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	if !strings.Contains(err.Error(), "status: 400") {
		t.Errorf("Expected error to contain status 400, got: %s", err)
	}
}
