package domain

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/jeroche/openshift-ci-mcp/pkg/client"
	"github.com/jeroche/openshift-ci-mcp/pkg/tools"
)

func RegisterSearchTools(s *server.MCPServer, search client.SearchCI) {
	s.AddTool(mcp.NewTool("search_ci_logs",
		mcp.WithDescription("Search build logs and JUnit failures across OpenShift CI. Searches recent test results, job logs, and associated Jira issues."),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithString("query", mcp.Required(), mcp.Description("Search string — test name, error message, or job name pattern")),
		mcp.WithString("max_age", mcp.Description("Max age of results (e.g. '7d', '24h'). Default: server default.")),
		mcp.WithString("type", mcp.Description("Search type: 'all', 'bug', 'junit'. Default: 'all'.")),
	), SearchCILogsHandler(search))
}

func SearchCILogsHandler(search client.SearchCI) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		query, err := req.RequireString("query")
		if err != nil {
			return tools.InvalidParam("query", "required")
		}
		params := map[string]string{}
		if maxAge := req.GetString("max_age", ""); maxAge != "" {
			params["maxAge"] = maxAge
		}
		if searchType := req.GetString("type", ""); searchType != "" {
			params["type"] = searchType
		}
		data, err := search.Search(ctx, query, params)
		if err != nil {
			return tools.ToolError(err)
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}
