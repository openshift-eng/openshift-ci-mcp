package domain_test

import (
	"context"
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

func TestGetRecentTestFailures(t *testing.T) {
	mock := newMockSippy(map[string][]byte{
		"/api/tests/recent_failures": []byte(`[{"name":"test-1","current_pass_percentage":50.0}]`),
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
