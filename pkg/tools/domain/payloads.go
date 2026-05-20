package domain

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/openshift-eng/openshift-ci-mcp/pkg/client"
	"github.com/openshift-eng/openshift-ci-mcp/pkg/filter"
	"github.com/openshift-eng/openshift-ci-mcp/pkg/tools"
)

func RegisterPayloadTools(s *server.MCPServer, sippy client.Sippy, rc client.ReleaseController, cache *client.ResponseCache) {
	s.AddTool(mcp.NewTool("get_payload_status",
		mcp.WithDescription("Use to get recent payload acceptance status from the Release Controller."),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(true),
		mcp.WithString("release", mcp.Required(), mcp.Description("Release version (e.g. '4.18')")),
		mcp.WithString("arch", mcp.Description("Architecture (default: amd64)"), mcp.DefaultString("amd64")),
		mcp.WithString("stream", mcp.Description("Stream: 'nightly' or 'ci' (default: nightly)"), mcp.DefaultString("nightly")),
		mcp.WithNumber("limit", mcp.Description("Max payload tags to return (default 10)"), mcp.DefaultNumber(10)),
		mcp.WithString("fields", mcp.Description("Comma-separated list of field names to include in response (default: all)")),
	), GetPayloadStatusHandler(rc))

	s.AddTool(mcp.NewTool("get_payload_diff",
		mcp.WithDescription("Use to list pull request changes between payload tags."),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(true),
		mcp.WithString("release", mcp.Description("Release version (e.g. '4.18')")),
		mcp.WithString("from_tag", mcp.Description("Source payload tag (default: previous payload)")),
		mcp.WithString("to_tag", mcp.Required(), mcp.Description("Target payload tag")),
		mcp.WithNumber("limit", mcp.Description("Max results per page (default 25)"), mcp.DefaultNumber(25)),
		mcp.WithNumber("page", mcp.Description("Page number (default 1)"), mcp.DefaultNumber(1)),
		mcp.WithString("fields", mcp.Description("Comma-separated list of field names to include in response (default: all)")),
	), GetPayloadDiffHandler(sippy, cache))

	s.AddTool(mcp.NewTool("get_payload_test_failures",
		mcp.WithDescription("Use to get test failures for payload job runs"),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(true),
		mcp.WithString("release", mcp.Description("Release version (e.g. '4.18')")),
		mcp.WithString("payload_tag", mcp.Description("Specific payload tag to check")),
		mcp.WithString("test_name", mcp.Description("Test name substring filter")),
		mcp.WithNumber("limit", mcp.Description("Max results per page (default 25)"), mcp.DefaultNumber(25)),
		mcp.WithNumber("page", mcp.Description("Page number (default 1)"), mcp.DefaultNumber(1)),
		mcp.WithString("fields", mcp.Description("Comma-separated list of field names to include in response (default: all)")),
	), GetPayloadTestFailuresHandler(sippy, cache))
}

func GetPayloadStatusHandler(rc client.ReleaseController) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		release, err := req.RequireString("release")
		if err != nil {
			return tools.InvalidParam("release", "required")
		}
		arch := req.GetString("arch", "amd64")
		stream := req.GetString("stream", "nightly")
		limit := req.GetInt("limit", 10)
		streamName := fmt.Sprintf("%s.0-0.%s", release, stream)
		if arch != "amd64" && stream == "nightly" {
			streamName = fmt.Sprintf("%s.0-0.%s-%s", release, stream, arch)
		}
		path := fmt.Sprintf("/api/v1/releasestream/%s/tags", streamName)
		data, err := rc.GetForArch(ctx, arch, path, nil)
		if err != nil {
			return tools.ToolError(err)
		}
		if truncated, err := truncatePayloadTags(data, limit); err == nil {
			data = truncated
		}
		if filtered, err := client.FilterFields(data, req.GetString("fields", "")); err == nil {
			data = filtered
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}

func truncatePayloadTags(data []byte, limit int) ([]byte, error) {
	var payload client.ReleaseStreamTags
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, err
	}
	if len(payload.Tags) > limit {
		payload.Tags = payload.Tags[:limit]
	}
	return json.Marshal(payload)
}

func GetPayloadDiffHandler(sippy client.Sippy, cache *client.ResponseCache) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		toTag, err := req.RequireString("to_tag")
		if err != nil {
			return tools.InvalidParam("to_tag", "required")
		}
		limit := req.GetInt("limit", 25)
		page := req.GetInt("page", 1)
		fromTag := req.GetString("from_tag", "")
		params := map[string]string{"toPayload": toTag}
		if fromTag != "" {
			params["fromPayload"] = fromTag
		}
		cacheKey := fmt.Sprintf("payload_diff:%s:%s", toTag, fromTag)
		data, err := cache.GetOrFetch(cacheKey, func() ([]byte, error) {
			raw, err := sippy.Get(ctx, "/api/payloads/diff", params)
			if err != nil {
				return nil, err
			}
			if trimmed, err := client.ReshapeJSON[[]client.PayloadDiffRow](raw); err == nil {
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

func GetPayloadTestFailuresHandler(sippy client.Sippy, cache *client.ResponseCache) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		release, err := tools.ResolveRelease(ctx, sippy, req.GetString("release", ""))
		if err != nil {
			return tools.ToolError(err)
		}
		limit := req.GetInt("limit", 25)
		page := req.GetInt("page", 1)
		payloadTag := req.GetString("payload_tag", "")
		params := map[string]string{"release": release}
		if payloadTag != "" {
			params["payload"] = payloadTag
		}
		if name := req.GetString("test_name", ""); name != "" {
			filter.MergeItemInto(params, filter.Item{ColumnField: "name", OperatorValue: "contains", Value: name})
		}
		cacheKey := fmt.Sprintf("payload_test_failures:%s:%s:%s", release, payloadTag, params["filter"])
		data, err := cache.GetOrFetch(cacheKey, func() ([]byte, error) {
			raw, err := sippy.Get(ctx, "/api/payloads/test_failures", params)
			if err != nil {
				return nil, err
			}
			if trimmed, err := client.ReshapeJSON[[]client.PayloadTestFailure](raw); err == nil {
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
