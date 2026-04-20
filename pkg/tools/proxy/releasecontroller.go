package proxy

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/jeroche/openshift-ci-mcp/pkg/client"
	"github.com/jeroche/openshift-ci-mcp/pkg/tools"
)

func RegisterReleaseControllerProxy(s *server.MCPServer, rc client.ReleaseController) {
	s.AddTool(mcp.NewTool("release_controller_api",
		mcp.WithDescription("Raw passthrough to the Release Controller API. Returns unmodified upstream response."),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithString("arch", mcp.Description("Architecture (default: amd64)"), mcp.DefaultString("amd64")),
		mcp.WithString("path", mcp.Required(), mcp.Description("API path (e.g. '/api/v1/releasestream/4.18.0-0.nightly/tags')")),
	), ReleaseControllerAPIHandler(rc))
}

func ReleaseControllerAPIHandler(rc client.ReleaseController) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		path, err := req.RequireString("path")
		if err != nil {
			return tools.InvalidParam("path", "required")
		}
		arch := req.GetString("arch", "amd64")
		params := extractParams(req)
		data, err := rc.GetForArch(ctx, arch, path, params)
		if err != nil {
			return tools.ToolError(err)
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}
