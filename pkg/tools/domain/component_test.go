package domain_test

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/jeroche/openshift-ci-mcp/pkg/tools/domain"
)

func TestGetComponentReadiness(t *testing.T) {
	mock := newMockSippy(map[string][]byte{
		"/api/component_readiness":       []byte(`{"rows":[{"component":"etcd"}]}`),
		"/api/component_readiness/views": []byte(`[{"name":"main","params":{}}]`),
	})
	handler := domain.GetComponentReadinessHandler(mock)
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
	handler := domain.GetRegressionsHandler(mock)
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

func TestGetRegressionDetail(t *testing.T) {
	mock := newMockSippy(map[string][]byte{
		"/api/component_readiness/regressions/42":         []byte(`{"id":42,"test_name":"test-1"}`),
		"/api/component_readiness/regressions/42/matches": []byte(`[{"id":1,"url":"https://issues.redhat.com/browse/OCPBUGS-123"}]`),
	})
	handler := domain.GetRegressionDetailHandler(mock)
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
