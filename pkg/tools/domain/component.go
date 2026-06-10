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

func RegisterComponentTools(s *server.MCPServer, sippy client.Sippy, cache *client.ResponseCache) {
	s.AddTool(mcp.NewTool("get_component_readiness",
		mcp.WithDescription("Use to get a report on component readiness for the current dev cycle. Can be slow (30+ seconds). Passing a view name avoids an extra API call to discover views."),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(true),
		mcp.WithString("release", mcp.Description("Release version. Default: current dev release.")),
		mcp.WithString("view", mcp.Description("Predefined view name. Default: auto-discovers first available view.")),
		mcp.WithString("fields", mcp.Description("Comma-separated list of field names to include in response (default: all)")),
	), GetComponentReadinessHandler(sippy, cache))

	s.AddTool(mcp.NewTool("get_regressions",
		mcp.WithDescription("Use to get tests performing significantly worse than the previous release"),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(true),
		mcp.WithString("release", mcp.Description("Release version. Default: current dev release")),
		mcp.WithString("view", mcp.Description("Component Readiness view name")),
		mcp.WithString("component", mcp.Description("Filter by component name")),
		mcp.WithNumber("limit", mcp.Description("Max results per page (default 25)"), mcp.DefaultNumber(25)),
		mcp.WithNumber("page", mcp.Description("Page number (default 1)"), mcp.DefaultNumber(1)),
		mcp.WithString("fields", mcp.Description("Comma-separated list of field names to include in response (default: all)")),
	), GetRegressionsHandler(sippy, cache))

	s.AddTool(mcp.NewTool("get_regression_detail",
		mcp.WithDescription("Use when you need details about a regression with triages and Jiras."),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(true),
		mcp.WithString("regression_id", mcp.Required(), mcp.Description("Regression ID")),
		mcp.WithString("fields", mcp.Description("Comma-separated list of field names to include in response (default: all)")),
	), GetRegressionDetailHandler(sippy, cache))
}

func GetComponentReadinessHandler(sippy client.Sippy, cache *client.ResponseCache) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		release, err := tools.ResolveRelease(ctx, sippy, req.GetString("release", ""))
		if err != nil {
			return tools.ToolError(err)
		}
		view := req.GetString("view", "")
		if view == "" {
			viewsData, err := sippy.Get(ctx, "/api/component_readiness/views", map[string]string{"release": release})
			if err == nil {
				var views []client.ComponentReadinessView
				if json.Unmarshal(viewsData, &views) == nil && len(views) > 0 {
					view = views[0].Name
				}
			}
		}
		params := map[string]string{"release": release}
		if view != "" {
			params["view"] = view
		}
		cacheKey := fmt.Sprintf("component_readiness:%s:%s", release, view)
		data, err := cache.GetOrFetch(cacheKey, func() ([]byte, error) {
			raw, err := sippy.Get(ctx, "/api/component_readiness", params)
			if err != nil {
				return nil, err
			}
			if trimmed, err := client.ReshapeJSON[client.ComponentReadinessResponse](raw); err == nil {
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

func GetRegressionsHandler(sippy client.Sippy, cache *client.ResponseCache) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		release, err := tools.ResolveRelease(ctx, sippy, req.GetString("release", ""))
		if err != nil {
			return tools.ToolError(err)
		}
		limit := req.GetInt("limit", 25)
		page := req.GetInt("page", 1)
		view := req.GetString("view", "")
		component := req.GetString("component", "")
		params := map[string]string{"release": release}
		if view != "" {
			params["view"] = view
		}
		if component != "" {
			params["component"] = component
		}
		cacheKey := fmt.Sprintf("regressions:%s:%s:%s", release, view, component)
		data, err := cache.GetOrFetch(cacheKey, func() ([]byte, error) {
			raw, err := sippy.Get(ctx, "/api/component_readiness/regressions", params)
			if err != nil {
				return nil, err
			}
			if trimmed, err := client.ReshapeJSON[[]client.Regression](raw); err == nil {
				return trimmed, nil
			}
			return raw, nil
		})
		if err != nil {
			return tools.ToolError(err)
		}
		if paginated, err := client.PaginateArray(data, limit, page); err == nil {
			data = paginated
		}
		if filtered, err := client.FilterFields(data, req.GetString("fields", "")); err == nil {
			data = filtered
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}

func GetRegressionDetailHandler(sippy client.Sippy, cache *client.ResponseCache) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id, err := req.RequireString("regression_id")
		if err != nil {
			return tools.InvalidParam("regression_id", "required")
		}
		cacheKey := fmt.Sprintf("regression_detail:%s", id)
		data, err := cache.GetOrFetch(cacheKey, func() ([]byte, error) {
			regressionData, err := sippy.Get(ctx, fmt.Sprintf("/api/component_readiness/regressions/%s", id), nil)
			if err != nil {
				return nil, err
			}
			if trimmed, err := client.ReshapeJSON[client.Regression](regressionData); err == nil {
				regressionData = trimmed
			}
			matchesData, err := sippy.Get(ctx, fmt.Sprintf("/api/component_readiness/regressions/%s/matches", id), nil)
			if err != nil {
				return nil, err
			}
			if trimmed, err := client.ReshapeJSON[[]client.RegressionMatch](matchesData); err == nil {
				matchesData = trimmed
			}
			return []byte(fmt.Sprintf(`{"regression":%s,"matching_triages":%s}`, string(regressionData), string(matchesData))), nil
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
