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

type capturingSearchCI struct {
	response []byte
	onSearch func(query string, params map[string]string)
}

func (m *capturingSearchCI) Search(ctx context.Context, query string, params map[string]string) ([]byte, error) {
	if m.onSearch != nil {
		m.onSearch(query, params)
	}
	return m.response, nil
}

func (m *capturingSearchCI) Get(ctx context.Context, path string, params map[string]string) ([]byte, error) {
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

func TestSearchCILogs_LimitParam(t *testing.T) {
	var capturedParams map[string]string
	mock := &capturingSearchCI{
		response: []byte(`{}`),
		onSearch: func(query string, params map[string]string) {
			capturedParams = params
		},
	}
	handler := domain.SearchCILogsHandler(mock)
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"query": "test-failure",
		"limit": float64(50),
	}
	_, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedParams["maxResults"] != "50" {
		t.Errorf("expected maxResults=50, got %s", capturedParams["maxResults"])
	}
}
