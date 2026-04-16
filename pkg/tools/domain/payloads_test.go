package domain_test

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/jeroche/openshift-ci-mcp/pkg/tools/domain"
)

type mockReleaseController struct {
	responses map[string][]byte
}

func (m *mockReleaseController) Get(ctx context.Context, path string, params map[string]string) ([]byte, error) {
	if resp, ok := m.responses[path]; ok {
		return resp, nil
	}
	return []byte(`{}`), nil
}

func (m *mockReleaseController) GetForArch(ctx context.Context, arch, path string, params map[string]string) ([]byte, error) {
	key := arch + ":" + path
	if resp, ok := m.responses[key]; ok {
		return resp, nil
	}
	return m.Get(ctx, path, params)
}

func TestGetPayloadStatus(t *testing.T) {
	rc := &mockReleaseController{responses: map[string][]byte{
		"amd64:/api/v1/releasestream/4.18.0-0.nightly/tags": []byte(`{"name":"4.18.0-0.nightly","tags":[{"name":"4.18.0-0.nightly-2025-01-01","phase":"Accepted"}]}`),
	}}
	handler := domain.GetPayloadStatusHandler(rc)
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"release": "4.18", "stream": "nightly"}
	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
}

func TestGetPayloadDiff(t *testing.T) {
	mock := newMockSippy(map[string][]byte{
		"/api/payloads/diff": []byte(`[{"name":"etcd","url":"https://github.com/openshift/etcd/pull/123"}]`),
	})
	handler := domain.GetPayloadDiffHandler(mock)
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"to_tag": "4.18.0-0.nightly-2025-01-02-000000"}
	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
}

func TestGetPayloadTestFailures(t *testing.T) {
	mock := newMockSippy(map[string][]byte{
		"/api/payloads/test_failures": []byte(`[{"test_name":"test-1","count":3}]`),
	})
	handler := domain.GetPayloadTestFailuresHandler(mock)
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
