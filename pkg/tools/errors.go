package tools

import (
	"errors"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/jeroche/openshift-ci-mcp/pkg/client"
)

func ToolError(err error) (*mcp.CallToolResult, error) {
	var httpErr *client.HTTPError
	if errors.As(err, &httpErr) {
		msg := fmt.Sprintf(`{"error":%q,"status_code":%d}`, httpErr.Error(), httpErr.StatusCode)
		return mcp.NewToolResultError(msg), nil
	}
	msg := fmt.Sprintf(`{"error":%q,"status_code":500}`, err.Error())
	return mcp.NewToolResultError(msg), nil
}

func InvalidParam(name, detail string) (*mcp.CallToolResult, error) {
	msg := fmt.Sprintf(`{"error":"invalid parameter %q: %s","status_code":400}`, name, detail)
	return mcp.NewToolResultError(msg), nil
}
