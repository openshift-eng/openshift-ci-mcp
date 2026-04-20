package domain

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/jeroche/openshift-ci-mcp/pkg/client"
	"github.com/jeroche/openshift-ci-mcp/pkg/tools"
)

func RegisterPayloadTools(s *server.MCPServer, sippy client.Sippy, rc client.ReleaseController) {
	s.AddTool(mcp.NewTool("get_payload_status",
		mcp.WithDescription("Recent payload acceptance/rejection status from the Release Controller. Shows payload tags with their phase (Accepted/Rejected/Ready)."),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithString("release", mcp.Required(), mcp.Description("Release version (e.g. '4.18')")),
		mcp.WithString("arch", mcp.Description("Architecture (default: amd64)"), mcp.DefaultString("amd64")),
		mcp.WithString("stream", mcp.Description("Release stream: 'nightly' or 'ci' (default: nightly)"), mcp.DefaultString("nightly")),
	), GetPayloadStatusHandler(rc))

	s.AddTool(mcp.NewTool("get_payload_diff",
		mcp.WithDescription("PRs that changed between two payloads — useful for identifying what went into a rejected payload"),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithString("release", mcp.Description("Release version. Defaults to current dev release.")),
		mcp.WithString("from_tag", mcp.Description("Source payload tag (if omitted, uses previous payload automatically)")),
		mcp.WithString("to_tag", mcp.Required(), mcp.Description("Target payload tag")),
	), GetPayloadDiffHandler(sippy))

	s.AddTool(mcp.NewTool("get_payload_test_failures",
		mcp.WithDescription("Test failures across payload runs"),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithString("release", mcp.Description("Release version. Defaults to current dev release.")),
		mcp.WithString("payload_tag", mcp.Description("Specific payload tag to check")),
	), GetPayloadTestFailuresHandler(sippy))
}

func GetPayloadStatusHandler(rc client.ReleaseController) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		release, err := req.RequireString("release")
		if err != nil {
			return tools.InvalidParam("release", "required")
		}
		arch := req.GetString("arch", "amd64")
		stream := req.GetString("stream", "nightly")
		streamName := fmt.Sprintf("%s.0-0.%s", release, stream)
		if arch != "amd64" && stream == "nightly" {
			streamName = fmt.Sprintf("%s.0-0.%s-%s", release, stream, arch)
		}
		path := fmt.Sprintf("/api/v1/releasestream/%s/tags", streamName)
		data, err := rc.GetForArch(ctx, arch, path, nil)
		if err != nil {
			return tools.ToolError(err)
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}

func GetPayloadDiffHandler(sippy client.Sippy) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		toTag, err := req.RequireString("to_tag")
		if err != nil {
			return tools.InvalidParam("to_tag", "required")
		}
		params := map[string]string{"toPayload": toTag}
		if fromTag := req.GetString("from_tag", ""); fromTag != "" {
			params["fromPayload"] = fromTag
		}
		data, err := sippy.Get(ctx, "/api/payloads/diff", params)
		if err != nil {
			return tools.ToolError(err)
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}

func GetPayloadTestFailuresHandler(sippy client.Sippy) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		release, err := tools.ResolveRelease(ctx, sippy, req.GetString("release", ""))
		if err != nil {
			return tools.ToolError(err)
		}
		params := map[string]string{"release": release}
		if tag := req.GetString("payload_tag", ""); tag != "" {
			params["payload"] = tag
		}
		data, err := sippy.Get(ctx, "/api/payloads/test_failures", params)
		if err != nil {
			return tools.ToolError(err)
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}
