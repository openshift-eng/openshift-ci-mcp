package domain

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/openshift-eng/openshift-ci-mcp/pkg/client"
	"github.com/openshift-eng/openshift-ci-mcp/pkg/tools"
)

func RegisterFieldTools(s *server.MCPServer) {
	s.AddTool(mcp.NewTool("get_tool_fields",
		mcp.WithDescription("Use to discover what field names a tool returns, for use with the 'fields' parameter."),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(false),
		mcp.WithString("tool_name", mcp.Required(), mcp.Description("Name of the mcp tool to get field names for")),
	), GetToolFieldsHandler())
}

func GetToolFieldsHandler() server.ToolHandlerFunc {
	registry := client.ToolFieldRegistry()
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		toolName, err := req.RequireString("tool_name")
		if err != nil {
			return tools.InvalidParam("tool_name", "required")
		}
		fields, ok := registry[toolName]
		if !ok {
			return tools.ToolError(fmt.Errorf("unknown tool %q", toolName))
		}
		resp := struct {
			Tool   string   `json:"tool"`
			Fields []string `json:"fields"`
		}{Tool: toolName, Fields: fields}
		data, _ := json.Marshal(resp)
		return mcp.NewToolResultText(string(data)), nil
	}
}
