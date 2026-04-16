package domain_test

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/jeroche/openshift-ci-mcp/pkg/tools/domain"
)

func TestGetVariants(t *testing.T) {
	mock := newMockSippy(map[string][]byte{
		"/api/job_variants": []byte(`{"variants":{"Architecture":["amd64","arm64"],"Topology":["ha","single"]}}`),
	})

	handler := domain.GetVariantsHandler(mock)
	result, err := handler(context.Background(), mcp.CallToolRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
}
