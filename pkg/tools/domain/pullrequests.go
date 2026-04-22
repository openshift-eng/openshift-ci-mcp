package domain

import (
	"context"
	"fmt"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/jeroche/openshift-ci-mcp/pkg/client"
	"github.com/jeroche/openshift-ci-mcp/pkg/filter"
	"github.com/jeroche/openshift-ci-mcp/pkg/tools"
)

func RegisterPullRequestTools(s *server.MCPServer, sippy client.Sippy) {
	s.AddTool(mcp.NewTool("get_pull_request_impact",
		mcp.WithDescription("Get detailed test failure data for a SPECIFIC pull request (requires a known PR number). To find PR numbers first, use get_pull_requests. Rate-limited to 20 req/hour."),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithString("org", mcp.Required(), mcp.Description("GitHub org (e.g. 'openshift')")),
		mcp.WithString("repo", mcp.Required(), mcp.Description("GitHub repo (e.g. 'kubernetes')")),
		mcp.WithString("pr_number", mcp.Required(), mcp.Description("Pull request number")),
		mcp.WithString("start_date", mcp.Description("Start date for test results (YYYY-MM-DD). Defaults to 14 days ago.")),
		mcp.WithString("end_date", mcp.Description("End date for test results (YYYY-MM-DD). Defaults to today.")),
	), GetPullRequestImpactHandler(sippy))

	s.AddTool(mcp.NewTool("get_pull_requests",
		mcp.WithDescription("List pull requests with titles, status, and summary data. Use this to discover and browse PRs. Supports filtering by org, repo, and release. For detailed CI test impact of a specific PR, use get_pull_request_impact after finding the PR number here."),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithString("release", mcp.Description("Release version. Defaults to 'Presubmits'.")),
		mcp.WithString("org", mcp.Description("Filter by GitHub org")),
		mcp.WithString("repo", mcp.Description("Filter by GitHub repo")),
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
