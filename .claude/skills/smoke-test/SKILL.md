---
name: smoke-test
description: Run smoke tests against the local binary or container image
disable-model-invocation: true
---

Build the binary with `make build`, then run the smoke test suite:

```bash
python3 tests/smoke_test.py --binary bin/openshift-ci-mcp
```

**Options the user may specify:**

- `--container <image>` — Test against a container image instead of the binary. Do NOT build the binary if the user specifies this.
- `--tools <tool1> <tool2> ...` — Only test specific tools
- `--release <version>` — Test against a specific release (default: 4.18)
- `--timeout <seconds>` — Per-request timeout (default: 30)

Report the results summary. If any tests fail, show the failure details and investigate the root cause.
