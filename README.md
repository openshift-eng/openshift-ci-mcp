# openshift-ci-mcp

![Go](https://img.shields.io/github/go-mod/go-version/openshift-eng/openshift-ci-mcp)
![License](https://img.shields.io/github/license/openshift-eng/openshift-ci-mcp)
![Release](https://img.shields.io/github/v/release/openshift-eng/openshift-ci-mcp)

MCP server providing read-only access to OpenShift CI data. Query Sippy, Release Controller, and Search.CI from any MCP-compatible client.

## Tools

<!-- Tool tables are auto-generated from source. Run `make generate` to update. -->

### Domain Tools

<!-- BEGIN DOMAIN TOOLS -->
| Tool | Description |
| ---- | ----------- |
| `get_component_readiness` | Component readiness report — the binding release gate. Shows statistical analysis comparing current release behavior against the previous stable release. This endpoint can be slow (30+ seconds). If view is omitted, the server auto-discovers the first available view, which adds an extra API call. |
| `get_job_report` | Get job pass rates with filtering by name, variant dimensions, and pass rate thresholds. Returns paginated results. |
| `get_job_run_summary` | Detailed summary of a single job run — test failures, cluster operator status |
| `get_job_runs` | Get recent runs of a specific CI job with pass/fail results, timings, and risk analysis. This is the primary tool for fetching job run history — do not use sippy_api, search_ci_api, or search_ci_logs for this purpose. Release version can be inferred from job names (e.g. '4.18' from 'nightly-4.18-e2e-aws'). |
| `get_payload_diff` | PRs that changed between two payloads — useful for identifying what went into a rejected payload |
| `get_payload_status` | Recent payload acceptance/rejection status from the Release Controller. Shows payload tags with their phase (Accepted/Rejected/Ready). |
| `get_payload_test_failures` | Test failures across payload runs |
| `get_pull_request_impact` | Get detailed test failure data for a SPECIFIC pull request (requires a known PR number). To find PR numbers first, use get_pull_requests. Rate-limited to 20 req/hour. |
| `get_pull_requests` | List pull requests with titles, status, and summary data. Use this to discover and browse PRs. Supports filtering by org, repo, and release. For detailed CI test impact of a specific PR, use get_pull_request_impact after finding the PR number here. |
| `get_recent_test_failures` | Tests that have recently started failing — useful for detecting new regressions |
| `get_regression_detail` | Details of a specific regression including linked triages and Jira bugs |
| `get_regressions` | Active regressions from Component Readiness — tests that are performing significantly worse than the previous release |
| `get_release_health` | Overall health of a release — install/upgrade/infrastructure success rates, variant summary, and payload acceptance statistics |
| `get_releases` | List available OpenShift releases with GA dates and development start dates |
| `get_test_details` | Detailed test analysis — pass rates broken down by variant and by job |
| `get_test_report` | Get test pass/fail/flake rates with filtering by name, component, and variant dimensions |
| `get_variants` | List all variant dimensions and their possible values. Use this to discover valid values for arch, topology, platform, network, and other variant filters. |
| `search_ci_logs` | Search build logs and JUnit failures across OpenShift CI for specific error messages, test names, or patterns. Not for fetching job run history (use get_job_runs) or listing jobs (use get_job_report). |
<!-- END DOMAIN TOOLS -->

### Proxy Tools

Raw passthrough to upstream APIs for advanced use cases.

<!-- BEGIN PROXY TOOLS -->
| Tool | Description |
| ---- | ----------- |
| `release_controller_api` | Raw passthrough to the Release Controller API. Returns unmodified upstream response. |
| `search_ci_api` | Low-level passthrough to Search.CI API (advanced use only). Prefer search_ci_logs for searching build logs and test failures. |
| `sippy_api` | Low-level passthrough to any Sippy API endpoint (advanced use only). Prefer domain-specific tools (get_job_runs, get_job_report, get_test_report, get_pull_requests, etc.) which handle filtering, pagination, and release resolution automatically. |
<!-- END PROXY TOOLS -->

## Usage

### stdio (default) (recommended)

```bash
# Run directly
bin/openshift-ci-mcp

# Run in container
podman run -i --rm quay.io/rh-edge-enablement/openshift-ci-mcp
```

### HTTP/SSE

```bash
bin/openshift-ci-mcp --transport http --port 8080

# Or in container
podman run -p 8080:8080 quay.io/rh-edge-enablement/openshift-ci-mcp --transport http --port 8080
```

### Claude Desktop

`go run` (no build required — recommended):

```json
{
  "mcpServers": {
    "openshift-ci": {
      "command": "go",
      "args": [
        "run",
        "github.com/openshift-eng/openshift-ci-mcp/cmd/openshift-ci-mcp@latest"
      ]
    }
  }
}
```

Local binary:

```json
{
  "mcpServers": {
    "openshift-ci": {
      "command": "/path/to/openshift-ci-mcp"
    }
  }
}
```

Container:

```json
{
  "mcpServers": {
    "openshift-ci": {
      "command": "podman",
      "args": ["run", "-i", "--rm", "quay.io/rh-edge-enablement/openshift-ci-mcp"]
    }
  }
}
```

### Claude Code

```bash
claude mcp add openshift-ci go -- run github.com/openshift-eng/openshift-ci-mcp/cmd/openshift-ci-mcp@latest
```

## Configuration

### CLI Flags

| Flag | Default | Description |
| ---- | ------- | ----------- |
| `--transport` | `stdio` | Transport mode: `stdio` or `http` |
| `--port` | `8080` | HTTP port (only used with `--transport http`) |
| `--timeout` | `30s` | Upstream request timeout |

### Environment Variables

| Variable | Default | Description |
| -------- | ------- | ----------- |
| `SIPPY_URL` | `https://sippy.dptools.openshift.org` | Sippy base URL |
| `RELEASE_CONTROLLER_URL` | `https://amd64.ocp.releases.ci.openshift.org` | Release Controller base URL |
| `SEARCH_CI_URL` | `https://search.ci.openshift.org` | Search.CI base URL |

## Variant Filtering

Tools that query jobs or tests accept variant parameters for filtering:

- `arch` — amd64, arm64, ppc64le, s390x, multi
- `topology` — ha, single, compact, external, microshift
- `platform` — aws, azure, gcp, metal, vsphere, rosa, etc.
- `network` — ovn, sdn, cilium
- `variants` — map of any other dimension (e.g. `{"Installer": "upi", "SecurityMode": "fips"}`)

Use `get_variants` to discover all available dimensions and values.

## Build

```bash
make build          # Build binary to bin/
make test           # Run unit tests
make test-integration  # Run integration tests (requires network)
make lint           # Run go vet
make image          # Build container image
make push           # Push to registry
make clean          # Remove build artifacts
```

Requires Go 1.24+.

## Data Sources

All read-only, no authentication required.

| Source | URL | Purpose |
| ------ | --- | ------- |
| Sippy | sippy.dptools.openshift.org | Jobs, tests, component readiness, regressions, payloads, health |
| Release Controller | {arch}.ocp.releases.ci.openshift.org | Payload tags, acceptance status |
| Search.CI | search.ci.openshift.org | Build log and JUnit failure search |
