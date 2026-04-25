# Developing

## Prerequisites

- Go 1.25+
- Python 3 (for smoke tests)
- Podman (for container builds)
- `gh` CLI (for releases)

## Contributing

Fork the repo and open a pull request. All changes should go through PR review — do not push directly to `main`.

Use semantic commit messages:

```text
feat: add get_payload_upgrades tool
fix: correct parameter name for payload test failures
docs: update variant filtering examples
chore: bump mcp-go to v0.49.0
test: add coverage for release resolution edge cases
```

## Build & Test

```bash
make build              # Build binary to bin/
make test               # Unit tests
make test-integration   # Integration tests (requires network, hits live APIs)
make lint               # go vet
make smoke              # Build + smoke test against binary
make smoke-container    # Smoke test against container image
```

Run a single test:

```bash
go test ./pkg/tools/domain/... -run TestGetJobReport -v
```

## Container

```bash
make image              # Build with podman
make push               # Build + push to quay.io

# Override image/version:
VERSION=0.2.0 make push
IMAGE=quay.io/myuser/openshift-ci-mcp make push
```

The `Containerfile` uses a multi-stage build: `golang:1.24` builder with `CGO_ENABLED=0`, then copies the static binary into a `scratch` image with CA certificates.

## Running Locally

```bash
# stdio (default, for MCP clients)
bin/openshift-ci-mcp

# HTTP/SSE
bin/openshift-ci-mcp --transport http --port 8080
```

### Environment Variables

| Variable | Default | Description |
| -------- | ------- | ----------- |
| `SIPPY_URL` | `https://sippy.dptools.openshift.org` | Sippy API base URL |
| `RELEASE_CONTROLLER_URL` | `https://amd64.ocp.releases.ci.openshift.org` | Release Controller base URL |
| `SEARCH_CI_URL` | `https://search.ci.openshift.org` | Search.CI base URL |

### CLI Flags

| Flag | Default | Description |
| ---- | ------- | ----------- |
| `--transport` | `stdio` | `stdio` or `http` |
| `--port` | `8080` | HTTP port (with `--transport http`) |
| `--timeout` | `30s` | Upstream request timeout |

## Adding a New Tool

1. Add a handler function in the appropriate `pkg/tools/domain/*.go` file. Handlers are closures that capture a client interface:

    ```go
    func GetThingHandler(sippy client.Sippy) server.ToolHandlerFunc {
        return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
            // ...
        }
    }
    ```

2. Register the tool in the `Register*Tools` function in the same file. Use `mcp.NewTool` with parameter definitions.

3. If the tool needs variant filtering, call `extractVariantParams(req)` and `filter.MergeInto(params, vp)`. For non-variant filters, use `filter.MergeItemInto()`.

4. Wire it up in `pkg/server/server.go` inside `New()` if you created a new registration function.

5. Add tests using `newMockSippy()` or `newCapturingSippy()` from `testhelpers_test.go`.

6. Verify:

    ```bash
    go test ./pkg/tools/domain/... -v
    go vet ./...
    ```

**Important:** Probe the actual Sippy API before building. Parameter names and response shapes often differ from what you'd expect. Use curl or the `sippy-api-explorer` agent.

## Smoke Tests

The smoke test suite (`tests/smoke_test.py`) starts the MCP server over stdio, calls all 21 tools against live upstream APIs, and validates responses. Tests with dependencies chain automatically (e.g., `get_job_report` provides the job name for `get_job_runs`).

```bash
# Against binary
python3 tests/smoke_test.py --binary bin/openshift-ci-mcp

# Against container
python3 tests/smoke_test.py --container quay.io/{{org_name}}/openshift-ci-mcp:latest

# Specific tools only
python3 tests/smoke_test.py --binary bin/openshift-ci-mcp --tools get_releases get_variants

# Custom release/timeout
python3 tests/smoke_test.py --binary bin/openshift-ci-mcp --release 4.19 --timeout 60
```

## Project Layout

```text
cmd/openshift-ci-mcp/       Entry point, CLI flags, transport setup
pkg/
  server/                    MCP server factory, wires all tools
  client/                    HTTP clients (Sippy, ReleaseController, SearchCI)
  filter/                    Variant params to Sippy filter JSON
  tools/
    domain/                  Intent-oriented tools with defaults and filtering
    proxy/                   Raw passthrough to upstream APIs
    errors.go                ToolError/InvalidParam helpers
    resolve.go               Default release resolution
tests/
  smoke_test.py              End-to-end smoke tests (Python, stdlib only)
```

## Claude Code Integration

The project includes Claude Code automations in `.claude/`:

**Skills** (invoke with `/skill-name`):

- `/smoke-test` — Run smoke tests
- `/add-tool` — Scaffold a new MCP tool
- `/generate-release` — Version, build, tag, and publish a release

**Agents:**

- `sippy-api-explorer` — Probe a Sippy API endpoint to discover its contract

**Hooks:**

- `PostToolUse` — Runs `go vet` after editing `.go` files
- `PreToolUse` — Blocks edits to `go.sum` and `bin/`
