package client_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jeroche/openshift-ci-mcp/pkg/client"
)

func TestReleaseControllerClient_Get(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/releasestream/4.18.0-0.nightly/tags" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"name":"4.18.0-0.nightly","tags":[{"name":"4.18.0-0.nightly-2025-01-01","phase":"Accepted"}]}`))
	}))
	defer ts.Close()

	c := client.NewReleaseController(ts.URL, ts.Client())
	data, err := c.Get(context.Background(), "/api/v1/releasestream/4.18.0-0.nightly/tags", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("expected non-empty response")
	}
}
