package domain

import (
	"context"
	"fmt"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/openshift-eng/openshift-ci-mcp/pkg/client"
	"github.com/openshift-eng/openshift-ci-mcp/pkg/filter"
	"github.com/openshift-eng/openshift-ci-mcp/pkg/tools"
)

func RegisterPullRequestTools(s *server.MCPServer, sippy client.Sippy) {
	s.AddTool(mcp.NewTool("get_pr_impact",
		mcp.WithDescription("Use to get test failure impact data for a specific, known pull request. Rate-limited to 20 req/hour."),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(true),
		mcp.WithString("org", mcp.Required(), mcp.Description("GitHub org (e.g. 'openshift')")),
		mcp.WithString("repo", mcp.Required(), mcp.Description("GitHub repo (e.g. 'kubernetes')")),
		mcp.WithString("pr_number", mcp.Required(), mcp.Description("GitHub pull request number")),
		mcp.WithString("start_date", mcp.Description("Start date (YYYY-MM-DD, default: 14 days ago)")),
		mcp.WithString("end_date", mcp.Description("End date (YYYY-MM-DD, default: today)")),
	), GetPullRequestImpactHandler(sippy))

	s.AddTool(mcp.NewTool("get_release_prs",
		mcp.WithDescription("Use to get a list of pull requests for a specific release or presubmits"),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(true),
		mcp.WithString("release", mcp.Description("Release version. Can be a version number (e.g. '4.18') or the default: 'Presubmits'")),
		mcp.WithString("org", mcp.Description("GitHub org")),
		mcp.WithString("repo", mcp.Description("GitHub repo")),
		mcp.WithNumber("limit", mcp.Description("Max results per page (default 25)"), mcp.DefaultNumber(25)),
		mcp.WithNumber("page", mcp.Description("Page number (default 1)"), mcp.DefaultNumber(1)),
	), GetPullRequestsHandler(sippy))
}

func GetPullRequestImpactHandler(sippy client.Sippy) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		org, err := req.RequireString("org")
		if err != nil {
			return tools.InvalidParam("org", "required")
		}
		repo, err := req.RequireString("repo")
		if err != nil {
			return tools.InvalidParam("repo", "required")
		}
		prNumber, err := req.RequireString("pr_number")
		if err != nil {
			return tools.InvalidParam("pr_number", "required")
		}
		dateFmt := "2006-01-02"
		startDate := req.GetString("start_date", time.Now().AddDate(0, 0, -14).Format(dateFmt))
		endDate := req.GetString("end_date", time.Now().Format(dateFmt))
		params := map[string]string{
			"org":        org,
			"repo":       repo,
			"pr_number":  prNumber,
			"start_date": startDate,
			"end_date":   endDate,
		}
		data, err := sippy.Get(ctx, "/api/pull_requests/test_results", params)
		if err != nil {
			return tools.ToolError(err)
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}

func GetPullRequestsHandler(sippy client.Sippy) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		release := req.GetString("release", "Presubmits")
		params := map[string]string{
			"release": release,
			"perPage": fmt.Sprintf("%d", req.GetInt("limit", 25)),
			"page":    fmt.Sprintf("%d", req.GetInt("page", 1)),
		}
		if org := req.GetString("org", ""); org != "" {
			filter.MergeItemInto(params, filter.Item{ColumnField: "org", OperatorValue: "equals", Value: org})
		}
		if repo := req.GetString("repo", ""); repo != "" {
			filter.MergeItemInto(params, filter.Item{ColumnField: "repo", OperatorValue: "equals", Value: repo})
		}
		data, err := sippy.Get(ctx, "/api/pull_requests", params)
		if err != nil {
			return tools.ToolError(err)
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}
