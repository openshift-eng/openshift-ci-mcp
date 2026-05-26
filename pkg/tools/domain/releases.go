package domain

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/openshift-eng/openshift-ci-mcp/pkg/client"
	"github.com/openshift-eng/openshift-ci-mcp/pkg/tools"
)

func RegisterReleaseTools(s *server.MCPServer, sippy client.Sippy) {
	s.AddTool(
		mcp.NewTool("get_releases",
			mcp.WithDescription("Use to get OpenShift releases with availability and dev cycle dates"),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithOpenWorldHintAnnotation(true),
			mcp.WithString("fields", mcp.Description("Comma-separated list of field names to include in response (default: all)")),
		),
		GetReleasesHandler(sippy),
	)

	s.AddTool(
		mcp.NewTool("get_release_health",
			mcp.WithDescription("Use to get health data for a specific release such as success rates, variant summary, and payload acceptance."),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithOpenWorldHintAnnotation(true),
			mcp.WithString("release",
				mcp.Description("Release version (e.g. '4.18')"),
			),
			mcp.WithString("sections",
				mcp.Description("Comma-separated sections to include: 'health', 'release_health'. Default: both."),
			),
			mcp.WithString("fields", mcp.Description("Comma-separated list of field names to include in response (default: all)")),
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
		if trimmed, err := client.ReshapeJSON[client.ReleasesResponse](data); err == nil {
			data = trimmed
		}
		if filtered, err := client.FilterFields(data, req.GetString("fields", "")); err == nil {
			data = filtered
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

		wantHealth := true
		wantReleaseHealth := true
		if sections := req.GetString("sections", ""); sections != "" {
			wantHealth = false
			wantReleaseHealth = false
			for _, s := range strings.Split(sections, ",") {
				switch strings.TrimSpace(s) {
				case "health":
					wantHealth = true
				case "release_health":
					wantReleaseHealth = true
				}
			}
		}

		parts := make([]string, 0, 2)
		if wantHealth {
			healthData, err := sippy.Get(ctx, "/api/health", params)
			if err != nil {
				return tools.ToolError(err)
			}
			if trimmed, err := client.ReshapeJSON[client.HealthResponse](healthData); err == nil {
				healthData = trimmed
			}
			parts = append(parts, fmt.Sprintf(`"health":%s`, string(healthData)))
		}
		if wantReleaseHealth {
			releaseHealthData, err := sippy.Get(ctx, "/api/releases/health", params)
			if err != nil {
				return tools.ToolError(err)
			}
			if trimmed, err := client.ReshapeJSON[[]client.ReleaseHealthRow](releaseHealthData); err == nil {
				releaseHealthData = trimmed
			}
			parts = append(parts, fmt.Sprintf(`"release_health":%s`, string(releaseHealthData)))
		}

		data := []byte(fmt.Sprintf(`{%s}`, strings.Join(parts, ",")))
		if filtered, err := client.FilterFields(data, req.GetString("fields", "")); err == nil {
			data = filtered
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}
