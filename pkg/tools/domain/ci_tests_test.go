package domain_test

import (
	"context"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/openshift-eng/openshift-ci-mcp/pkg/tools/domain"
)

func TestGetTestReport(t *testing.T) {
	mock := newMockSippy(map[string][]byte{
		"/api/tests": []byte(`[{"name":"test-1","current_pass_percentage":99.0}]`),
	})
	handler := domain.GetTestReportHandler(mock)
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"release": "4.18"}
	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
}

func TestGetTestDetails(t *testing.T) {
	mock := newMockSippy(map[string][]byte{
		"/api/tests/details": []byte(`{"name":"test-1"}`),
	})
	handler := domain.GetTestDetailsHandler(mock)
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"release": "4.18", "test_name": "[sig-network] pods should work"}
	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
}

func TestGetTestDetails_VariantFiltering(t *testing.T) {
	var capturedParams map[string]string
	mock := &capturingSippy{
		response: []byte(`{"name":"test-1"}`),
		onGet: func(path string, params map[string]string) {
			capturedParams = params
		},
	}
	handler := domain.GetTestDetailsHandler(mock)
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"release":   "4.18",
		"test_name": "[sig-network] test",
		"arch":      "arm64",
		"platform":  "aws",
	}
	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
	if !strings.Contains(capturedParams["filter"], "Architecture:arm64") {
		t.Error("expected filter to contain Architecture:arm64")
	}
	if !strings.Contains(capturedParams["filter"], "Platform:aws") {
		t.Error("expected filter to contain Platform:aws")
	}
}

func TestGetTestReport_ResponseTrimming(t *testing.T) {
	mockData := `[{"name":"test-1","id":42,"jira_component_id":99,"current_pass_percentage":95.0,"current_runs":100,"current_successes":95,"current_failures":3,"current_flakes":2,"current_failure_percentage":3.0,"net_improvement":5.0,"net_failure_improvement":2.0,"open_bugs":1}]`
	mock := newMockSippy(map[string][]byte{"/api/tests": []byte(mockData)})
	handler := domain.GetTestReportHandler(mock)

	t.Run("metrics excluded by default", func(t *testing.T) {
		req := mcp.CallToolRequest{}
		req.Params.Arguments = map[string]any{"release": "4.18"}
		result, err := handler(context.Background(), req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		text := result.Content[0].(mcp.TextContent).Text
		for _, dropped := range []string{"\"id\"", "jira_component_id"} {
			if strings.Contains(text, dropped) {
				t.Errorf("expected %s to be dropped", dropped)
			}
		}
		if !strings.Contains(text, "current_pass_percentage") {
			t.Error("expected current_pass_percentage to be kept")
		}
		if strings.Contains(text, `"metrics"`) {
			t.Error("expected metrics to be excluded by default")
		}
		if strings.Contains(text, "current_failure_percentage") {
			t.Error("expected current_failure_percentage to be excluded by default")
		}
	})

	t.Run("metrics included when requested", func(t *testing.T) {
		req := mcp.CallToolRequest{}
		req.Params.Arguments = map[string]any{"release": "4.18", "include_metrics": true}
		result, err := handler(context.Background(), req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		text := result.Content[0].(mcp.TextContent).Text
		if !strings.Contains(text, `"metrics"`) {
			t.Error("expected metrics sub-object when include_metrics=true")
		}
		if !strings.Contains(text, "current_failure_percentage") {
			t.Error("expected current_failure_percentage inside metrics")
		}
		if !strings.Contains(text, "net_failure_improvement") {
			t.Error("expected net_failure_improvement inside metrics")
		}
	})
}

func TestGetRecentTestFailures(t *testing.T) {
	mock := newMockSippy(map[string][]byte{
		"/api/tests/recent_failures": []byte(`[{"name":"test-1","current_pass_percentage":50.0,"current_runs":10,"current_successes":5,"current_failures":3,"current_flakes":2,"net_improvement":-5.0,"open_bugs":0}]`),
	})
	handler := domain.GetRecentTestFailuresHandler(mock)
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"release": "4.18"}
	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
}

func TestGetRecentTestFailures_PaginationAndFilters(t *testing.T) {
	var capturedParams map[string]string
	mock := &capturingSippy{
		response: []byte(`[{"name":"test-1","current_pass_percentage":50.0,"current_runs":10,"current_successes":5,"current_failures":3,"current_flakes":2,"net_improvement":-5.0,"open_bugs":0}]`),
		onGet: func(path string, params map[string]string) {
			capturedParams = params
		},
	}
	handler := domain.GetRecentTestFailuresHandler(mock)
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"release":   "4.18",
		"limit":     float64(10),
		"page":      float64(2),
		"test_name": "sig-network",
		"component": "Networking",
	}
	_, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedParams["perPage"] != "10" {
		t.Errorf("expected perPage=10, got %s", capturedParams["perPage"])
	}
	if capturedParams["page"] != "2" {
		t.Errorf("expected page=2, got %s", capturedParams["page"])
	}
	if !strings.Contains(capturedParams["filter"], "sig-network") {
		t.Error("expected filter to contain test_name filter")
	}
	if !strings.Contains(capturedParams["filter"], "Networking") {
		t.Error("expected filter to contain component filter")
	}
}
