package domain

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/jeroche/openshift-ci-mcp/pkg/client"
	"github.com/jeroche/openshift-ci-mcp/pkg/tools"
)

func RegisterReleaseTools(s *server.MCPServer, sippy client.Sippy) {
	s.AddTool(
		mcp.NewTool("get_releases",
			mcp.WithDescription("List available OpenShift releases with GA dates and development start dates"),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithIdempotentHintAnnotation(true),
		),
		GetReleasesHandler(sippy),
	)

	s.AddTool(
		mcp.NewTool("get_release_health",
			mcp.WithDescription("Overall health of a release — install/upgrade/infrastructure success rates, variant summary, and payload acceptance statistics"),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithString("release",
				mcp.Description("Release version (e.g. '4.18'). Defaults to current development release if omitted."),
			),
		),
		GetReleaseHealthHandler(sippy),
	)
}

func GetReleasesHandler(sippy client.Sippy) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		data, err := sippy.Get(ctx, "/api/releases", nil)
		if err != nil {
			return tools.ToolError(err)
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}

func GetReleaseHealthHandler(sippy client.Sippy) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		release, err := tools.ResolveRelease(ctx, sippy, req.GetString("release", ""))
		if err != nil {
			return tools.ToolError(err)
		}

		params := map[string]string{"release": release}

		healthData, err := sippy.Get(ctx, "/api/health", params)
		if err != nil {
			return tools.ToolError(err)
		}

		releaseHealthData, err := sippy.Get(ctx, "/api/releases/health", params)
		if err != nil {
			return tools.ToolError(err)
		}

		combined := fmt.Sprintf(`{"health":%s,"release_health":%s}`, string(healthData), string(releaseHealthData))
		return mcp.NewToolResultText(combined), nil
	}
}
