package domain_test

import (
	"context"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/jeroche/openshift-ci-mcp/pkg/tools/domain"
)

func TestGetJobReport(t *testing.T) {
	mock := newMockSippy(map[string][]byte{
		"/api/jobs": []byte(`{"rows":[{"name":"periodic-ci-e2e-aws","current_pass_percentage":95.5}],"total_rows":1}`),
	})

	handler := domain.GetJobReportHandler(mock)
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"release": "4.18",
		"arch":    "amd64",
	}
	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
}

func TestGetJobReport_VariantFiltering(t *testing.T) {
	var capturedParams map[string]string
	mock := &capturingSippy{
		response: []byte(`{"rows":[],"total_rows":0}`),
		onGet: func(path string, params map[string]string) {
			capturedParams = params
		},
	}

	handler := domain.GetJobReportHandler(mock)
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"release":  "4.18",
		"arch":     "arm64",
		"topology": "single",
	}
	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
	if !strings.Contains(capturedParams["filter"], "Architecture:arm64") {
		t.Error("expected filter to contain Architecture:arm64")
	}
	if !strings.Contains(capturedParams["filter"], "Topology:single") {
		t.Error("expected filter to contain Topology:single")
	}
}

func TestGetJobRuns(t *testing.T) {
	mock := newMockSippy(map[string][]byte{
		"/api/jobs/runs": []byte(`{"rows":[{"id":1,"name":"test-job"}],"total_rows":1}`),
	})

	handler := domain.GetJobRunsHandler(mock)
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"release":  "4.18",
		"job_name": "periodic-ci-e2e-aws",
	}
	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
}

func TestGetJobRunSummary(t *testing.T) {
	mock := newMockSippy(map[string][]byte{
		"/api/job/run/summary": []byte(`{"id":12345,"name":"e2e-aws","succeeded":true}`),
	})

	handler := domain.GetJobRunSummaryHandler(mock)
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"prow_job_run_id": "12345",
	}
	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
}
