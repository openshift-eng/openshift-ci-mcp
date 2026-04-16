package domain_test

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/jeroche/openshift-ci-mcp/pkg/tools/domain"
)

func TestGetPullRequestImpact(t *testing.T) {
	mock := newMockSippy(map[string][]byte{
		"/api/pull_requests/test_results": []byte(`[{"test_name":"test-1","result":"Failed"}]`),
	})
	handler := domain.GetPullRequestImpactHandler(mock)
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

func TestGetPullRequests(t *testing.T) {
	mock := newMockSippy(map[string][]byte{
		"/api/pull_requests": []byte(`{"rows":[{"org":"openshift","repo":"kubernetes","number":12345}],"total_rows":1}`),
	})
	handler := domain.GetPullRequestsHandler(mock)
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
