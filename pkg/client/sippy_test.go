package client_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jeroche/openshift-ci-mcp/pkg/client"
)

func TestSippyClient_Get_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/releases" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("release") != "4.18" {
			t.Errorf("unexpected release param: %s", r.URL.Query().Get("release"))
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"releases":["4.18","4.17"]}`))
	}))
	defer ts.Close()

	c := client.NewSippy(ts.URL, ts.Client())
	params := map[string]string{"release": "4.18"}
	data, err := c.Get(context.Background(), "/api/releases", params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(data) != `{"releases":["4.18","4.17"]}` {
		t.Errorf("unexpected response: %s", string(data))
	}
}

func TestSippyClient_Get_HTTPError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error":"not found"}`))
	}))
	defer ts.Close()

	c := client.NewSippy(ts.URL, ts.Client())
	_, err := c.Get(context.Background(), "/api/missing", nil)
	if err == nil {
		t.Fatal("expected error")
	}
	var httpErr *client.HTTPError
	if !errors.As(err, &httpErr) {
		t.Fatalf("expected HTTPError, got %T", err)
	}
	if httpErr.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", httpErr.StatusCode)
	}
}

func TestSippyClient_Get_Timeout(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
	}))
	defer ts.Close()

	httpClient := ts.Client()
	httpClient.Timeout = 10 * time.Millisecond
	c := client.NewSippy(ts.URL, httpClient)
	_, err := c.Get(context.Background(), "/api/releases", nil)
	if err == nil {
		t.Fatal("expected timeout error")
	}
}
