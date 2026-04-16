# openshift-ci-mcp

MCP server providing read-only access to OpenShift CI data. Query Sippy, Release Controller, and Search.CI from any MCP-compatible client.

## Tools

### Domain Tools

| Tool | Description |
| ---- | ----------- |
| `get_releases` | List available releases with GA dates |
| `get_release_health` | Install/upgrade/infrastructure success rates, variant summary, payload acceptance |
| `get_variants` | Discover all variant dimensions and their possible values |
| `get_job_report` | Job pass rates with filtering by name, variant, and pass rate thresholds |
| `get_job_runs` | Recent runs of a specific job with results and risk analysis |
| `get_job_run_summary` | Test failures and cluster operator status for a single job run |
| `get_test_report` | Test pass/fail/flake rates with filtering by name, component, and variants |
| `get_test_details` | Test analysis broken down by variant and by job |
| `get_recent_test_failures` | Tests that recently started failing |
| `get_component_readiness` | Component readiness report — the binding release gate |
| `get_regressions` | Active regressions from Component Readiness |
| `get_regression_detail` | Specific regression with linked triages and Jira bugs |
| `get_payload_status` | Payload acceptance/rejection status from the Release Controller |
| `get_payload_diff` | PRs that changed between two payloads |
| `get_payload_test_failures` | Test failures across payload runs |
| `get_pull_request_impact` | Test failures associated with a specific PR |
| `get_pull_requests` | PR reports with filtering by org, repo, and release |
| `search_ci_logs` | Search build logs and JUnit failures across CI |

### Proxy Tools

Raw passthrough to upstream APIs for advanced use cases.

| Tool | Description |
| ---- | ----------- |
| `sippy_api` | Passthrough to any Sippy API endpoint |
| `release_controller_api` | Passthrough to the Release Controller API |
| `search_ci_api` | Passthrough to Search.CI API |

## Usage

### stdio (default) (recommended)

```bash
# Run directly
bin/openshift-ci-mcp

# Run in container
podman run -i --rm quay.io/rh_ee_jeroche/openshift-ci-mcp
```

### HTTP/SSE

```bash
bin/openshift-ci-mcp --transport http --port 8080

# Or in container
podman run -p 8080:8080 quay.io/rh_ee_jeroche/openshift-ci-mcp --transport http --port 8080
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
      "args": ["run", "-i", "--rm", "quay.io/rh_ee_jeroche/openshift-ci-mcp"]
    }
  }
}
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
