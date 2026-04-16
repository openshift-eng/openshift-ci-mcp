---
name: release
description: Build, tag, and publish a new release with container image and GitHub release
disable-model-invocation: true
---

Create a versioned release of openshift-ci-mcp. This pushes artifacts to quay.io and GitHub — confirm the version with the user before proceeding.

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

### 3. Build and push container image

```bash
VERSION=<version-without-v-prefix> make push
```

This builds the image and pushes both `:VERSION` and `:latest` tags to quay.io.

### 4. Build the binary

```bash
VERSION=<version-without-v-prefix> make build
```

### 5. Generate release notes

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

### 6. Create git tag and GitHub release

```bash
git tag -a <version> -m "Release <version>"
git push origin <version>

gh release create <version> \
  --title "<version>" \
  --notes "<release-notes>" \
  bin/openshift-ci-mcp
```

This attaches the binary as a release asset.

### 7. Report

Print a summary:
- Version released
- Container image URL and tags
- GitHub release URL (from `gh release create` output)
