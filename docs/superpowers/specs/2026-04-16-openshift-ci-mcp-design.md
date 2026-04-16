# openshift-ci-mcp Design Spec

## Overview

A Go MCP server providing read-only access to OpenShift CI data sources. Designed for broad OpenShift engineering use and AI tool interoperability (Claude Desktop, Cursor, custom agents, Claude Code plugins).

**Goals:**
- Make CI data accessible to any MCP-compatible client without requiring knowledge of Sippy's API surface
- Provide domain-oriented tools for common engineering questions (release health, job failures, regressions)
- Provide thin proxy tools as escape hatches for power users and advanced plugins
- Run locally in a container on user machines

**Non-goals (v1):**
- Write operations (triage, job triggering, JIRA filing)
- Authentication
- BigQuery direct access
- Workflow orchestration (that's the plugin layer's job)

## Data Sources

All read-only, no authentication required.

| Source | Base URL | Purpose |
|--------|----------|---------|
| Sippy | `https://sippy.dptools.openshift.org` | Central CI analysis — jobs, tests, component readiness, regressions, payloads, health |
| Release Controller | `https://{arch}.ocp.releases.ci.openshift.org` | Payload tags, acceptance status, blocking jobs |
| Search.CI | `https://search.ci.openshift.org` | Build log and JUnit failure search |

Base URLs are configurable via environment variables: `SIPPY_URL`, `RELEASE_CONTROLLER_URL`, `SEARCH_CI_URL`.

## Tools

### Domain-Oriented Tools (Tier B — Primary)

Tools organized around user intent. Each has sensible defaults and may compose multiple API calls internally.

#### Release & Health

| Tool | Purpose | Key Parameters | Upstream Endpoints |
|------|---------|---------------|-------------------|
| `get_releases` | List available releases with GA dates | — | `/api/releases` |
| `get_release_health` | Overall health — install/upgrade/infra rates, variant summary, payload acceptance | `release` (default: current dev) | `/api/health`, `/api/releases/health` |

#### Jobs

| Tool | Purpose | Key Parameters | Upstream Endpoints |
|------|---------|---------------|-------------------|
| `get_job_report` | Job pass rates with filtering | `release`, `job_name`, `arch`, `topology`, `platform`, `network`, `variants` (map), `min_pass_rate`, `max_pass_rate` | `/api/jobs` |
| `get_job_runs` | Recent runs of a job with results and risk analysis | `release`, `job_name`, `count` | `/api/jobs/runs`, `/api/jobs/runs/risk_analysis` |
| `get_job_run_summary` | Detailed summary of a single run — test failures, cluster operators | `prow_job_run_id` | `/api/job/run/summary` |

#### Tests

| Tool | Purpose | Key Parameters | Upstream Endpoints |
|------|---------|---------------|-------------------|
| `get_test_report` | Test pass/fail/flake rates | `release`, `test_name`, `arch`, `topology`, `platform`, `network`, `variants` (map), `component` | `/api/tests` |
| `get_test_details` | Test analysis by variant and by job | `release`, `test_name` | `/api/tests/details`, `/api/tests/analysis/variants`, `/api/tests/analysis/jobs` |
| `get_recent_test_failures` | Tests that recently started failing | `release` | `/api/tests/recent_failures` |

#### Component Readiness

| Tool | Purpose | Key Parameters | Upstream Endpoints |
|------|---------|---------------|-------------------|
| `get_component_readiness` | Component readiness report (the release gate) | `release`, `view` (default: main) | `/api/component_readiness`, `/api/component_readiness/views` |
| `get_regressions` | Active regressions from Component Readiness | `release`, `view`, `component` | `/api/component_readiness/regressions` |
| `get_regression_detail` | Specific regression with linked triages | `regression_id` | `/api/component_readiness/regressions/{id}`, `.../matches` |

#### Payloads

| Tool | Purpose | Key Parameters | Upstream Endpoints |
|------|---------|---------------|-------------------|
| `get_payload_status` | Recent payload acceptance/rejection status | `release`, `arch` (default: amd64) | Release Controller `/api/v1/releasestream/*/tags` |
| `get_payload_diff` | PRs that changed between two payloads | `release`, `from_tag`, `to_tag` | `/api/payloads/diff` |
| `get_payload_test_failures` | Test failures in payload runs | `release`, `payload_tag` | `/api/payloads/test_failures` |

#### Pull Requests

| Tool | Purpose | Key Parameters | Upstream Endpoints |
|------|---------|---------------|-------------------|
| `get_pull_request_impact` | Test failures associated with a PR | `org`, `repo`, `pr_number` | `/api/pull_requests/test_results` |
| `get_pull_requests` | PR reports with filtering | `release`, `org`, `repo` | `/api/pull_requests` |

#### Discovery

