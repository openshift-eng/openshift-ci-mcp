package client_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/openshift-eng/openshift-ci-mcp/pkg/client"
)

func TestSearchCIClient_Search(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Query().Get("search") == "" {
			t.Error("expected search parameter")
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"results":{}}`))
	}))
	defer ts.Close()

	c := client.NewSearchCI(ts.URL, ts.Client())
	data, err := c.Search(context.Background(), "test failure", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("expected non-empty response")
	}
}

func TestSearchCIClient_Search_TypeDefault(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("type"); got != "all" {
			t.Errorf("expected default type=all, got %q", got)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer ts.Close()

	c := client.NewSearchCI(ts.URL, ts.Client())
	_, err := c.Search(context.Background(), "query", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSearchCIClient_Search_TypePassthrough(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("type"); got != "junit" {
			t.Errorf("expected type=junit to pass through, got %q", got)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer ts.Close()

	c := client.NewSearchCI(ts.URL, ts.Client())
	_, err := c.Search(context.Background(), "query", map[string]string{"type": "junit"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
