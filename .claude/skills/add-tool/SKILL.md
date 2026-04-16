---
name: add-tool
description: Scaffold a new MCP tool with handler, registration, and test
---

Add a new domain tool to the MCP server. Follow the established patterns:

## Steps

1. **Identify the right file** in `pkg/tools/domain/`. Group related tools together (jobs.go, tests.go, component.go, payloads.go, pullrequests.go, search.go, releases.go, variants.go). Create a new file only if the tool doesn't fit an existing group.

2. **Write the handler** as a closure that captures the client interface:
   ```go
   func GetThingHandler(sippy client.Sippy) server.ToolHandlerFunc {
       return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
           // ...
       }
   }
   ```

3. **Register the tool** in the `Register*Tools` function in the same file. If creating a new file, add a new registration function and call it from `pkg/server/server.go` inside `New()`.

4. **Apply variant filtering** if the tool queries jobs or tests:
   ```go
   vp := extractVariantParams(req)
   filter.MergeInto(params, vp)
   ```

5. **Use filter.MergeItemInto** for non-variant filters (name, component, org, etc.):
   ```go
   filter.MergeItemInto(params, filter.Item{ColumnField: "name", OperatorValue: "contains", Value: name})
   ```

6. **Resolve the release** when it's optional:
   ```go
   release, err := tools.ResolveRelease(ctx, sippy, req.GetString("release", ""))
   ```

7. **Write tests** using `newMockSippy()` and `newCapturingSippy()` from `testhelpers_test.go`. Mock provides canned responses; CapturingSippy lets you assert on parameters sent upstream.

8. **Verify**: Run `go test ./pkg/tools/domain/... -v` and `go vet ./...`.

## Before building

Probe the upstream Sippy API endpoint to confirm required parameters and response shape. Use curl or invoke the `/sippy-api-explorer` agent. Don't assume parameter names match the Sippy UI — verify against the actual API.
