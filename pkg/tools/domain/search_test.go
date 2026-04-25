package domain_test

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/openshift-eng/openshift-ci-mcp/pkg/tools/domain"
)

type mockSearchCI struct {
	response []byte
}

func (m *mockSearchCI) Search(ctx context.Context, query string, params map[string]string) ([]byte, error) {
	return m.response, nil
}

func (m *mockSearchCI) Get(ctx context.Context, path string, params map[string]string) ([]byte, error) {
	return m.response, nil
}

func TestSearchCILogs(t *testing.T) {
	mock := &mockSearchCI{response: []byte(`{"results":{"test-failure":{"matches":[{"context":["job-1"]}]}}}`)}
	handler := domain.SearchCILogsHandler(mock)
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"query": "test-failure"}
	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
}
