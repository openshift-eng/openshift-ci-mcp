package domain_test

import (
	"context"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/openshift-eng/openshift-ci-mcp/pkg/tools/domain"
)

func TestGetComponentReadiness(t *testing.T) {
	mock := newMockSippy(map[string][]byte{
		"/api/component_readiness":       []byte(`{"rows":[{"component":"etcd"}]}`),
		"/api/component_readiness/views": []byte(`[{"name":"main","params":{}}]`),
	})
	handler := domain.GetComponentReadinessHandler(mock, nil)
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

func TestGetRegressions(t *testing.T) {
	mock := newMockSippy(map[string][]byte{
		"/api/component_readiness/regressions": []byte(`[{"id":1,"test_name":"test-1","component":"etcd"}]`),
	})
	handler := domain.GetRegressionsHandler(mock, nil)
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

func TestGetRegressions_Pagination(t *testing.T) {
	var capturedParams map[string]string
	mock := &capturingSippy{
		response: []byte(`[{"id":1},{"id":2},{"id":3},{"id":4},{"id":5}]`),
		onGet: func(path string, params map[string]string) {
			capturedParams = params
		},
	}
	handler := domain.GetRegressionsHandler(mock, nil)
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"release": "4.18",
		"limit":   float64(2),
		"page":    float64(2),
	}
	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := capturedParams["perPage"]; ok {
		t.Error("expected perPage to NOT be sent upstream")
	}
	text := result.Content[0].(mcp.TextContent).Text
	if !strings.Contains(text, `"total_rows":5`) {
		t.Errorf("expected total_rows=5 in paginated response, got %s", text)
	}
	if !strings.Contains(text, `"page":2`) {
		t.Errorf("expected page=2 in response, got %s", text)
	}
}

func TestGetRegressionDetail(t *testing.T) {
	mock := newMockSippy(map[string][]byte{
		"/api/component_readiness/regressions/42":         []byte(`{"id":42,"test_name":"test-1"}`),
		"/api/component_readiness/regressions/42/matches": []byte(`[{"id":1,"url":"https://issues.redhat.com/browse/OCPBUGS-123"}]`),
	})
	handler := domain.GetRegressionDetailHandler(mock, nil)
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"regression_id": "42"}
	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
}
