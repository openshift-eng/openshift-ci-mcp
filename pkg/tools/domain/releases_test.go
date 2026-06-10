package domain_test

import (
	"context"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/openshift-eng/openshift-ci-mcp/pkg/tools/domain"
)

func TestGetReleases(t *testing.T) {
	mock := newMockSippy(map[string][]byte{
		"/api/releases": []byte(`{"releases":["4.19","4.18"],"dates":{"4.19":{},"4.18":{"ga":"2025-06-15T00:00:00Z"}}}`),
	})

	handler := domain.GetReleasesHandler(mock, nil)
	result, err := handler(context.Background(), mcp.CallToolRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
	if len(result.Content) == 0 {
		t.Fatal("expected content in result")
	}
}

func TestGetReleaseHealth(t *testing.T) {
	mock := newMockSippy(map[string][]byte{
		"/api/health":          []byte(`{"indicators":{},"last_updated":"2025-01-01T00:00:00Z"}`),
		"/api/releases/health": []byte(`[{"release_tag":"4.18.0","last_phase":"Accepted"}]`),
	})

	handler := domain.GetReleaseHealthHandler(mock, nil)
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"release": "4.18"}
	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
	text := result.Content[0].(mcp.TextContent).Text
	if !strings.Contains(text, `"health"`) || !strings.Contains(text, `"release_health"`) {
		t.Error("expected both health and release_health sections by default")
	}
}

func TestGetReleaseHealth_SectionFiltering(t *testing.T) {
	var calledPaths []string
	mock := &capturingSippy{
		response: []byte(`{}`),
		onGet: func(path string, params map[string]string) {
			calledPaths = append(calledPaths, path)
		},
	}

	handler := domain.GetReleaseHealthHandler(mock, nil)
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"release": "4.18", "sections": "health"}
	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}

	for _, p := range calledPaths {
		if p == "/api/releases/health" {
			t.Error("expected /api/releases/health to NOT be called when sections=health")
		}
	}
	text := result.Content[0].(mcp.TextContent).Text
	if !strings.Contains(text, `"health"`) {
		t.Error("expected health section in response")
	}
	if strings.Contains(text, `"release_health"`) {
		t.Error("expected release_health section to be omitted")
	}
}
