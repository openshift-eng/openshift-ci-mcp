package proxy

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/jeroche/openshift-ci-mcp/pkg/client"
	"github.com/jeroche/openshift-ci-mcp/pkg/tools"
)

func RegisterSippyProxy(s *server.MCPServer, sippy client.Sippy) {
	s.AddTool(mcp.NewTool("sippy_api",
		mcp.WithDescription("Raw passthrough to any Sippy API endpoint. Returns unmodified upstream response. See https://sippy.dptools.openshift.org/api for available endpoints."),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithString("path", mcp.Required(), mcp.Description("API path (e.g. '/api/jobs')")),
	), SippyAPIHandler(sippy))
}

func SippyAPIHandler(sippy client.Sippy) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		path, err := req.RequireString("path")
		if err != nil {
			return tools.InvalidParam("path", "required")
		}
		params := extractParams(req)
		data, err := sippy.Get(ctx, path, params)
		if err != nil {
			return tools.ToolError(err)
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}

func extractParams(req mcp.CallToolRequest) map[string]string {
	result := make(map[string]string)
	args := req.GetArguments()
	if paramsRaw, ok := args["params"].(map[string]any); ok {
		for k, v := range paramsRaw {
			if s, ok := v.(string); ok {
				result[k] = s
			}
		}
	}
	if filterStr, ok := args["filter"].(string); ok && filterStr != "" {
		result["filter"] = filterStr
	}
	return result
}
