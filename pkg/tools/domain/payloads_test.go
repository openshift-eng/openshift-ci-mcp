package domain_test

import (
	"context"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/openshift-eng/openshift-ci-mcp/pkg/tools/domain"
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
	handler := domain.GetPayloadDiffHandler(mock, nil)
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

func TestGetPayloadStatus_Truncation(t *testing.T) {
	rc := &mockReleaseController{responses: map[string][]byte{
		"amd64:/api/v1/releasestream/4.18.0-0.nightly/tags": []byte(`{"name":"4.18.0-0.nightly","tags":[{"name":"tag1"},{"name":"tag2"},{"name":"tag3"},{"name":"tag4"},{"name":"tag5"}]}`),
	}}
	handler := domain.GetPayloadStatusHandler(rc)
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"release": "4.18", "limit": float64(2)}
	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := result.Content[0].(mcp.TextContent).Text
	if strings.Contains(text, "tag3") {
		t.Errorf("expected tag3 to be truncated, got text: %s", text)
	}
	if !strings.Contains(text, "tag1") || !strings.Contains(text, "tag2") {
		t.Error("expected tag1 and tag2 to be preserved")
	}
}

func TestGetPayloadDiff_Pagination(t *testing.T) {
	var capturedParams map[string]string
	mock := &capturingSippy{
		response: []byte(`[{"name":"a"},{"name":"b"},{"name":"c"}]`),
		onGet: func(path string, params map[string]string) {
			capturedParams = params
		},
	}
	handler := domain.GetPayloadDiffHandler(mock, nil)
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"to_tag": "tag-1",
		"limit":  float64(2),
		"page":   float64(1),
	}
	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := capturedParams["perPage"]; ok {
		t.Error("expected perPage to NOT be sent upstream")
	}
	text := result.Content[0].(mcp.TextContent).Text
	if !strings.Contains(text, `"total_rows":3`) {
		t.Errorf("expected total_rows=3 in paginated response, got %s", text)
	}
	if !strings.Contains(text, `"page_size":2`) {
		t.Errorf("expected page_size=2 in response, got %s", text)
	}
}

func TestGetPayloadTestFailures(t *testing.T) {
	mock := newMockSippy(map[string][]byte{
		"/api/payloads/test_failures": []byte(`[{"test_name":"test-1","count":3}]`),
	})
	handler := domain.GetPayloadTestFailuresHandler(mock, nil)
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

func TestGetPayloadTestFailures_PaginationAndFilter(t *testing.T) {
	var capturedParams map[string]string
	mock := &capturingSippy{
		response: []byte(`[{"name":"e2e-install-test","failure_count":3},{"name":"e2e-install-other","failure_count":1}]`),
		onGet: func(path string, params map[string]string) {
			capturedParams = params
		},
	}
	handler := domain.GetPayloadTestFailuresHandler(mock, nil)
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"release":   "4.18",
		"limit":     float64(10),
		"page":      float64(1),
		"test_name": "e2e-install",
	}
	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := capturedParams["perPage"]; ok {
		t.Error("expected perPage to NOT be sent upstream")
	}
	if !strings.Contains(capturedParams["filter"], "e2e-install") {
		t.Error("expected filter to contain test_name")
	}
	text := result.Content[0].(mcp.TextContent).Text
	if !strings.Contains(text, `"total_rows":2`) {
		t.Errorf("expected total_rows=2 in paginated response, got %s", text)
	}
}
