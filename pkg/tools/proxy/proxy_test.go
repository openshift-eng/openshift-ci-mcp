package proxy_test

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/jeroche/openshift-ci-mcp/pkg/tools/proxy"
)

type mockClient struct {
	response []byte
}

func (m *mockClient) Get(ctx context.Context, path string, params map[string]string) ([]byte, error) {
	return m.response, nil
}
func (m *mockClient) GetForArch(ctx context.Context, arch, path string, params map[string]string) ([]byte, error) {
	return m.response, nil
}
func (m *mockClient) Search(ctx context.Context, query string, params map[string]string) ([]byte, error) {
	return m.response, nil
}

func TestSippyProxy(t *testing.T) {
	mock := &mockClient{response: []byte(`{"data":"raw"}`)}
	handler := proxy.SippyAPIHandler(mock)
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"path": "/api/health"}
	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
}

func TestReleaseControllerProxy(t *testing.T) {
	mock := &mockClient{response: []byte(`{"data":"raw"}`)}
	handler := proxy.ReleaseControllerAPIHandler(mock)
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"arch": "amd64", "path": "/api/v1/releasestream/4.18.0-0.nightly/tags"}
	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
}

func TestSearchCIProxy(t *testing.T) {
	mock := &mockClient{response: []byte(`{"results":{}}`)}
	handler := proxy.SearchCIAPIHandler(mock)
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"query": "test failure"}
	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
}