| Tool | Purpose | Key Parameters | Upstream Endpoints |
|------|---------|---------------|-------------------|
| `get_variants` | All variant dimensions and possible values | — | `/api/job_variants` |

#### Search

| Tool | Purpose | Key Parameters | Upstream Endpoints |
|------|---------|---------------|-------------------|
| `search_ci_logs` | Search build logs and JUnit failures | `query`, `max_age` | Search.CI API |

### Thin Proxy Tools (Tier A — Escape Hatch)

For power users and plugins that need raw API access.

| Tool | Purpose | Parameters |
|------|---------|-----------|
| `sippy_api` | Raw passthrough to any Sippy endpoint | `path`, `params` (map), `filter` (Sippy filter JSON) |
| `release_controller_api` | Raw passthrough to Release Controller | `arch`, `path`, `params` |
| `search_ci_api` | Raw passthrough to Search.CI | `query`, `params` |

### Variant Filtering

Domain tools that query jobs or tests accept these first-class variant parameters:

| Parameter | Type | Example Values |
|-----------|------|---------------|
| `arch` | string | amd64, arm64, ppc64le, s390x, multi |
| `topology` | string | ha, single, compact, external, microshift, two-node-arbiter, two-node-fencing |
| `platform` | string | aws, azure, gcp, metal, vsphere, rosa, etc. |
| `network` | string | ovn, sdn, cilium |
| `variants` | map[string]string | Any other dimension, e.g. `{"Installer": "upi", "SecurityMode": "fips"}` |

The first-class parameters (`arch`, `topology`, `platform`, `network`) are convenience aliases. During filter construction, they are merged into the `variants` map as `{"Architecture": arch, "Topology": topology, ...}` before conversion to Sippy's filter JSON. If both `arch` and `variants["Architecture"]` are provided, the explicit `arch` parameter takes precedence.

Sippy stores variants as colon-separated strings in a PostgreSQL array (e.g. `"Architecture:amd64"`). The `filter` package translates each variant entry into a Sippy filter item using the `has entry` operator on the `variants` column. Multiple variant filters are combined with `AND`.

The `get_variants` tool (wrapping `/api/job_variants`) lets clients discover all 26 variant dimensions and their possible values.

## Architecture

```
┌─────────────────────────────────────────────┐
│              MCP Client                      │
│  (Claude Desktop, Cursor, Claude Plugin)     │
└─────────┬───────────────────────────┬────────┘
          │ stdio (local)             │ HTTP/SSE (shared)
          ▼                           ▼
┌─────────────────────────────────────────────┐
│            openshift-ci-mcp                  │
│                                              │
│  ┌─────────┐  ┌──────────┐  ┌────────────┐  │
│  │Transport │  │ Tool     │  │ Tool       │  │
│  │Layer     │  │ Registry │  │ Registry   │  │
│  │(stdio/   │  │(Domain)  │  │(Proxy)     │  │
│  │ http)    │  └────┬─────┘  └─────┬──────┘  │
│  └────┬─────┘       │              │          │
│       │        ┌────▼──────────────▼──────┐   │
│       ├───────►│     Handler Layer        │   │
│       │        │  - param defaults        │   │
│       │        │  - variant expansion     │   │
│       │        │  - response trimming     │   │
│       │        └────────────┬─────────────┘   │
│       │                     │                 │
│       │        ┌────────────▼─────────────┐   │
│       │        │     Client Layer         │   │
│       │        │  ┌───────┐ ┌──────────┐  │   │
│       │        │  │Sippy  │ │Release   │  │   │
│       │        │  │Client │ │Controller│  │   │
│       │        │  │       │ │Client    │  │   │
│       │        │  └───────┘ └──────────┘  │   │
│       │        │  ┌───────┐               │   │
│       │        │  │Search │               │   │
│       │        │  │CI     │               │   │
│       │        │  │Client │               │   │
│       │        │  └───────┘               │   │
│       │        └──────────────────────────┘   │
│       │                                       │
└───────┴───────────────────────────────────────┘
```

**Layers:**

- **Transport Layer** — `mcp-go` handles stdio and HTTP/SSE. CLI flag `--transport stdio|http` selects mode (default: stdio). `--port 8080` for HTTP mode.
- **Tool Registry** — Domain and proxy tools registered separately, sharing the same handler interface. Each tool declares name, description, and JSON Schema input.
- **Tool Handlers** — Each tool has its own handler function. Domain tool handlers resolve defaults, call `filter.Build()` to expand variant parameters, call the appropriate client(s), and trim responses to useful fields. Proxy tool handlers pass parameters through to clients and return raw upstream responses unmodified. There is no shared "handler layer" — each tool owns its full request lifecycle.
- **Client Layer** — Typed HTTP clients per upstream service. Handles base URLs, retries, timeouts. Interface-based for testability.

