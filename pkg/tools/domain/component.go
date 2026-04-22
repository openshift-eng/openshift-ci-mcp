package domain

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/jeroche/openshift-ci-mcp/pkg/client"
	"github.com/jeroche/openshift-ci-mcp/pkg/tools"
)

func RegisterComponentTools(s *server.MCPServer, sippy client.Sippy) {
	s.AddTool(mcp.NewTool("get_component_readiness",
		mcp.WithDescription("Component readiness report — the binding release gate. Shows statistical analysis comparing current release behavior against the previous stable release. This endpoint can be slow (30+ seconds). If view is omitted, the server auto-discovers the first available view, which adds an extra API call."),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithString("release", mcp.Description("Release version. Defaults to current dev release.")),
		mcp.WithString("view", mcp.Description("Predefined view name (default: server default). Use get_variants or check Sippy for available views.")),
	), GetComponentReadinessHandler(sippy))

	s.AddTool(mcp.NewTool("get_regressions",
		mcp.WithDescription("Active regressions from Component Readiness — tests that are performing significantly worse than the previous release"),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithString("release", mcp.Description("Release version. Defaults to current dev release.")),
		mcp.WithString("view", mcp.Description("Component Readiness view name")),
		mcp.WithString("component", mcp.Description("Filter by component name")),
	), GetRegressionsHandler(sippy))

	s.AddTool(mcp.NewTool("get_regression_detail",
		mcp.WithDescription("Details of a specific regression including linked triages and Jira bugs"),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithString("regression_id", mcp.Required(), mcp.Description("Regression ID")),
	), GetRegressionDetailHandler(sippy))
}

func GetComponentReadinessHandler(sippy client.Sippy) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		release, err := tools.ResolveRelease(ctx, sippy, req.GetString("release", ""))
		if err != nil {
			return tools.ToolError(err)
		}
		view := req.GetString("view", "")
		if view == "" {
			viewsData, err := sippy.Get(ctx, "/api/component_readiness/views", map[string]string{"release": release})
			if err == nil {
				var views []struct{ Name string `json:"name"` }
				if json.Unmarshal(viewsData, &views) == nil && len(views) > 0 {
					view = views[0].Name
				}
			}
		}
		params := map[string]string{"release": release}
		if view != "" {
			params["view"] = view
		}
		data, err := sippy.Get(ctx, "/api/component_readiness", params)
		if err != nil {
			return tools.ToolError(err)
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}

func GetRegressionsHandler(sippy client.Sippy) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		release, err := tools.ResolveRelease(ctx, sippy, req.GetString("release", ""))
		if err != nil {
			return tools.ToolError(err)
		}
		params := map[string]string{"release": release}
		if view := req.GetString("view", ""); view != "" {
			params["view"] = view
		}
		if component := req.GetString("component", ""); component != "" {
			params["component"] = component
		}
		data, err := sippy.Get(ctx, "/api/component_readiness/regressions", params)
		if err != nil {
			return tools.ToolError(err)
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}

func GetRegressionDetailHandler(sippy client.Sippy) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id, err := req.RequireString("regression_id")
		if err != nil {
			return tools.InvalidParam("regression_id", "required")
		}
		regressionData, err := sippy.Get(ctx, fmt.Sprintf("/api/component_readiness/regressions/%s", id), nil)
		if err != nil {
			return tools.ToolError(err)
		}
		matchesData, err := sippy.Get(ctx, fmt.Sprintf("/api/component_readiness/regressions/%s/matches", id), nil)
		if err != nil {
			return tools.ToolError(err)
		}
		combined := fmt.Sprintf(`{"regression":%s,"matching_triages":%s}`, string(regressionData), string(matchesData))
		return mcp.NewToolResultText(combined), nil
	}
}
