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
| `get_ci_test_report` | Use to get pass/fail/flake rates for tests with optional filtering |
| `get_component_readiness` | Use to get a report on component readiness for the current dev cycle. Can be slow (30+ seconds) |
| `get_job_report` | Use to get CI job pass rates with filtering and pagination |
| `get_job_run_summary` | Use to get test failures and cluster operator status of a single job run |
| `get_job_runs` | Use to get results, timings, and risk analysis of recent job runs |
| `get_payload_diff` | Use to list pull request changes between payload tags. |
| `get_payload_status` | Use to get recent payload acceptance status from the Release Controller. |
| `get_payload_test_failures` | Use to get test failures for payload job runs |
| `get_pr_impact` | Use to get test failure impact data for a specific, known pull request. Rate-limited to 20 req/hour. |
| `get_recent_test_failures` | Tests that recently started failing, useful for detecting new regressions. |
| `get_regression_detail` | Use when you need details about a regression with triages and Jiras. |
| `get_regressions` | Use to get tests performing significantly worse than the previous release |
| `get_release_health` | Use to get health data for a specific release such as success rates, variant summary, and payload acceptance. |
| `get_release_prs` | Use to get a list of pull requests for a specific release or presubmits |
| `get_releases` | Use to get OpenShift releases with availability and dev cycle dates |
| `get_test_details` | Use to get pass rates broken down by variant and by job. |
| `get_variants` | Use to list variants and their possible values (arch, topology, platform, network, etc.). |
| `search_ci_logs` | Search logs and JUnit output across OpenShift CI for error messages, test names, or patterns. |
<!-- END DOMAIN TOOLS -->

### Proxy Tools

Raw passthrough to upstream APIs for advanced use cases. Disabled by default to reduce schema overhead — enable with `--enable-proxy-tools` or `ENABLE_PROXY_TOOLS=true`.

<!-- BEGIN PROXY TOOLS -->
| Tool | Description |
| ---- | ----------- |
| `release_controller_api` | Raw passthrough to the Release Controller API. |
| `search_ci_api` | Raw passthrough to the Search.CI API. |
| `sippy_api` | Raw passthrough to any Sippy API endpoint. |
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

`go run` (convenient when podman is not available, e.g. running Claude in a pod):

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
| `--enable-proxy-tools` | `false` | Register low-level proxy tools (`sippy_api`, `release_controller_api`, `search_ci_api`) |

### Environment Variables

| Variable | Default | Description |
| -------- | ------- | ----------- |
| `SIPPY_URL` | `https://sippy.dptools.openshift.org` | Sippy base URL |
| `RELEASE_CONTROLLER_URL` | `https://amd64.ocp.releases.ci.openshift.org` | Release Controller base URL |
| `SEARCH_CI_URL` | `https://search.ci.openshift.org` | Search.CI base URL |
| `ENABLE_PROXY_TOOLS` | `false` | Set to `true` to register proxy tools |

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
make build              # Build binary to bin/
make test               # Run unit tests
make test-integration   # Run integration tests (requires network)
make lint               # Run go vet
make smoke              # Build + smoke test against binary
make check              # Build + run mcpchecker eval suite
make image              # Build container image
make push               # Push to registry
make generate           # Regenerate tool tables in README
make clean              # Remove build artifacts
```

Requires Go 1.24+.

## Data Sources

All read-only, no authentication required.

| Source | URL | Purpose |
| ------ | --- | ------- |
| Sippy | sippy.dptools.openshift.org | Jobs, tests, component readiness, regressions, payloads, health |
| Release Controller | {arch}.ocp.releases.ci.openshift.org | Payload tags, acceptance status |
| Search.CI | search.ci.openshift.org | Build log and JUnit failure search |
