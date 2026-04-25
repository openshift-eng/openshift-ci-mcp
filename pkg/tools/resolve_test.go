package tools_test

import (
	"context"
	"testing"

	"github.com/openshift-eng/openshift-ci-mcp/pkg/tools"
)

type mockSippy struct {
	response []byte
	err      error
}

func (m *mockSippy) Get(ctx context.Context, path string, params map[string]string) ([]byte, error) {
	return m.response, m.err
}

func TestResolveRelease_ExplicitRelease(t *testing.T) {
	mock := &mockSippy{}
	release, err := tools.ResolveRelease(context.Background(), mock, "4.18")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if release != "4.18" {
		t.Errorf("expected '4.18', got %q", release)
	}
}

func TestResolveRelease_DefaultToCurrentDev(t *testing.T) {
	mock := &mockSippy{
		response: []byte(`{
			"releases": ["4.19", "4.18", "4.17"],
			"dates": {
				"4.19": {},
				"4.18": {"ga": "2025-06-15T00:00:00Z"},
				"4.17": {"ga": "2024-12-15T00:00:00Z"}
			}
		}`),
	}
	release, err := tools.ResolveRelease(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if release != "4.19" {
		t.Errorf("expected '4.19' (no GA date = in development), got %q", release)
	}
}

func TestResolveRelease_AllGA_FallbackToLatest(t *testing.T) {
	mock := &mockSippy{
		response: []byte(`{
			"releases": ["4.18", "4.17"],
			"dates": {
				"4.18": {"ga": "2025-06-15T00:00:00Z"},
				"4.17": {"ga": "2024-12-15T00:00:00Z"}
			}
		}`),
	}
	release, err := tools.ResolveRelease(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if release != "4.18" {
		t.Errorf("expected '4.18' (latest GA), got %q", release)
	}
}
