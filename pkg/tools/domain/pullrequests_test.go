package domain_test

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/openshift-eng/openshift-ci-mcp/pkg/tools/domain"
)

func TestGetPullRequestImpact(t *testing.T) {
	mock := newMockSippy(map[string][]byte{
		"/api/pull_requests/test_results": []byte(`[{"test_name":"test-1","result":"Failed"}]`),
	})
	handler := domain.GetPullRequestImpactHandler(mock, nil)
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"org": "openshift", "repo": "kubernetes", "pr_number": "12345"}
	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
}

func TestGetPullRequestImpact_Pagination(t *testing.T) {
	var capturedParams map[string]string
	mock := &capturingSippy{
		response: []byte(`[]`),
		onGet: func(path string, params map[string]string) {
			capturedParams = params
		},
	}
	handler := domain.GetPullRequestImpactHandler(mock, nil)
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"org":       "openshift",
		"repo":      "kubernetes",
		"pr_number": "12345",
		"limit":     float64(10),
		"page":      float64(2),
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
}

func TestGetPullRequests(t *testing.T) {
	mock := newMockSippy(map[string][]byte{
		"/api/pull_requests": []byte(`{"rows":[{"org":"openshift","repo":"kubernetes","number":12345}],"total_rows":1}`),
	})
	handler := domain.GetPullRequestsHandler(mock, nil)
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"release": "4.18", "org": "openshift", "repo": "kubernetes"}
	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
}
