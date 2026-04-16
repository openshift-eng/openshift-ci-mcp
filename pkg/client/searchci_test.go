package client_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jeroche/openshift-ci-mcp/pkg/client"
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
