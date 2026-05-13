package domain

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/openshift-eng/openshift-ci-mcp/pkg/client"
	"github.com/openshift-eng/openshift-ci-mcp/pkg/filter"
	"github.com/openshift-eng/openshift-ci-mcp/pkg/tools"
)

func RegisterTestTools(s *server.MCPServer, sippy client.Sippy) {
	s.AddTool(mcp.NewTool("get_ci_test_report",
		mcp.WithDescription("Use to get pass/fail/flake rates for tests with optional filtering"),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(true),
		mcp.WithString("release", mcp.Description("Release version. Default: current dev release.")),
		mcp.WithString("test_name", mcp.Description("Test name substring filter")),
		mcp.WithString("component", mcp.Description("Jira component name")),
		mcp.WithString("arch", mcp.Description("Architecture: amd64, arm64, ppc64le, s390x, multi")),
		mcp.WithString("topology", mcp.Description("Topology: ha, single, compact, external, microshift")),
		mcp.WithString("platform", mcp.Description("Platform: aws, azure, gcp, metal, vsphere, rosa, etc.")),
		mcp.WithString("network", mcp.Description("Network: ovn, sdn, cilium")),
		mcp.WithNumber("limit", mcp.Description("Max results per page (default 25)"), mcp.DefaultNumber(25)),
		mcp.WithNumber("page", mcp.Description("Page number (default 1)"), mcp.DefaultNumber(1)),
	), GetTestReportHandler(sippy))

	s.AddTool(mcp.NewTool("get_test_details",
		mcp.WithDescription("Use to get pass rates broken down by variant and by job."),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(true),
		mcp.WithString("release", mcp.Description("Release version. Defaults to current dev release.")),
		mcp.WithString("test_name", mcp.Required(), mcp.Description("Exact test name")),
	), GetTestDetailsHandler(sippy))

	s.AddTool(mcp.NewTool("get_recent_test_failures",
		mcp.WithDescription("Tests that recently started failing, useful for detecting new regressions."),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(true),
		mcp.WithString("release", mcp.Description("Release version. Defaults to current dev release.")),
		mcp.WithString("period", mcp.Description("Time window (e.g. '168h', '48h'). Default: 168h")),
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
