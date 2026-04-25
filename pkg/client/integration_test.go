//go:build integration

package client_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/openshift-eng/openshift-ci-mcp/pkg/client"
)

const integrationRelease = "4.18"

func integrationHTTPClient() *http.Client {
	return &http.Client{Timeout: 30 * time.Second}
}

func TestIntegration_Sippy_GetReleases(t *testing.T) {
	c := client.NewSippy("https://sippy.dptools.openshift.org", integrationHTTPClient())
	data, err := c.Get(context.Background(), "/api/releases", nil)
	if err != nil {
		t.Fatalf("failed to get releases: %v", err)
	}
	var resp map[string]any
	if err := json.Unmarshal(data, &resp); err != nil {
		t.Fatalf("invalid JSON response: %v", err)
	}
	releases, ok := resp["releases"].([]any)
	if !ok || len(releases) == 0 {
		t.Fatal("expected non-empty releases list")
	}
	t.Logf("found %d releases", len(releases))
}

func TestIntegration_Sippy_GetHealth(t *testing.T) {
	c := client.NewSippy("https://sippy.dptools.openshift.org", integrationHTTPClient())
	data, err := c.Get(context.Background(), "/api/health", map[string]string{"release": integrationRelease})
	if err != nil {
		t.Fatalf("failed to get health: %v", err)
	}
	var resp map[string]any
	if err := json.Unmarshal(data, &resp); err != nil {
		t.Fatalf("invalid JSON response: %v", err)
	}
	if _, ok := resp["indicators"]; !ok {
		t.Error("expected indicators in health response")
	}
}

func TestIntegration_Sippy_GetJobs(t *testing.T) {
	c := client.NewSippy("https://sippy.dptools.openshift.org", integrationHTTPClient())
	data, err := c.Get(context.Background(), "/api/jobs", map[string]string{"release": integrationRelease, "perPage": "5"})
	if err != nil {
		t.Fatalf("failed to get jobs: %v", err)
	}
	var rows []any
	if err := json.Unmarshal(data, &rows); err != nil {
		t.Fatalf("invalid JSON response: %v", err)
	}
	if len(rows) == 0 {
		t.Fatal("expected non-empty jobs list")
	}
	t.Logf("found %d jobs (limit 5)", len(rows))
}

func TestIntegration_Sippy_GetJobVariants(t *testing.T) {
	c := client.NewSippy("https://sippy.dptools.openshift.org", integrationHTTPClient())
	data, err := c.Get(context.Background(), "/api/job_variants", nil)
	if err != nil {
		t.Fatalf("failed to get job variants: %v", err)
	}
	var resp map[string]any
	if err := json.Unmarshal(data, &resp); err != nil {
		t.Fatalf("invalid JSON response: %v", err)
	}
	variants, ok := resp["variants"].(map[string]any)
	if !ok {
		t.Fatal("expected variants map in response")
	}
	t.Logf("found %d variant dimensions", len(variants))
}

func TestIntegration_ReleaseController_GetTags(t *testing.T) {
	c := client.NewReleaseController("https://amd64.ocp.releases.ci.openshift.org", integrationHTTPClient())
	data, err := c.Get(context.Background(), "/api/v1/releasestream/4.18.0-0.nightly/tags", nil)
	if err != nil {
		t.Fatalf("failed to get tags: %v", err)
	}
	var resp map[string]any
	if err := json.Unmarshal(data, &resp); err != nil {
		t.Fatalf("invalid JSON response: %v", err)
	}
	tags, ok := resp["tags"].([]any)
	if !ok {
		t.Fatal("expected tags array in response")
	}
	t.Logf("found %d payload tags", len(tags))
}