**Default resolution:** When `release` is omitted, the tool handler fetches `/api/releases` and selects the release with the latest `ga_date` that has no `ga_date` set (i.e., still in development). If all releases have GA dates, the most recent GA release is used. This logic lives in a shared helper, not in each tool individually.

**Proxy tool responses:** Proxy tools return the raw upstream HTTP response body as the MCP tool result content (JSON text). No wrapping, no transformation. Errors from upstream are returned as MCP `isError: true` responses with the HTTP status code and body.

## Project Structure

```
openshift-ci-mcp/
├── cmd/
│   └── openshift-ci-mcp/
│       └── main.go              # CLI entry point, flag parsing, transport selection
├── pkg/
│   ├── server/
│   │   └── server.go            # MCP server setup, tool registration
│   ├── tools/
│   │   ├── domain/
│   │   │   ├── releases.go      # get_releases, get_release_health
│   │   │   ├── jobs.go          # get_job_report, get_job_runs, get_job_run_summary
│   │   │   ├── tests.go         # get_test_report, get_test_details, get_recent_test_failures
│   │   │   ├── component.go     # get_component_readiness, get_regressions, get_regression_detail
│   │   │   ├── payloads.go      # get_payload_status, get_payload_diff, get_payload_test_failures
│   │   │   ├── pullrequests.go  # get_pull_request_impact, get_pull_requests
│   │   │   ├── variants.go      # get_variants
│   │   │   └── search.go        # search_ci_logs
│   │   └── proxy/
│   │       ├── sippy.go         # sippy_api
│   │       ├── releasecontroller.go  # release_controller_api
│   │       └── searchci.go      # search_ci_api
│   ├── client/
│   │   ├── sippy.go
│   │   ├── releasecontroller.go
│   │   └── searchci.go
│   └── filter/
│       └── filter.go            # Variant params → Sippy filter JSON (Sippy-specific, not generic)
├── Containerfile
├── go.mod
├── go.sum
├── Makefile
└── README.md
```

## Container & Distribution

```dockerfile
FROM golang:1.24 AS builder
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 go build -o /openshift-ci-mcp ./cmd/openshift-ci-mcp

FROM scratch
COPY --from=builder /openshift-ci-mcp /openshift-ci-mcp
ENTRYPOINT ["/openshift-ci-mcp"]
```

- Image: `quay.io/<personal-namespace>/openshift-ci-mcp` (moves to `openshift-eng` once adopted)
- Tags: semver (`v0.1.0`) + `latest`
- `scratch` base image — ~10-15MB, no shell, minimal attack surface
- Static binary via `CGO_ENABLED=0`

**Client configuration (Claude Desktop):**
```json
{
  "mcpServers": {
    "openshift-ci": {
      "command": "podman",
      "args": ["run", "-i", "--rm", "quay.io/<namespace>/openshift-ci-mcp"]
    }
  }
}
```

**HTTP mode:**
```bash
podman run -p 8080:8080 quay.io/<namespace>/openshift-ci-mcp --transport http --port 8080
```

## Error Handling

All errors are returned as MCP tool results with `isError: true` and a JSON text content body containing `{"error": "<message>", "status_code": <int>}`.

- **Upstream API failures:** Return the HTTP status code and upstream error body.
- **Network timeout:** 30s default per upstream call, configurable via `--timeout` flag. Timeouts return `{"error": "upstream timeout", "status_code": 504}`.
- **Invalid parameters:** Validated before making upstream calls. Return `{"error": "<description>", "status_code": 400}`.
- **Rate limiting (429):** Returned to the MCP client as-is with retry-after info. No automatic retry — the client or agent decides whether to wait and retry.

## Testing

- **Client layer:** Interface-based. Tests use mock implementations.
- **Handler layer:** Unit tests per tool — given mock client responses, verify correct upstream calls and response shaping. Primary test surface.
- **Filter package:** Table-driven unit tests for variant parameter → Sippy filter JSON conversion.
- **Integration tests:** Behind `//go:build integration` tag. Available locally via `go test -tags=integration ./...` but not run by default `go test ./...`. Hit real Sippy API using a hardcoded known-stable release (e.g. `4.18`). CI runs these on every PR.
- **No MCP protocol tests** — `mcp-go` owns protocol correctness.

## Relationship to Existing Tools

- **Sippy's built-in MCP server** (`/mcp/v1/`): Only exposes 2 tools. This server is a superset, external, and independently deployable.
- **ai-helpers CI plugin**: Claude Code-specific. This MCP server provides the data layer that the plugin (and future plugins) can consume. Existing plugin skills can be updated to call MCP tools instead of running Python scripts directly.

## Future Considerations (Not in v1)

- Write operations: triage creation, job triggering, JIRA integration (requires auth)
- Caching layer for frequently-queried data (releases, variants, health)
- MCP Resources for slow-changing reference data
- BigQuery direct access for historical analysis
- Prometheus/Thanos metrics access
