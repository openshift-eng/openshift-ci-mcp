package proxy

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/jeroche/openshift-ci-mcp/pkg/client"
	"github.com/jeroche/openshift-ci-mcp/pkg/tools"
)

func RegisterSearchCIProxy(s *server.MCPServer, search client.SearchCI) {
	s.AddTool(mcp.NewTool("search_ci_api",
		mcp.WithDescription("Low-level passthrough to Search.CI API (advanced use only). Prefer search_ci_logs for searching build logs and test failures."),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithString("query", mcp.Required(), mcp.Description("Search query string")),
	), SearchCIAPIHandler(search))
}

func SearchCIAPIHandler(search client.SearchCI) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		query, err := req.RequireString("query")
		if err != nil {
			return tools.InvalidParam("query", "required")
		}
		params := extractParams(req)
		data, err := search.Search(ctx, query, params)
		if err != nil {
			return tools.ToolError(err)
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}
