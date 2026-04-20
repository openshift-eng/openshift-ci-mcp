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

func RegisterJobTools(s *server.MCPServer, sippy client.Sippy) {
	s.AddTool(
		mcp.NewTool("get_job_report",
			mcp.WithDescription("Get job pass rates with filtering by name, variant dimensions, and pass rate thresholds. Returns paginated results."),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithString("release", mcp.Description("Release version (e.g. '4.18'). Defaults to current dev release.")),
			mcp.WithString("job_name", mcp.Description("Filter jobs by name substring")),
			mcp.WithString("arch", mcp.Description("Filter by architecture: amd64, arm64, ppc64le, s390x, multi")),
			mcp.WithString("topology", mcp.Description("Filter by topology: ha, single, compact, external, microshift")),
			mcp.WithString("platform", mcp.Description("Filter by platform: aws, azure, gcp, metal, vsphere, rosa, etc.")),
			mcp.WithString("network", mcp.Description("Filter by network: ovn, sdn, cilium")),
			mcp.WithNumber("min_pass_rate", mcp.Description("Minimum pass rate percentage (e.g. 0)")),
			mcp.WithNumber("max_pass_rate", mcp.Description("Maximum pass rate percentage (e.g. 80)")),
			mcp.WithNumber("limit", mcp.Description("Max results per page (default 25)"), mcp.DefaultNumber(25)),
			mcp.WithNumber("page", mcp.Description("Page number (default 1)"), mcp.DefaultNumber(1)),
		),
		GetJobReportHandler(sippy),
	)

	s.AddTool(
		mcp.NewTool("get_job_runs",
			mcp.WithDescription("Get recent runs of a specific job with results and risk analysis"),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithString("release", mcp.Description("Release version. Defaults to current dev release.")),
			mcp.WithString("job_name", mcp.Required(), mcp.Description("Exact job name")),
			mcp.WithNumber("limit", mcp.Description("Max results (default 10)"), mcp.DefaultNumber(10)),
		),
		GetJobRunsHandler(sippy),
	)

	s.AddTool(
		mcp.NewTool("get_job_run_summary",
			mcp.WithDescription("Detailed summary of a single job run — test failures, cluster operator status"),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithString("prow_job_run_id", mcp.Required(), mcp.Description("Prow job run ID")),
		),
		GetJobRunSummaryHandler(sippy),
	)
}

func GetJobReportHandler(sippy client.Sippy) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		release, err := tools.ResolveRelease(ctx, sippy, req.GetString("release", ""))
		if err != nil {
			return tools.ToolError(err)
		}

		params := map[string]string{
			"release":   release,
			"sortField": "current_pass_percentage",
			"sort":      "asc",
			"perPage":   fmt.Sprintf("%d", req.GetInt("limit", 25)),
			"page":      fmt.Sprintf("%d", req.GetInt("page", 1)),
		}

		vp := extractVariantParams(req)
		if err := filter.MergeInto(params, vp); err != nil {
			return tools.ToolError(err)
		}

		if name := req.GetString("job_name", ""); name != "" {
			filter.MergeItemInto(params, filter.Item{
				ColumnField:   "name",
				OperatorValue: "contains",
				Value:         name,
			})
		}
		if minRate := req.GetFloat("min_pass_rate", -1); minRate >= 0 {
			filter.MergeItemInto(params, filter.Item{
				ColumnField:   "current_pass_percentage",
				OperatorValue: ">=",
				Value:         fmt.Sprintf("%g", minRate),
			})
		}
		if maxRate := req.GetFloat("max_pass_rate", -1); maxRate >= 0 {
			filter.MergeItemInto(params, filter.Item{
				ColumnField:   "current_pass_percentage",
				OperatorValue: "<=",
				Value:         fmt.Sprintf("%g", maxRate),
			})
		}

		data, err := sippy.Get(ctx, "/api/jobs", params)
		if err != nil {
			return tools.ToolError(err)
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}

func GetJobRunsHandler(sippy client.Sippy) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		release, err := tools.ResolveRelease(ctx, sippy, req.GetString("release", ""))
		if err != nil {
			return tools.ToolError(err)
		}

		jobName, err := req.RequireString("job_name")
		if err != nil {
			return tools.InvalidParam("job_name", "required")
		}

		params := map[string]string{
			"release": release,
			"perPage": fmt.Sprintf("%d", req.GetInt("limit", 10)),
			"filter":  fmt.Sprintf(`{"items":[{"columnField":"name","operatorValue":"equals","value":%q}],"linkOperator":"and"}`, jobName),
		}

		data, err := sippy.Get(ctx, "/api/jobs/runs", params)
		if err != nil {
			return tools.ToolError(err)
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}

func GetJobRunSummaryHandler(sippy client.Sippy) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		runID, err := req.RequireString("prow_job_run_id")
		if err != nil {
			return tools.InvalidParam("prow_job_run_id", "required")
		}

		params := map[string]string{"prow_job_run_id": runID}
		data, err := sippy.Get(ctx, "/api/job/run/summary", params)
		if err != nil {
			return tools.ToolError(err)
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}

func extractVariantParams(req mcp.CallToolRequest) filter.VariantParams {
	vp := filter.VariantParams{
		Arch:     req.GetString("arch", ""),
		Topology: req.GetString("topology", ""),
		Platform: req.GetString("platform", ""),
		Network:  req.GetString("network", ""),
	}
	args := req.GetArguments()
	if variants, ok := args["variants"].(map[string]any); ok {
		vp.Variants = make(map[string]string)
		for k, v := range variants {
			if s, ok := v.(string); ok {
				vp.Variants[k] = s
			}
		}
	}
	return vp
}
