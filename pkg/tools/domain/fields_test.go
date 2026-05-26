package domain_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/openshift-eng/openshift-ci-mcp/pkg/tools/domain"
)

func TestGetToolFields_KnownTool(t *testing.T) {
	handler := domain.GetToolFieldsHandler()
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"tool_name": "get_job_report"}
	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
	text := result.Content[0].(mcp.TextContent).Text
	var resp struct {
		Tool   string   `json:"tool"`
		Fields []string `json:"fields"`
	}
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Tool != "get_job_report" {
		t.Errorf("expected tool=get_job_report, got %s", resp.Tool)
	}
	if len(resp.Fields) == 0 {
		t.Fatal("expected non-empty fields list")
	}
	found := false
	for _, f := range resp.Fields {
		if f == "name" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'name' in fields, got %v", resp.Fields)
	}
}

func TestGetToolFields_UnknownTool(t *testing.T) {
	handler := domain.GetToolFieldsHandler()
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"tool_name": "nonexistent_tool"}
	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected tool error for unknown tool")
	}
}

func TestGetToolFields_CombinedResponseTool(t *testing.T) {
	handler := domain.GetToolFieldsHandler()
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"tool_name": "get_release_health"}
	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := result.Content[0].(mcp.TextContent).Text
	if !strings.Contains(text, "health") || !strings.Contains(text, "release_health") {
		t.Errorf("expected health and release_health fields, got %s", text)
	}
}
