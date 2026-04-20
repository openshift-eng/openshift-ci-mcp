package domain

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/jeroche/openshift-ci-mcp/pkg/client"
	"github.com/jeroche/openshift-ci-mcp/pkg/filter"
	"github.com/jeroche/openshift-ci-mcp/pkg/tools"
)

func RegisterTestTools(s *server.MCPServer, sippy client.Sippy) {
	s.AddTool(mcp.NewTool("get_test_report",
		mcp.WithDescription("Get test pass/fail/flake rates with filtering by name, component, and variant dimensions"),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithString("release", mcp.Description("Release version. Defaults to current dev release.")),
		mcp.WithString("test_name", mcp.Description("Filter tests by name substring")),
		mcp.WithString("component", mcp.Description("Filter by Jira component")),
		mcp.WithString("arch", mcp.Description("Filter by architecture")),
		mcp.WithString("topology", mcp.Description("Filter by topology")),
		mcp.WithString("platform", mcp.Description("Filter by platform")),
		mcp.WithString("network", mcp.Description("Filter by network")),
		mcp.WithNumber("limit", mcp.Description("Max results per page (default 25)"), mcp.DefaultNumber(25)),
		mcp.WithNumber("page", mcp.Description("Page number (default 1)"), mcp.DefaultNumber(1)),
	), GetTestReportHandler(sippy))

	s.AddTool(mcp.NewTool("get_test_details",
		mcp.WithDescription("Detailed test analysis — pass rates broken down by variant and by job"),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithString("release", mcp.Description("Release version. Defaults to current dev release.")),
		mcp.WithString("test_name", mcp.Required(), mcp.Description("Exact test name")),
	), GetTestDetailsHandler(sippy))

	s.AddTool(mcp.NewTool("get_recent_test_failures",
		mcp.WithDescription("Tests that have recently started failing — useful for detecting new regressions"),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithString("release", mcp.Description("Release version. Defaults to current dev release.")),
		mcp.WithString("period", mcp.Description("Time window as a Go duration (e.g. '168h' for 7 days, '48h' for 2 days). Default: 168h")),
	), GetRecentTestFailuresHandler(sippy))
}

func GetTestReportHandler(sippy client.Sippy) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		release, err := tools.ResolveRelease(ctx, sippy, req.GetString("release", ""))
		if err != nil {
			return tools.ToolError(err)
		}
		params := map[string]string{
			"release": release,
			"perPage": fmt.Sprintf("%d", req.GetInt("limit", 25)),
			"page":    fmt.Sprintf("%d", req.GetInt("page", 1)),
		}
		if name := req.GetString("test_name", ""); name != "" {
			filter.MergeItemInto(params, filter.Item{ColumnField: "name", OperatorValue: "contains", Value: name})
		}
		if component := req.GetString("component", ""); component != "" {
			filter.MergeItemInto(params, filter.Item{ColumnField: "jira_component", OperatorValue: "equals", Value: component})
		}
		vp := extractVariantParams(req)
		if err := filter.MergeInto(params, vp); err != nil {
			return tools.ToolError(err)
		}
		data, err := sippy.Get(ctx, "/api/tests", params)
		if err != nil {
			return tools.ToolError(err)
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}

func GetTestDetailsHandler(sippy client.Sippy) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		release, err := tools.ResolveRelease(ctx, sippy, req.GetString("release", ""))
		if err != nil {
			return tools.ToolError(err)
		}
		testName, err := req.RequireString("test_name")
		if err != nil {
			return tools.InvalidParam("test_name", "required")
		}
		params := map[string]string{"release": release, "test": testName}
		data, err := sippy.Get(ctx, "/api/tests/details", params)
		if err != nil {
			return tools.ToolError(err)
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}

func GetRecentTestFailuresHandler(sippy client.Sippy) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		release, err := tools.ResolveRelease(ctx, sippy, req.GetString("release", ""))
		if err != nil {
			return tools.ToolError(err)
		}
		params := map[string]string{
			"release": release,
			"period":  req.GetString("period", "168h"),
		}
		data, err := sippy.Get(ctx, "/api/tests/recent_failures", params)
		if err != nil {
			return tools.ToolError(err)
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}
