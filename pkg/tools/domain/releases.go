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

func RegisterReleaseTools(s *server.MCPServer, sippy client.Sippy, cache *client.ResponseCache) {
	s.AddTool(
		mcp.NewTool("get_releases",
			mcp.WithDescription("Use to get OpenShift releases with availability and dev cycle dates"),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithOpenWorldHintAnnotation(true),
			mcp.WithString("fields", mcp.Description("Comma-separated list of field names to include in response (default: all)")),
		),
		GetReleasesHandler(sippy, cache),
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
		GetReleaseHealthHandler(sippy, cache),
	)
}

func GetReleasesHandler(sippy client.Sippy, cache *client.ResponseCache) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		data, err := cache.GetOrFetch("releases", func() ([]byte, error) {
			raw, err := sippy.Get(ctx, "/api/releases", nil)
			if err != nil {
				return nil, err
			}
			if trimmed, err := client.ReshapeJSON[client.ReleasesResponse](raw); err == nil {
				return trimmed, nil
			}
			return raw, nil
		})
		if err != nil {
			return tools.ToolError(err)
		}
		if filtered, err := client.FilterFields(data, req.GetString("fields", "")); err == nil {
			data = filtered
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}

func GetReleaseHealthHandler(sippy client.Sippy, cache *client.ResponseCache) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		release, err := tools.ResolveRelease(ctx, sippy, req.GetString("release", ""))
		if err != nil {
			return tools.ToolError(err)
		}

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

		cacheKey := fmt.Sprintf("release_health:%s:%v:%v", release, wantHealth, wantReleaseHealth)
		data, err := cache.GetOrFetch(cacheKey, func() ([]byte, error) {
			params := map[string]string{"release": release}
			parts := make([]string, 0, 2)
			if wantHealth {
				healthData, err := sippy.Get(ctx, "/api/health", params)
				if err != nil {
					return nil, err
				}
				if trimmed, err := client.ReshapeJSON[client.HealthResponse](healthData); err == nil {
					healthData = trimmed
				}
				parts = append(parts, fmt.Sprintf(`"health":%s`, string(healthData)))
			}
			if wantReleaseHealth {
				releaseHealthData, err := sippy.Get(ctx, "/api/releases/health", params)
				if err != nil {
					return nil, err
				}
				if trimmed, err := client.ReshapeJSON[[]client.ReleaseHealthRow](releaseHealthData); err == nil {
					releaseHealthData = trimmed
				}
				parts = append(parts, fmt.Sprintf(`"release_health":%s`, string(releaseHealthData)))
			}
			return []byte(fmt.Sprintf(`{%s}`, strings.Join(parts, ","))), nil
		})
		if err != nil {
			return tools.ToolError(err)
		}
		if filtered, err := client.FilterFields(data, req.GetString("fields", "")); err == nil {
			data = filtered
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}
