package domain

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/jeroche/openshift-ci-mcp/pkg/client"
	"github.com/jeroche/openshift-ci-mcp/pkg/tools"
)

func RegisterVariantTools(s *server.MCPServer, sippy client.Sippy) {
	s.AddTool(
		mcp.NewTool("get_variants",
			mcp.WithDescription("List all variant dimensions and their possible values. Use this to discover valid values for arch, topology, platform, network, and other variant filters."),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithIdempotentHintAnnotation(true),
		),
		GetVariantsHandler(sippy),
	)
}

func GetVariantsHandler(sippy client.Sippy) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		data, err := sippy.Get(ctx, "/api/job_variants", nil)
		if err != nil {
			return tools.ToolError(err)
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}
