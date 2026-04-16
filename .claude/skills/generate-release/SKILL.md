---
name: generate-release
description: Build, tag, and publish a new release with cross-compiled binaries and GitHub release
disable-model-invocation: true
---

Create a versioned release of openshift-ci-mcp. Pushing the tag triggers a container build on Quay.io automatically. Confirm the version with the user before proceeding.

## Arguments

The user may specify:
- `major`, `minor`, or `patch` (default: `patch`) — which semver component to bump
- An explicit version like `v1.2.3`

## Process

### 1. Determine version

```bash
# Get the latest semver tag (ignore non-semver tags)
git tag -l 'v*' --sort=-v:refname | head -1
```

- If no tags exist, the first release is `v0.1.0`.
- If the user gave an explicit version, use it (add `v` prefix if missing).
- Otherwise bump the specified component from the latest tag:
  - `patch`: v0.1.0 → v0.1.1
  - `minor`: v0.1.0 → v0.2.0
  - `major`: v0.1.0 → v1.0.0

**Tell the user the version and ask for confirmation before continuing.**

### 2. Validate

Run tests and lint before building anything:

```bash
make test
make lint
```

Stop if either fails.

### 3. Cross-compile binaries

```bash
make build-all
```

This builds for linux/amd64, linux/arm64, darwin/amd64, and darwin/arm64. Binaries are written to `bin/` as `openshift-ci-mcp-<os>-<arch>`.

### 4. Generate release notes

Generate release notes from the git log since the previous tag (or all commits if first release):

```bash
# If previous tag exists:
git log <prev-tag>..HEAD --oneline --no-merges

# If first release:
git log --oneline --no-merges
```

Organize the notes into sections based on commit prefixes:
- **Features** — commits starting with `feat:` or `Add`
- **Bug Fixes** — commits starting with `fix:` or `Fix`
- **Other** — everything else

Include a summary line at the top describing the release. Keep it concise.

### 5. Create git tag and GitHub release

```bash
git tag -a <version> -m "Release <version>"
git push origin <version>

gh release create <version> \
  --title "<version>" \
  --notes "<release-notes>" \
  bin/openshift-ci-mcp-*
```

This attaches all cross-compiled binaries as release assets. Pushing the tag also triggers the Quay.io container build.

### 6. Report

Print a summary:
- Version released
- Container image: `quay.io/rh_ee_jeroche/openshift-ci-mcp:<version>` (built automatically by Quay)
- GitHub release URL (from `gh release create` output)
