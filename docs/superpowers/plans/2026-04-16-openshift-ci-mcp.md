# openshift-ci-mcp Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a Go MCP server providing read-only access to OpenShift CI data (Sippy, Release Controller, Search.CI) for use by any MCP-compatible AI client.

**Architecture:** Thin HTTP client layer wraps upstream APIs. Domain tools provide intent-oriented queries with sensible defaults and variant filtering. Proxy tools offer raw API passthrough. Transport is stdio (default) or HTTP/SSE via CLI flag.

**Tech Stack:** Go 1.24, github.com/mark3labs/mcp-go, containers via podman/docker

**Spec:** `docs/superpowers/specs/2026-04-16-openshift-ci-mcp-design.md`

---

### Task 1: Project Scaffolding

**Files:**
- Create: `go.mod`
- Create: `cmd/openshift-ci-mcp/main.go`
- Create: `pkg/server/server.go`

- [ ] **Step 1: Initialize Go module**

Run:
```bash
cd /home/jeroche/claude_workdir/openshift-ci-mcp
go mod init github.com/jeroche/openshift-ci-mcp
go get github.com/mark3labs/mcp-go@latest
```
Expected: `go.mod` and `go.sum` created

- [ ] **Step 2: Create minimal server package**

```go
// pkg/server/server.go
package server

import (
	"github.com/mark3labs/mcp-go/server"
)

func New() *server.MCPServer {
	s := server.NewMCPServer(
		"openshift-ci-mcp",
		"0.1.0",
		server.WithToolCapabilities(false),
		server.WithRecovery(),
	)
	return s
}
```

- [ ] **Step 3: Create main entry point**

```go
// cmd/openshift-ci-mcp/main.go
package main

import (
	"fmt"
	"os"

	"github.com/mark3labs/mcp-go/server"

	mcpserver "github.com/jeroche/openshift-ci-mcp/pkg/server"
)

func main() {
	s := mcpserver.New()
	if err := server.ServeStdio(s); err != nil {
		fmt.Fprintf(os.Stderr, "server error: %v\n", err)
		os.Exit(1)
	}
}
```

- [ ] **Step 4: Verify it compiles**

Run: `go build ./cmd/openshift-ci-mcp/`
Expected: binary created with no errors

- [ ] **Step 5: Commit**

```bash
git init
git add go.mod go.sum cmd/ pkg/
git commit -m "feat: project scaffolding with minimal MCP server"
```

---

### Task 2: HTTP Client Layer

**Files:**
- Create: `pkg/client/client.go`
- Create: `pkg/client/sippy.go`
- Create: `pkg/client/sippy_test.go`

- [ ] **Step 1: Write the failing test for Sippy client**

```go
// pkg/client/sippy_test.go
package client_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jeroche/openshift-ci-mcp/pkg/client"
)

func TestSippyClient_Get_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/releases" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("release") != "4.18" {
			t.Errorf("unexpected release param: %s", r.URL.Query().Get("release"))
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"releases":["4.18","4.17"]}`))
	}))
	defer ts.Close()

	c := client.NewSippy(ts.URL, ts.Client())
	params := map[string]string{"release": "4.18"}
	data, err := c.Get(context.Background(), "/api/releases", params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(data) != `{"releases":["4.18","4.17"]}` {
		t.Errorf("unexpected response: %s", string(data))
	}
}

func TestSippyClient_Get_HTTPError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error":"not found"}`))
	}))
	defer ts.Close()

	c := client.NewSippy(ts.URL, ts.Client())
	_, err := c.Get(context.Background(), "/api/missing", nil)
	if err == nil {
		t.Fatal("expected error")
	}
	var httpErr *client.HTTPError
	if !errors.As(err, &httpErr) {
		t.Fatalf("expected HTTPError, got %T", err)
	}
	if httpErr.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", httpErr.StatusCode)
	}
}

func TestSippyClient_Get_Timeout(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {}
	}))
	defer ts.Close()

	httpClient := ts.Client()
	httpClient.Timeout = 10 * time.Millisecond
	c := client.NewSippy(ts.URL, httpClient)
	_, err := c.Get(context.Background(), "/api/releases", nil)
	if err == nil {
		t.Fatal("expected timeout error")
	}
}
```

Add missing imports to the test file:

```go
import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jeroche/openshift-ci-mcp/pkg/client"
)
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./pkg/client/...`
Expected: compilation failure — `client` package does not exist

- [ ] **Step 3: Implement the client layer**

```go
// pkg/client/client.go
package client

import (
	"fmt"
)

type HTTPError struct {
	StatusCode int
	Body       []byte
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("HTTP %d: %s", e.StatusCode, string(e.Body))
}
```

```go
// pkg/client/sippy.go
package client

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

type Sippy interface {
	Get(ctx context.Context, path string, params map[string]string) ([]byte, error)
}

type sippyClient struct {
	baseURL    string
	httpClient *http.Client
}

func NewSippy(baseURL string, httpClient *http.Client) Sippy {
	return &sippyClient{baseURL: baseURL, httpClient: httpClient}
}

func (c *sippyClient) Get(ctx context.Context, path string, params map[string]string) ([]byte, error) {
	u, err := url.Parse(c.baseURL + path)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}
	q := u.Query()
	for k, v := range params {
		q.Set(k, v)
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, &HTTPError{StatusCode: resp.StatusCode, Body: body}
	}

	return body, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./pkg/client/... -v`
Expected: all 3 tests pass

- [ ] **Step 5: Commit**

```bash
git add pkg/client/
git commit -m "feat: add Sippy HTTP client with error handling"
```

---

### Task 3: Filter Package

**Files:**
- Create: `pkg/filter/filter.go`
- Create: `pkg/filter/filter_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// pkg/filter/filter_test.go
package filter_test

import (
	"encoding/json"
	"testing"

	"github.com/jeroche/openshift-ci-mcp/pkg/filter"
)

func TestBuild_SingleArch(t *testing.T) {
	result, err := filter.Build(filter.VariantParams{Arch: "arm64"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var f filter.Filter
	if err := json.Unmarshal([]byte(result), &f); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if f.LinkOperator != "and" {
		t.Errorf("expected linkOperator 'and', got %q", f.LinkOperator)
	}
	if len(f.Items) != 1 {
		t.Fatalf("expected 1 filter item, got %d", len(f.Items))
	}
	if f.Items[0].ColumnField != "variants" {
		t.Errorf("expected columnField 'variants', got %q", f.Items[0].ColumnField)
	}
	if f.Items[0].OperatorValue != "contains" {
		t.Errorf("expected operatorValue 'contains', got %q", f.Items[0].OperatorValue)
	}
	if f.Items[0].Value != "Architecture:arm64" {
		t.Errorf("expected value 'Architecture:arm64', got %q", f.Items[0].Value)
	}
}

func TestBuild_MultipleVariants(t *testing.T) {
	result, err := filter.Build(filter.VariantParams{
		Arch:     "amd64",
		Topology: "single",
		Platform: "aws",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var f filter.Filter
	if err := json.Unmarshal([]byte(result), &f); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(f.Items) != 3 {
		t.Fatalf("expected 3 filter items, got %d", len(f.Items))
	}
}

func TestBuild_ExplicitOverridesMap(t *testing.T) {
	result, err := filter.Build(filter.VariantParams{
		Arch:     "arm64",
		Variants: map[string]string{"Architecture": "amd64"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var f filter.Filter
	if err := json.Unmarshal([]byte(result), &f); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(f.Items) != 1 {
		t.Fatalf("expected 1 filter item, got %d", len(f.Items))
	}
	if f.Items[0].Value != "Architecture:arm64" {
		t.Errorf("explicit arch should override map, got %q", f.Items[0].Value)
	}
}

func TestBuild_Empty(t *testing.T) {
	result, err := filter.Build(filter.VariantParams{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "" {
		t.Errorf("expected empty string for no variants, got %q", result)
	}
}

func TestBuild_CustomVariants(t *testing.T) {
	result, err := filter.Build(filter.VariantParams{
		Variants: map[string]string{
			"Installer":    "upi",
			"SecurityMode": "fips",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var f filter.Filter
	if err := json.Unmarshal([]byte(result), &f); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(f.Items) != 2 {
		t.Fatalf("expected 2 filter items, got %d", len(f.Items))
	}
}

func TestMergeInto_AddsFilterToExistingParams(t *testing.T) {
	params := map[string]string{"release": "4.18"}
	err := filter.MergeInto(params, filter.VariantParams{Arch: "arm64"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if params["release"] != "4.18" {
		t.Error("existing params should be preserved")
	}
	if params["filter"] == "" {
		t.Error("filter param should be set")
	}
}

func TestMergeInto_NoOpWhenEmpty(t *testing.T) {
	params := map[string]string{"release": "4.18"}
	err := filter.MergeInto(params, filter.VariantParams{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := params["filter"]; ok {
		t.Error("filter param should not be set when no variants")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./pkg/filter/...`
Expected: compilation failure — `filter` package does not exist

- [ ] **Step 3: Implement the filter package**

```go
// pkg/filter/filter.go
package filter

import (
	"encoding/json"
	"fmt"
	"sort"
)

type Item struct {
	ColumnField   string `json:"columnField"`
	OperatorValue string `json:"operatorValue"`
	Value         string `json:"value"`
}

type Filter struct {
	Items        []Item `json:"items"`
	LinkOperator string `json:"linkOperator"`
}

type VariantParams struct {
	Arch     string
	Topology string
	Platform string
	Network  string
	Variants map[string]string
}

var aliasMap = map[string]string{
	"Arch":     "Architecture",
	"Topology": "Topology",
	"Platform": "Platform",
	"Network":  "Network",
}

func Build(p VariantParams) (string, error) {
	merged := make(map[string]string)
	for k, v := range p.Variants {
		merged[k] = v
	}
	if p.Arch != "" {
		merged["Architecture"] = p.Arch
	}
	if p.Topology != "" {
		merged["Topology"] = p.Topology
	}
	if p.Platform != "" {
		merged["Platform"] = p.Platform
	}
	if p.Network != "" {
		merged["Network"] = p.Network
	}

	if len(merged) == 0 {
		return "", nil
	}

	keys := make([]string, 0, len(merged))
	for k := range merged {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	f := Filter{LinkOperator: "and"}
	for _, k := range keys {
		f.Items = append(f.Items, Item{
			ColumnField:   "variants",
			OperatorValue: "contains",
			Value:         fmt.Sprintf("%s:%s", k, merged[k]),
		})
	}

	b, err := json.Marshal(f)
	if err != nil {
		return "", fmt.Errorf("marshaling filter: %w", err)
	}
	return string(b), nil
}

func MergeInto(params map[string]string, vp VariantParams) error {
	filterJSON, err := Build(vp)
	if err != nil {
		return err
	}
	if filterJSON != "" {
		params["filter"] = filterJSON
	}
	return nil
}

func MergeItemInto(params map[string]string, item Item) {
	existing := params["filter"]
	if existing == "" {
		f := Filter{
			Items:        []Item{item},
			LinkOperator: "and",
		}
		b, _ := json.Marshal(f)
		params["filter"] = string(b)
		return
	}

	var f Filter
	if err := json.Unmarshal([]byte(existing), &f); err != nil {
		f = Filter{LinkOperator: "and"}
	}
	f.Items = append(f.Items, item)
	b, _ := json.Marshal(f)
	params["filter"] = string(b)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./pkg/filter/... -v`
Expected: all 7 tests pass

- [ ] **Step 5: Commit**

```bash
git add pkg/filter/
git commit -m "feat: add variant parameter to Sippy filter JSON conversion"
```

---

### Task 4: Shared Tool Utilities

**Files:**
- Create: `pkg/tools/errors.go`
- Create: `pkg/tools/resolve.go`
- Create: `pkg/tools/resolve_test.go`

- [ ] **Step 1: Write the failing tests for release resolution**

```go
// pkg/tools/resolve_test.go
package tools_test

import (
	"context"
	"testing"

	"github.com/jeroche/openshift-ci-mcp/pkg/tools"
)

type mockSippy struct {
	response []byte
	err      error
}

func (m *mockSippy) Get(ctx context.Context, path string, params map[string]string) ([]byte, error) {
	return m.response, m.err
}

func TestResolveRelease_ExplicitRelease(t *testing.T) {
	mock := &mockSippy{}
	release, err := tools.ResolveRelease(context.Background(), mock, "4.18")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if release != "4.18" {
		t.Errorf("expected '4.18', got %q", release)
	}
}

func TestResolveRelease_DefaultToCurrentDev(t *testing.T) {
	mock := &mockSippy{
		response: []byte(`{
			"releases": ["4.19", "4.18", "4.17"],
			"dates": {
				"4.19": {},
				"4.18": {"ga": "2025-06-15T00:00:00Z"},
				"4.17": {"ga": "2024-12-15T00:00:00Z"}
			}
		}`),
	}
	release, err := tools.ResolveRelease(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if release != "4.19" {
		t.Errorf("expected '4.19' (no GA date = in development), got %q", release)
	}
}

func TestResolveRelease_AllGA_FallbackToLatest(t *testing.T) {
	mock := &mockSippy{
		response: []byte(`{
			"releases": ["4.18", "4.17"],
			"dates": {
				"4.18": {"ga": "2025-06-15T00:00:00Z"},
				"4.17": {"ga": "2024-12-15T00:00:00Z"}
			}
		}`),
	}
	release, err := tools.ResolveRelease(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if release != "4.18" {
		t.Errorf("expected '4.18' (latest GA), got %q", release)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./pkg/tools/...`
Expected: compilation failure — package does not exist

- [ ] **Step 3: Implement error helpers and release resolution**

```go
// pkg/tools/errors.go
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
```

```go
// pkg/tools/resolve.go
package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jeroche/openshift-ci-mcp/pkg/client"
)

type releasesResponse struct {
	Releases []string                   `json:"releases"`
	Dates    map[string]releaseDateInfo `json:"dates"`
}

type releaseDateInfo struct {
	GA *string `json:"ga,omitempty"`
}

func ResolveRelease(ctx context.Context, sippy client.Sippy, release string) (string, error) {
	if release != "" {
		return release, nil
	}

	data, err := sippy.Get(ctx, "/api/releases", nil)
	if err != nil {
		return "", fmt.Errorf("fetching releases: %w", err)
	}

	var resp releasesResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return "", fmt.Errorf("parsing releases: %w", err)
	}

	if len(resp.Releases) == 0 {
		return "", fmt.Errorf("no releases available")
	}

	for _, r := range resp.Releases {
		dates, ok := resp.Dates[r]
		if !ok || dates.GA == nil {
			return r, nil
		}
	}

	return resp.Releases[0], nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./pkg/tools/... -v`
Expected: all 3 tests pass

- [ ] **Step 5: Commit**

```bash
git add pkg/tools/errors.go pkg/tools/resolve.go pkg/tools/resolve_test.go
git commit -m "feat: add shared tool utilities — error helpers and release resolution"
```

---

### Task 5: Release & Health Domain Tools

**Files:**
- Create: `pkg/tools/domain/releases.go`
- Create: `pkg/tools/domain/releases_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// pkg/tools/domain/releases_test.go
package domain_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/jeroche/openshift-ci-mcp/pkg/tools/domain"
)

type mockSippy struct {
	responses map[string][]byte
	calls     []string
}

func (m *mockSippy) Get(ctx context.Context, path string, params map[string]string) ([]byte, error) {
	m.calls = append(m.calls, path)
	if resp, ok := m.responses[path]; ok {
		return resp, nil
	}
	return nil, &client.HTTPError{StatusCode: 404, Body: []byte("not found")}
}

func newMockSippy(responses map[string][]byte) *mockSippy {
	return &mockSippy{responses: responses}
}

func TestGetReleases(t *testing.T) {
	mock := newMockSippy(map[string][]byte{
		"/api/releases": []byte(`{"releases":["4.19","4.18"],"dates":{"4.19":{},"4.18":{"ga":"2025-06-15T00:00:00Z"}}}`),
	})

	handler := domain.GetReleasesHandler(mock)
	result, err := handler(context.Background(), mcp.CallToolRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}

	var content []mcp.Content
	contentJSON, _ := json.Marshal(result.Content)
	json.Unmarshal(contentJSON, &content)
	if len(result.Content) == 0 {
		t.Fatal("expected content in result")
	}
}

func TestGetReleaseHealth(t *testing.T) {
	mock := newMockSippy(map[string][]byte{
		"/api/health":          []byte(`{"indicators":{},"last_updated":"2025-01-01T00:00:00Z"}`),
		"/api/releases/health": []byte(`[{"release_tag":"4.18.0","last_phase":"Accepted"}]`),
	})

	handler := domain.GetReleaseHealthHandler(mock)
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"release": "4.18"}
	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
}
```

Add to imports:

```go
import (
	"github.com/jeroche/openshift-ci-mcp/pkg/client"
)
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./pkg/tools/domain/...`
Expected: compilation failure — `domain` package does not exist

- [ ] **Step 3: Implement release and health tools**

```go
// pkg/tools/domain/releases.go
package domain

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/jeroche/openshift-ci-mcp/pkg/client"
	"github.com/jeroche/openshift-ci-mcp/pkg/tools"
)

func RegisterReleaseTools(s *server.MCPServer, sippy client.Sippy) {
	s.AddTool(
		mcp.NewTool("get_releases",
			mcp.WithDescription("List available OpenShift releases with GA dates and development start dates"),
		),
		GetReleasesHandler(sippy),
	)

	s.AddTool(
		mcp.NewTool("get_release_health",
			mcp.WithDescription("Overall health of a release — install/upgrade/infrastructure success rates, variant summary, and payload acceptance statistics"),
			mcp.WithString("release",
				mcp.Description("Release version (e.g. '4.18'). Defaults to current development release if omitted."),
			),
		),
		GetReleaseHealthHandler(sippy),
	)
}

func GetReleasesHandler(sippy client.Sippy) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		data, err := sippy.Get(ctx, "/api/releases", nil)
		if err != nil {
			return tools.ToolError(err)
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}

func GetReleaseHealthHandler(sippy client.Sippy) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		release, err := tools.ResolveRelease(ctx, sippy, req.GetString("release", ""))
		if err != nil {
			return tools.ToolError(err)
		}

		params := map[string]string{"release": release}

		healthData, err := sippy.Get(ctx, "/api/health", params)
		if err != nil {
			return tools.ToolError(err)
		}

		releaseHealthData, err := sippy.Get(ctx, "/api/releases/health", params)
		if err != nil {
			return tools.ToolError(err)
		}

		combined := fmt.Sprintf(`{"health":%s,"release_health":%s}`, string(healthData), string(releaseHealthData))
		return mcp.NewToolResultText(combined), nil
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./pkg/tools/domain/... -v`
Expected: all tests pass

- [ ] **Step 5: Wire tools into the server**

Update `pkg/server/server.go`:

```go
package server

import (
	"net/http"
	"time"

	"github.com/mark3labs/mcp-go/server"

	"github.com/jeroche/openshift-ci-mcp/pkg/client"
	"github.com/jeroche/openshift-ci-mcp/pkg/tools/domain"
)

type Config struct {
	SippyURL             string
	ReleaseControllerURL string
	SearchCIURL          string
	Timeout              time.Duration
}

func DefaultConfig() Config {
	return Config{
		SippyURL:             "https://sippy.dptools.openshift.org",
		ReleaseControllerURL: "https://%s.ocp.releases.ci.openshift.org",
		SearchCIURL:          "https://search.ci.openshift.org",
		Timeout:              30 * time.Second,
	}
}

func New(cfg Config) *server.MCPServer {
	httpClient := &http.Client{Timeout: cfg.Timeout}

	sippy := client.NewSippy(cfg.SippyURL, httpClient)

	s := server.NewMCPServer(
		"openshift-ci-mcp",
		"0.1.0",
		server.WithToolCapabilities(false),
		server.WithRecovery(),
	)

	domain.RegisterReleaseTools(s, sippy)

	return s
}
```

Update `cmd/openshift-ci-mcp/main.go`:

```go
package main

import (
	"fmt"
	"os"

	"github.com/mark3labs/mcp-go/server"

	mcpserver "github.com/jeroche/openshift-ci-mcp/pkg/server"
)

func main() {
	cfg := mcpserver.DefaultConfig()
	if v := os.Getenv("SIPPY_URL"); v != "" {
		cfg.SippyURL = v
	}

	s := mcpserver.New(cfg)
	if err := server.ServeStdio(s); err != nil {
		fmt.Fprintf(os.Stderr, "server error: %v\n", err)
		os.Exit(1)
	}
}
```

- [ ] **Step 6: Verify it compiles**

Run: `go build ./cmd/openshift-ci-mcp/`
Expected: compiles successfully

- [ ] **Step 7: Commit**

```bash
git add pkg/tools/domain/releases.go pkg/tools/domain/releases_test.go pkg/server/server.go cmd/openshift-ci-mcp/main.go
git commit -m "feat: add get_releases and get_release_health tools"
```

---

### Task 6: Variant Discovery Tool

**Files:**
- Create: `pkg/tools/domain/variants.go`
- Create: `pkg/tools/domain/variants_test.go`

- [ ] **Step 1: Write the failing test**

```go
// pkg/tools/domain/variants_test.go
package domain_test

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/jeroche/openshift-ci-mcp/pkg/tools/domain"
)

func TestGetVariants(t *testing.T) {
	mock := newMockSippy(map[string][]byte{
		"/api/job_variants": []byte(`{"variants":{"Architecture":["amd64","arm64"],"Topology":["ha","single"]}}`),
	})

	handler := domain.GetVariantsHandler(mock)
	result, err := handler(context.Background(), mcp.CallToolRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./pkg/tools/domain/... -run TestGetVariants`
Expected: compilation failure

- [ ] **Step 3: Implement**

```go
// pkg/tools/domain/variants.go
package domain

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/jeroche/openshift-ci-mcp/pkg/client"
	"github.com/jeroche/openshift-ci-mcp/pkg/tools"
)

func RegisterVariantTools(s *server.MCPServer, sippy client.Sippy) {
	s.AddTool(
		mcp.NewTool("get_variants",
			mcp.WithDescription("List all variant dimensions and their possible values. Use this to discover valid values for arch, topology, platform, network, and other variant filters."),
		),
		GetVariantsHandler(sippy),
	)
}

func GetVariantsHandler(sippy client.Sippy) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		data, err := sippy.Get(ctx, "/api/job_variants", nil)
		if err != nil {
			return tools.ToolError(err)
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./pkg/tools/domain/... -run TestGetVariants -v`
Expected: PASS

- [ ] **Step 5: Register in server and commit**

Add to `pkg/server/server.go` in `New()`:

```go
domain.RegisterVariantTools(s, sippy)
```

```bash
git add pkg/tools/domain/variants.go pkg/tools/domain/variants_test.go pkg/server/server.go
git commit -m "feat: add get_variants discovery tool"
```

---

### Task 7: Job Domain Tools

**Files:**
- Create: `pkg/tools/domain/jobs.go`
- Create: `pkg/tools/domain/jobs_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// pkg/tools/domain/jobs_test.go
package domain_test

import (
	"context"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/jeroche/openshift-ci-mcp/pkg/tools/domain"
)

func TestGetJobReport(t *testing.T) {
	mock := newMockSippy(map[string][]byte{
		"/api/jobs": []byte(`{"rows":[{"name":"periodic-ci-e2e-aws","current_pass_percentage":95.5}],"total_rows":1}`),
	})

	handler := domain.GetJobReportHandler(mock)
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"release": "4.18",
		"arch":    "amd64",
	}
	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
}

func TestGetJobReport_VariantFiltering(t *testing.T) {
	var capturedPath string
	var capturedParams map[string]string
	mock := &capturingSippy{
		response: []byte(`{"rows":[],"total_rows":0}`),
		onGet: func(path string, params map[string]string) {
			capturedPath = path
			capturedParams = params
		},
	}

	handler := domain.GetJobReportHandler(mock)
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"release":  "4.18",
		"arch":     "arm64",
		"topology": "single",
	}
	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
	if capturedPath != "/api/jobs" {
		t.Errorf("expected /api/jobs, got %q", capturedPath)
	}
	if !strings.Contains(capturedParams["filter"], "Architecture:arm64") {
		t.Error("expected filter to contain Architecture:arm64")
	}
	if !strings.Contains(capturedParams["filter"], "Topology:single") {
		t.Error("expected filter to contain Topology:single")
	}
}

func TestGetJobRuns(t *testing.T) {
	mock := newMockSippy(map[string][]byte{
		"/api/jobs/runs": []byte(`{"rows":[{"id":1,"name":"test-job"}],"total_rows":1}`),
	})

	handler := domain.GetJobRunsHandler(mock)
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"release":  "4.18",
		"job_name": "periodic-ci-e2e-aws",
	}
	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
}

func TestGetJobRunSummary(t *testing.T) {
	mock := newMockSippy(map[string][]byte{
		"/api/job/run/summary": []byte(`{"id":12345,"name":"e2e-aws","succeeded":true}`),
	})

	handler := domain.GetJobRunSummaryHandler(mock)
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"prow_job_run_id": "12345",
	}
	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
}

type capturingSippy struct {
	response []byte
	onGet    func(path string, params map[string]string)
}

func (m *capturingSippy) Get(ctx context.Context, path string, params map[string]string) ([]byte, error) {
	if m.onGet != nil {
		m.onGet(path, params)
	}
	return m.response, nil
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./pkg/tools/domain/... -run TestGetJob`
Expected: compilation failure

- [ ] **Step 3: Implement job tools**

```go
// pkg/tools/domain/jobs.go
package domain

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/jeroche/openshift-ci-mcp/pkg/client"
	"github.com/jeroche/openshift-ci-mcp/pkg/filter"
	"github.com/jeroche/openshift-ci-mcp/pkg/tools"
)

func RegisterJobTools(s *server.MCPServer, sippy client.Sippy) {
	s.AddTool(
		mcp.NewTool("get_job_report",
			mcp.WithDescription("Get job pass rates with filtering by name, variant dimensions, and pass rate thresholds. Returns paginated results."),
			mcp.WithString("release", mcp.Description("Release version (e.g. '4.18'). Defaults to current dev release.")),
			mcp.WithString("job_name", mcp.Description("Filter jobs by name substring")),
			mcp.WithString("arch", mcp.Description("Filter by architecture: amd64, arm64, ppc64le, s390x, multi")),
			mcp.WithString("topology", mcp.Description("Filter by topology: ha, single, compact, external, microshift")),
			mcp.WithString("platform", mcp.Description("Filter by platform: aws, azure, gcp, metal, vsphere, rosa, etc.")),
			mcp.WithString("network", mcp.Description("Filter by network: ovn, sdn, cilium")),
			mcp.WithNumber("min_pass_rate", mcp.Description("Minimum pass rate percentage (e.g. 0)")),
			mcp.WithNumber("max_pass_rate", mcp.Description("Maximum pass rate percentage (e.g. 80)")),
			mcp.WithNumber("limit", mcp.Description("Max results per page (default 25)"), mcp.DefaultNumber(25)),
			mcp.WithNumber("page", mcp.Description("Page number (default 1)"), mcp.DefaultNumber(1)),
		),
		GetJobReportHandler(sippy),
	)

	s.AddTool(
		mcp.NewTool("get_job_runs",
			mcp.WithDescription("Get recent runs of a specific job with results and risk analysis"),
			mcp.WithString("release", mcp.Description("Release version. Defaults to current dev release.")),
			mcp.WithString("job_name", mcp.Required(), mcp.Description("Exact job name")),
			mcp.WithNumber("limit", mcp.Description("Max results (default 10)"), mcp.DefaultNumber(10)),
		),
		GetJobRunsHandler(sippy),
	)

	s.AddTool(
		mcp.NewTool("get_job_run_summary",
			mcp.WithDescription("Detailed summary of a single job run — test failures, cluster operator status"),
			mcp.WithString("prow_job_run_id", mcp.Required(), mcp.Description("Prow job run ID")),
		),
		GetJobRunSummaryHandler(sippy),
	)
}

func GetJobReportHandler(sippy client.Sippy) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		release, err := tools.ResolveRelease(ctx, sippy, req.GetString("release", ""))
		if err != nil {
			return tools.ToolError(err)
		}

		params := map[string]string{
			"release":   release,
			"sortField": "current_pass_percentage",
			"sort":      "asc",
			"perPage":   fmt.Sprintf("%d", req.GetInt("limit", 25)),
			"page":      fmt.Sprintf("%d", req.GetInt("page", 1)),
		}

		vp := extractVariantParams(req)
		if err := filter.MergeInto(params, vp); err != nil {
			return tools.ToolError(err)
		}

		if name := req.GetString("job_name", ""); name != "" {
			filter.MergeItemInto(params, filter.Item{
				ColumnField:   "name",
				OperatorValue: "contains",
				Value:         name,
			})
		}
		if minRate := req.GetFloat("min_pass_rate", -1); minRate >= 0 {
			filter.MergeItemInto(params, filter.Item{
				ColumnField:   "current_pass_percentage",
				OperatorValue: ">=",
				Value:         fmt.Sprintf("%g", minRate),
			})
		}
		if maxRate := req.GetFloat("max_pass_rate", -1); maxRate >= 0 {
			filter.MergeItemInto(params, filter.Item{
				ColumnField:   "current_pass_percentage",
				OperatorValue: "<=",
				Value:         fmt.Sprintf("%g", maxRate),
			})
		}

		data, err := sippy.Get(ctx, "/api/jobs", params)
		if err != nil {
			return tools.ToolError(err)
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}

func GetJobRunsHandler(sippy client.Sippy) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		release, err := tools.ResolveRelease(ctx, sippy, req.GetString("release", ""))
		if err != nil {
			return tools.ToolError(err)
		}

		jobName, err := req.RequireString("job_name")
		if err != nil {
			return tools.InvalidParam("job_name", "required")
		}

		params := map[string]string{
			"release": release,
			"perPage": fmt.Sprintf("%d", req.GetInt("limit", 10)),
			"filter":  fmt.Sprintf(`{"items":[{"columnField":"name","operatorValue":"equals","value":%q}],"linkOperator":"and"}`, jobName),
		}

		data, err := sippy.Get(ctx, "/api/jobs/runs", params)
		if err != nil {
			return tools.ToolError(err)
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}

func GetJobRunSummaryHandler(sippy client.Sippy) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		runID, err := req.RequireString("prow_job_run_id")
		if err != nil {
			return tools.InvalidParam("prow_job_run_id", "required")
		}

		params := map[string]string{"prow_job_run_id": runID}
		data, err := sippy.Get(ctx, "/api/job/run/summary", params)
		if err != nil {
			return tools.ToolError(err)
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}

func extractVariantParams(req mcp.CallToolRequest) filter.VariantParams {
	vp := filter.VariantParams{
		Arch:     req.GetString("arch", ""),
		Topology: req.GetString("topology", ""),
		Platform: req.GetString("platform", ""),
		Network:  req.GetString("network", ""),
	}
	args := req.GetArguments()
	if variants, ok := args["variants"].(map[string]any); ok {
		vp.Variants = make(map[string]string)
		for k, v := range variants {
			if s, ok := v.(string); ok {
				vp.Variants[k] = s
			}
		}
	}
	return vp
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./pkg/tools/domain/... -run TestGetJob -v`
Expected: all 4 tests pass

- [ ] **Step 5: Register in server and commit**

Add to `pkg/server/server.go` in `New()`:

```go
domain.RegisterJobTools(s, sippy)
```

```bash
git add pkg/tools/domain/jobs.go pkg/tools/domain/jobs_test.go pkg/server/server.go
git commit -m "feat: add job domain tools — get_job_report, get_job_runs, get_job_run_summary"
```

---

### Task 8: Test Domain Tools

**Files:**
- Create: `pkg/tools/domain/tests.go`
- Create: `pkg/tools/domain/tests_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// pkg/tools/domain/tests_test.go
package domain_test

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/jeroche/openshift-ci-mcp/pkg/tools/domain"
)

func TestGetTestReport(t *testing.T) {
	mock := newMockSippy(map[string][]byte{
		"/api/tests": []byte(`[{"name":"test-1","current_pass_percentage":99.0}]`),
	})

	handler := domain.GetTestReportHandler(mock)
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"release": "4.18"}
	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
}

func TestGetTestDetails(t *testing.T) {
	mock := newMockSippy(map[string][]byte{
		"/api/tests/details": []byte(`{"name":"test-1","variants":[{"name":"aws","pass_percentage":98.0}]}`),
	})

	handler := domain.GetTestDetailsHandler(mock)
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"release":   "4.18",
		"test_name": "[sig-network] pods should work",
	}
	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
}

func TestGetRecentTestFailures(t *testing.T) {
	mock := newMockSippy(map[string][]byte{
		"/api/tests/recent_failures": []byte(`[{"name":"test-1","current_pass_percentage":50.0}]`),
	})

	handler := domain.GetRecentTestFailuresHandler(mock)
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"release": "4.18"}
	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./pkg/tools/domain/... -run TestGetTest`
Expected: compilation failure

- [ ] **Step 3: Implement test tools**

```go
// pkg/tools/domain/tests.go
package domain

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/jeroche/openshift-ci-mcp/pkg/client"
	"github.com/jeroche/openshift-ci-mcp/pkg/filter"
	"github.com/jeroche/openshift-ci-mcp/pkg/tools"
)

func RegisterTestTools(s *server.MCPServer, sippy client.Sippy) {
	s.AddTool(
		mcp.NewTool("get_test_report",
			mcp.WithDescription("Get test pass/fail/flake rates with filtering by name, component, and variant dimensions"),
			mcp.WithString("release", mcp.Description("Release version. Defaults to current dev release.")),
			mcp.WithString("test_name", mcp.Description("Filter tests by name substring")),
			mcp.WithString("component", mcp.Description("Filter by Jira component")),
			mcp.WithString("arch", mcp.Description("Filter by architecture")),
			mcp.WithString("topology", mcp.Description("Filter by topology")),
			mcp.WithString("platform", mcp.Description("Filter by platform")),
			mcp.WithString("network", mcp.Description("Filter by network")),
			mcp.WithNumber("limit", mcp.Description("Max results per page (default 25)"), mcp.DefaultNumber(25)),
			mcp.WithNumber("page", mcp.Description("Page number (default 1)"), mcp.DefaultNumber(1)),
		),
		GetTestReportHandler(sippy),
	)

	s.AddTool(
		mcp.NewTool("get_test_details",
			mcp.WithDescription("Detailed test analysis — pass rates broken down by variant and by job"),
			mcp.WithString("release", mcp.Description("Release version. Defaults to current dev release.")),
			mcp.WithString("test_name", mcp.Required(), mcp.Description("Exact test name")),
		),
		GetTestDetailsHandler(sippy),
	)

	s.AddTool(
		mcp.NewTool("get_recent_test_failures",
			mcp.WithDescription("Tests that have recently started failing — useful for detecting new regressions"),
			mcp.WithString("release", mcp.Description("Release version. Defaults to current dev release.")),
		),
		GetRecentTestFailuresHandler(sippy),
	)
}

func GetTestReportHandler(sippy client.Sippy) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		release, err := tools.ResolveRelease(ctx, sippy, req.GetString("release", ""))
		if err != nil {
			return tools.ToolError(err)
		}

		params := map[string]string{
			"release": release,
			"perPage": fmt.Sprintf("%d", req.GetInt("limit", 25)),
			"page":    fmt.Sprintf("%d", req.GetInt("page", 1)),
		}

		if name := req.GetString("test_name", ""); name != "" {
			params["filter"] = fmt.Sprintf(`{"items":[{"columnField":"name","operatorValue":"contains","value":%q}],"linkOperator":"and"}`, name)
		}
		if component := req.GetString("component", ""); component != "" {
			params["filter"] = fmt.Sprintf(`{"items":[{"columnField":"jira_component","operatorValue":"equals","value":%q}],"linkOperator":"and"}`, component)
		}

		vp := extractVariantParams(req)
		if err := filter.MergeInto(params, vp); err != nil {
			return tools.ToolError(err)
		}

		data, err := sippy.Get(ctx, "/api/tests", params)
		if err != nil {
			return tools.ToolError(err)
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}

func GetTestDetailsHandler(sippy client.Sippy) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		release, err := tools.ResolveRelease(ctx, sippy, req.GetString("release", ""))
		if err != nil {
			return tools.ToolError(err)
		}

		testName, err := req.RequireString("test_name")
		if err != nil {
			return tools.InvalidParam("test_name", "required")
		}

		params := map[string]string{
			"release": release,
			"test":    testName,
		}

		data, err := sippy.Get(ctx, "/api/tests/details", params)
		if err != nil {
			return tools.ToolError(err)
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}

func GetRecentTestFailuresHandler(sippy client.Sippy) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		release, err := tools.ResolveRelease(ctx, sippy, req.GetString("release", ""))
		if err != nil {
			return tools.ToolError(err)
		}

		params := map[string]string{"release": release}
		data, err := sippy.Get(ctx, "/api/tests/recent_failures", params)
		if err != nil {
			return tools.ToolError(err)
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./pkg/tools/domain/... -run TestGetTest -v`
Expected: all 3 tests pass

- [ ] **Step 5: Register in server and commit**

Add to `pkg/server/server.go` in `New()`:

```go
domain.RegisterTestTools(s, sippy)
```

```bash
git add pkg/tools/domain/tests.go pkg/tools/domain/tests_test.go pkg/server/server.go
git commit -m "feat: add test domain tools — get_test_report, get_test_details, get_recent_test_failures"
```

---

### Task 9: Component Readiness Domain Tools

**Files:**
- Create: `pkg/tools/domain/component.go`
- Create: `pkg/tools/domain/component_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// pkg/tools/domain/component_test.go
package domain_test

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/jeroche/openshift-ci-mcp/pkg/tools/domain"
)

func TestGetComponentReadiness(t *testing.T) {
	mock := newMockSippy(map[string][]byte{
		"/api/component_readiness":       []byte(`{"rows":[{"component":"etcd"}]}`),
		"/api/component_readiness/views": []byte(`[{"name":"main","params":{}}]`),
	})

	handler := domain.GetComponentReadinessHandler(mock)
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"release": "4.18"}
	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
}

func TestGetRegressions(t *testing.T) {
	mock := newMockSippy(map[string][]byte{
		"/api/component_readiness/regressions": []byte(`[{"id":1,"test_name":"test-1","component":"etcd"}]`),
	})

	handler := domain.GetRegressionsHandler(mock)
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"release": "4.18"}
	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
}

func TestGetRegressionDetail(t *testing.T) {
	mock := newMockSippy(map[string][]byte{
		"/api/component_readiness/regressions/42":         []byte(`{"id":42,"test_name":"test-1"}`),
		"/api/component_readiness/regressions/42/matches": []byte(`[{"id":1,"url":"https://issues.redhat.com/browse/OCPBUGS-123"}]`),
	})

	handler := domain.GetRegressionDetailHandler(mock)
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"regression_id": "42"}
	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./pkg/tools/domain/... -run TestGetComponent -v; go test ./pkg/tools/domain/... -run TestGetRegression -v`
Expected: compilation failure

- [ ] **Step 3: Implement component readiness tools**

```go
// pkg/tools/domain/component.go
package domain

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/jeroche/openshift-ci-mcp/pkg/client"
	"github.com/jeroche/openshift-ci-mcp/pkg/tools"
)

func RegisterComponentTools(s *server.MCPServer, sippy client.Sippy) {
	s.AddTool(
		mcp.NewTool("get_component_readiness",
			mcp.WithDescription("Component readiness report — the binding release gate. Shows statistical analysis comparing current release behavior against the previous stable release."),
			mcp.WithString("release", mcp.Description("Release version. Defaults to current dev release.")),
			mcp.WithString("view", mcp.Description("Predefined view name (default: server default). Use get_variants or check Sippy for available views.")),
		),
		GetComponentReadinessHandler(sippy),
	)

	s.AddTool(
		mcp.NewTool("get_regressions",
			mcp.WithDescription("Active regressions from Component Readiness — tests that are performing significantly worse than the previous release"),
			mcp.WithString("release", mcp.Description("Release version. Defaults to current dev release.")),
			mcp.WithString("view", mcp.Description("Component Readiness view name")),
			mcp.WithString("component", mcp.Description("Filter by component name")),
		),
		GetRegressionsHandler(sippy),
	)

	s.AddTool(
		mcp.NewTool("get_regression_detail",
			mcp.WithDescription("Details of a specific regression including linked triages and Jira bugs"),
			mcp.WithString("regression_id", mcp.Required(), mcp.Description("Regression ID")),
		),
		GetRegressionDetailHandler(sippy),
	)
}

func GetComponentReadinessHandler(sippy client.Sippy) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		release, err := tools.ResolveRelease(ctx, sippy, req.GetString("release", ""))
		if err != nil {
			return tools.ToolError(err)
		}

		view := req.GetString("view", "")

		if view == "" {
			viewsData, err := sippy.Get(ctx, "/api/component_readiness/views", map[string]string{"release": release})
			if err == nil {
				var views []struct{ Name string `json:"name"` }
				if json.Unmarshal(viewsData, &views) == nil && len(views) > 0 {
					view = views[0].Name
				}
			}
		}

		params := map[string]string{"release": release}
		if view != "" {
			params["view"] = view
		}

		data, err := sippy.Get(ctx, "/api/component_readiness", params)
		if err != nil {
			return tools.ToolError(err)
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}

func GetRegressionsHandler(sippy client.Sippy) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		release, err := tools.ResolveRelease(ctx, sippy, req.GetString("release", ""))
		if err != nil {
			return tools.ToolError(err)
		}

		params := map[string]string{"release": release}
		if view := req.GetString("view", ""); view != "" {
			params["view"] = view
		}
		if component := req.GetString("component", ""); component != "" {
			params["component"] = component
		}

		data, err := sippy.Get(ctx, "/api/component_readiness/regressions", params)
		if err != nil {
			return tools.ToolError(err)
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}

func GetRegressionDetailHandler(sippy client.Sippy) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id, err := req.RequireString("regression_id")
		if err != nil {
			return tools.InvalidParam("regression_id", "required")
		}

		regressionData, err := sippy.Get(ctx, fmt.Sprintf("/api/component_readiness/regressions/%s", id), nil)
		if err != nil {
			return tools.ToolError(err)
		}

		matchesData, err := sippy.Get(ctx, fmt.Sprintf("/api/component_readiness/regressions/%s/matches", id), nil)
		if err != nil {
			return tools.ToolError(err)
		}

		combined := fmt.Sprintf(`{"regression":%s,"matching_triages":%s}`, string(regressionData), string(matchesData))
		return mcp.NewToolResultText(combined), nil
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./pkg/tools/domain/... -run "TestGetComponent|TestGetRegression" -v`
Expected: all 3 tests pass

- [ ] **Step 5: Register in server and commit**

Add to `pkg/server/server.go` in `New()`:

```go
domain.RegisterComponentTools(s, sippy)
```

```bash
git add pkg/tools/domain/component.go pkg/tools/domain/component_test.go pkg/server/server.go
git commit -m "feat: add component readiness tools — get_component_readiness, get_regressions, get_regression_detail"
```

---

### Task 10: Release Controller Client

**Files:**
- Create: `pkg/client/releasecontroller.go`
- Create: `pkg/client/releasecontroller_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// pkg/client/releasecontroller_test.go
package client_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jeroche/openshift-ci-mcp/pkg/client"
)

func TestReleaseControllerClient_Get(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/releasestream/4.18.0-0.nightly/tags" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"name":"4.18.0-0.nightly","tags":[{"name":"4.18.0-0.nightly-2025-01-01","phase":"Accepted"}]}`))
	}))
	defer ts.Close()

	c := client.NewReleaseController(ts.URL, ts.Client())
	data, err := c.Get(context.Background(), "/api/v1/releasestream/4.18.0-0.nightly/tags", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("expected non-empty response")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./pkg/client/... -run TestReleaseController`
Expected: compilation failure

- [ ] **Step 3: Implement Release Controller client**

```go
// pkg/client/releasecontroller.go
package client

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

type ReleaseController interface {
	Get(ctx context.Context, path string, params map[string]string) ([]byte, error)
	GetForArch(ctx context.Context, arch, path string, params map[string]string) ([]byte, error)
}

type releaseControllerClient struct {
	baseURL    string
	urlPattern string
	httpClient *http.Client
}

func NewReleaseController(baseURL string, httpClient *http.Client) ReleaseController {
	return &releaseControllerClient{
		baseURL:    baseURL,
		urlPattern: "https://%s.ocp.releases.ci.openshift.org",
		httpClient: httpClient,
	}
}

func NewReleaseControllerWithPattern(urlPattern string, httpClient *http.Client) ReleaseController {
	return &releaseControllerClient{
		urlPattern: urlPattern,
		httpClient: httpClient,
	}
}

func (c *releaseControllerClient) Get(ctx context.Context, path string, params map[string]string) ([]byte, error) {
	return c.doGet(ctx, c.baseURL+path, params)
}

func (c *releaseControllerClient) GetForArch(ctx context.Context, arch, path string, params map[string]string) ([]byte, error) {
	base := fmt.Sprintf(c.urlPattern, arch)
	return c.doGet(ctx, base+path, params)
}

func (c *releaseControllerClient) doGet(ctx context.Context, rawURL string, params map[string]string) ([]byte, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}
	q := u.Query()
	for k, v := range params {
		q.Set(k, v)
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, &HTTPError{StatusCode: resp.StatusCode, Body: body}
	}

	return body, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./pkg/client/... -run TestReleaseController -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add pkg/client/releasecontroller.go pkg/client/releasecontroller_test.go
git commit -m "feat: add Release Controller HTTP client"
```

---

### Task 11: Payload Domain Tools

**Files:**
- Create: `pkg/tools/domain/payloads.go`
- Create: `pkg/tools/domain/payloads_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// pkg/tools/domain/payloads_test.go
package domain_test

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/jeroche/openshift-ci-mcp/pkg/tools/domain"
)

type mockReleaseController struct {
	responses map[string][]byte
}

func (m *mockReleaseController) Get(ctx context.Context, path string, params map[string]string) ([]byte, error) {
	if resp, ok := m.responses[path]; ok {
		return resp, nil
	}
	return nil, &client.HTTPError{StatusCode: 404, Body: []byte("not found")}
}

func (m *mockReleaseController) GetForArch(ctx context.Context, arch, path string, params map[string]string) ([]byte, error) {
	key := arch + ":" + path
	if resp, ok := m.responses[key]; ok {
		return resp, nil
	}
	return m.Get(ctx, path, params)
}

func TestGetPayloadStatus(t *testing.T) {
	rc := &mockReleaseController{
		responses: map[string][]byte{
			"amd64:/api/v1/releasestream/4.18.0-0.nightly/tags": []byte(`{"name":"4.18.0-0.nightly","tags":[{"name":"4.18.0-0.nightly-2025-01-01","phase":"Accepted"}]}`),
		},
	}

	handler := domain.GetPayloadStatusHandler(rc)
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"release": "4.18",
		"stream":  "nightly",
	}
	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
}

func TestGetPayloadDiff(t *testing.T) {
	mock := newMockSippy(map[string][]byte{
		"/api/payloads/diff": []byte(`[{"name":"etcd","url":"https://github.com/openshift/etcd/pull/123","pull_request_id":"123"}]`),
	})

	handler := domain.GetPayloadDiffHandler(mock)
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"release":  "4.18",
		"from_tag": "4.18.0-0.nightly-2025-01-01-000000",
		"to_tag":   "4.18.0-0.nightly-2025-01-02-000000",
	}
	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
}

func TestGetPayloadTestFailures(t *testing.T) {
	mock := newMockSippy(map[string][]byte{
		"/api/payloads/test_failures": []byte(`[{"test_name":"test-1","count":3}]`),
	})

	handler := domain.GetPayloadTestFailuresHandler(mock)
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"release": "4.18",
	}
	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
}
```

Add the import for `client` package to the test file.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./pkg/tools/domain/... -run TestGetPayload`
Expected: compilation failure

- [ ] **Step 3: Implement payload tools**

```go
// pkg/tools/domain/payloads.go
package domain

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/jeroche/openshift-ci-mcp/pkg/client"
	"github.com/jeroche/openshift-ci-mcp/pkg/tools"
)

func RegisterPayloadTools(s *server.MCPServer, sippy client.Sippy, rc client.ReleaseController) {
	s.AddTool(
		mcp.NewTool("get_payload_status",
			mcp.WithDescription("Recent payload acceptance/rejection status from the Release Controller. Shows payload tags with their phase (Accepted/Rejected/Ready)."),
			mcp.WithString("release", mcp.Required(), mcp.Description("Release version (e.g. '4.18')")),
			mcp.WithString("arch", mcp.Description("Architecture (default: amd64)"), mcp.DefaultString("amd64")),
			mcp.WithString("stream", mcp.Description("Release stream: 'nightly' or 'ci' (default: nightly)"), mcp.DefaultString("nightly")),
		),
		GetPayloadStatusHandler(rc),
	)

	s.AddTool(
		mcp.NewTool("get_payload_diff",
			mcp.WithDescription("PRs that changed between two payloads — useful for identifying what went into a rejected payload"),
			mcp.WithString("release", mcp.Description("Release version. Defaults to current dev release.")),
			mcp.WithString("from_tag", mcp.Description("Source payload tag (if omitted, uses previous payload automatically)")),
			mcp.WithString("to_tag", mcp.Required(), mcp.Description("Target payload tag")),
		),
		GetPayloadDiffHandler(sippy),
	)

	s.AddTool(
		mcp.NewTool("get_payload_test_failures",
			mcp.WithDescription("Test failures across payload runs"),
			mcp.WithString("release", mcp.Description("Release version. Defaults to current dev release.")),
			mcp.WithString("payload_tag", mcp.Description("Specific payload tag to check")),
		),
		GetPayloadTestFailuresHandler(sippy),
	)
}

func GetPayloadStatusHandler(rc client.ReleaseController) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		release, err := req.RequireString("release")
		if err != nil {
			return tools.InvalidParam("release", "required")
		}

		arch := req.GetString("arch", "amd64")
		stream := req.GetString("stream", "nightly")

		streamName := fmt.Sprintf("%s.0-0.%s", release, stream)
		if arch != "amd64" && stream == "nightly" {
			streamName = fmt.Sprintf("%s.0-0.%s-%s", release, stream, arch)
		}

		path := fmt.Sprintf("/api/v1/releasestream/%s/tags", streamName)
		data, err := rc.GetForArch(ctx, arch, path, nil)
		if err != nil {
			return tools.ToolError(err)
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}

func GetPayloadDiffHandler(sippy client.Sippy) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		toTag, err := req.RequireString("to_tag")
		if err != nil {
			return tools.InvalidParam("to_tag", "required")
		}

		params := map[string]string{"toPayload": toTag}
		if fromTag := req.GetString("from_tag", ""); fromTag != "" {
			params["fromPayload"] = fromTag
		}

		data, err := sippy.Get(ctx, "/api/payloads/diff", params)
		if err != nil {
			return tools.ToolError(err)
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}

func GetPayloadTestFailuresHandler(sippy client.Sippy) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		release, err := tools.ResolveRelease(ctx, sippy, req.GetString("release", ""))
		if err != nil {
			return tools.ToolError(err)
		}

		params := map[string]string{"release": release}
		if tag := req.GetString("payload_tag", ""); tag != "" {
			params["payload_tag"] = tag
		}

		data, err := sippy.Get(ctx, "/api/payloads/test_failures", params)
		if err != nil {
			return tools.ToolError(err)
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./pkg/tools/domain/... -run TestGetPayload -v`
Expected: all 3 tests pass

- [ ] **Step 5: Register in server and commit**

Update `pkg/server/server.go` — add Release Controller client creation in the `New()` function, after the `sippy` client creation:

```go
rc := client.NewReleaseController(cfg.ReleaseControllerURL, httpClient)
```

And add the tool registration after the existing `RegisterComponentTools` call:

```go
domain.RegisterPayloadTools(s, sippy, rc)
```

```bash
git add pkg/tools/domain/payloads.go pkg/tools/domain/payloads_test.go pkg/server/server.go
git commit -m "feat: add payload tools — get_payload_status, get_payload_diff, get_payload_test_failures"
```

---

### Task 12: Search.CI Client and Search Tool

**Files:**
- Create: `pkg/client/searchci.go`
- Create: `pkg/client/searchci_test.go`
- Create: `pkg/tools/domain/search.go`
- Create: `pkg/tools/domain/search_test.go`

- [ ] **Step 1: Write the failing client test**

```go
// pkg/client/searchci_test.go
package client_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jeroche/openshift-ci-mcp/pkg/client"
)

func TestSearchCIClient_Search(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Query().Get("search") == "" {
			t.Error("expected search parameter")
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"results":{}}`))
	}))
	defer ts.Close()

	c := client.NewSearchCI(ts.URL, ts.Client())
	data, err := c.Search(context.Background(), "test failure", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("expected non-empty response")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./pkg/client/... -run TestSearchCI`
Expected: compilation failure

- [ ] **Step 3: Implement Search.CI client**

```go
// pkg/client/searchci.go
package client

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

type SearchCI interface {
	Search(ctx context.Context, query string, params map[string]string) ([]byte, error)
	Get(ctx context.Context, path string, params map[string]string) ([]byte, error)
}

type searchCIClient struct {
	baseURL    string
	httpClient *http.Client
}

func NewSearchCI(baseURL string, httpClient *http.Client) SearchCI {
	return &searchCIClient{baseURL: baseURL, httpClient: httpClient}
}

func (c *searchCIClient) Search(ctx context.Context, query string, params map[string]string) ([]byte, error) {
	u, err := url.Parse(c.baseURL + "/v2/search")
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}
	q := u.Query()
	q.Set("search", query)
	q.Set("type", "all")
	for k, v := range params {
		q.Set(k, v)
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, &HTTPError{StatusCode: resp.StatusCode, Body: body}
	}

	return body, nil
}

func (c *searchCIClient) Get(ctx context.Context, path string, params map[string]string) ([]byte, error) {
	u, err := url.Parse(c.baseURL + path)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}
	q := u.Query()
	for k, v := range params {
		q.Set(k, v)
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, &HTTPError{StatusCode: resp.StatusCode, Body: body}
	}

	return body, nil
}
```

- [ ] **Step 4: Write the failing search tool test**

```go
// pkg/tools/domain/search_test.go
package domain_test

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/jeroche/openshift-ci-mcp/pkg/tools/domain"
)

type mockSearchCI struct {
	response []byte
}

func (m *mockSearchCI) Search(ctx context.Context, query string, params map[string]string) ([]byte, error) {
	return m.response, nil
}

func (m *mockSearchCI) Get(ctx context.Context, path string, params map[string]string) ([]byte, error) {
	return m.response, nil
}

func TestSearchCILogs(t *testing.T) {
	mock := &mockSearchCI{
		response: []byte(`{"results":{"test-failure":{"matches":[{"context":["job-1"]}]}}}`),
	}

	handler := domain.SearchCILogsHandler(mock)
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"query": "test-failure",
	}
	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
}
```

- [ ] **Step 5: Implement search tool**

```go
// pkg/tools/domain/search.go
package domain

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/jeroche/openshift-ci-mcp/pkg/client"
	"github.com/jeroche/openshift-ci-mcp/pkg/tools"
)

func RegisterSearchTools(s *server.MCPServer, search client.SearchCI) {
	s.AddTool(
		mcp.NewTool("search_ci_logs",
			mcp.WithDescription("Search build logs and JUnit failures across OpenShift CI. Searches recent test results, job logs, and associated Jira issues."),
			mcp.WithString("query", mcp.Required(), mcp.Description("Search string — test name, error message, or job name pattern")),
			mcp.WithString("max_age", mcp.Description("Max age of results (e.g. '7d', '24h'). Default: server default.")),
			mcp.WithString("type", mcp.Description("Search type: 'all', 'bug', 'junit'. Default: 'all'.")),
		),
		SearchCILogsHandler(search),
	)
}

func SearchCILogsHandler(search client.SearchCI) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		query, err := req.RequireString("query")
		if err != nil {
			return tools.InvalidParam("query", "required")
		}

		params := map[string]string{}
		if maxAge := req.GetString("max_age", ""); maxAge != "" {
			params["maxAge"] = maxAge
		}
		if searchType := req.GetString("type", ""); searchType != "" {
			params["type"] = searchType
		}

		data, err := search.Search(ctx, query, params)
		if err != nil {
			return tools.ToolError(err)
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}
```

- [ ] **Step 6: Run all tests**

Run: `go test ./pkg/client/... -run TestSearchCI -v && go test ./pkg/tools/domain/... -run TestSearchCI -v`
Expected: all pass

- [ ] **Step 7: Register in server and commit**

Update `pkg/server/server.go`:

```go
search := client.NewSearchCI(cfg.SearchCIURL, httpClient)
// ...
domain.RegisterSearchTools(s, search)
```

```bash
git add pkg/client/searchci.go pkg/client/searchci_test.go pkg/tools/domain/search.go pkg/tools/domain/search_test.go pkg/server/server.go
git commit -m "feat: add Search.CI client and search_ci_logs tool"
```

---

### Task 13: Pull Request Domain Tools

**Files:**
- Create: `pkg/tools/domain/pullrequests.go`
- Create: `pkg/tools/domain/pullrequests_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// pkg/tools/domain/pullrequests_test.go
package domain_test

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/jeroche/openshift-ci-mcp/pkg/tools/domain"
)

func TestGetPullRequestImpact(t *testing.T) {
	mock := newMockSippy(map[string][]byte{
		"/api/pull_requests/test_results": []byte(`[{"test_name":"test-1","result":"Failed"}]`),
	})

	handler := domain.GetPullRequestImpactHandler(mock)
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"org":       "openshift",
		"repo":      "kubernetes",
		"pr_number": "12345",
	}
	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
}

func TestGetPullRequests(t *testing.T) {
	mock := newMockSippy(map[string][]byte{
		"/api/pull_requests": []byte(`{"rows":[{"org":"openshift","repo":"kubernetes","number":12345}],"total_rows":1}`),
	})

	handler := domain.GetPullRequestsHandler(mock)
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"release": "4.18",
		"org":     "openshift",
		"repo":    "kubernetes",
	}
	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./pkg/tools/domain/... -run TestGetPull`
Expected: compilation failure

- [ ] **Step 3: Implement pull request tools**

```go
// pkg/tools/domain/pullrequests.go
package domain

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/jeroche/openshift-ci-mcp/pkg/client"
	"github.com/jeroche/openshift-ci-mcp/pkg/tools"
)

func RegisterPullRequestTools(s *server.MCPServer, sippy client.Sippy) {
	s.AddTool(
		mcp.NewTool("get_pull_request_impact",
			mcp.WithDescription("Test failures associated with a specific pull request. Note: this endpoint is rate-limited to 20 req/hour."),
			mcp.WithString("org", mcp.Required(), mcp.Description("GitHub org (e.g. 'openshift')")),
			mcp.WithString("repo", mcp.Required(), mcp.Description("GitHub repo (e.g. 'kubernetes')")),
			mcp.WithString("pr_number", mcp.Required(), mcp.Description("Pull request number")),
		),
		GetPullRequestImpactHandler(sippy),
	)

	s.AddTool(
		mcp.NewTool("get_pull_requests",
			mcp.WithDescription("PR reports with filtering by org, repo, and release"),
			mcp.WithString("release", mcp.Description("Release version. Defaults to current dev release.")),
			mcp.WithString("org", mcp.Description("Filter by GitHub org")),
			mcp.WithString("repo", mcp.Description("Filter by GitHub repo")),
			mcp.WithNumber("limit", mcp.Description("Max results per page (default 25)"), mcp.DefaultNumber(25)),
			mcp.WithNumber("page", mcp.Description("Page number (default 1)"), mcp.DefaultNumber(1)),
		),
		GetPullRequestsHandler(sippy),
	)
}

func GetPullRequestImpactHandler(sippy client.Sippy) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		org, err := req.RequireString("org")
		if err != nil {
			return tools.InvalidParam("org", "required")
		}
		repo, err := req.RequireString("repo")
		if err != nil {
			return tools.InvalidParam("repo", "required")
		}
		prNumber, err := req.RequireString("pr_number")
		if err != nil {
			return tools.InvalidParam("pr_number", "required")
		}

		params := map[string]string{
			"org":       org,
			"repo":      repo,
			"pr_number": prNumber,
		}

		data, err := sippy.Get(ctx, "/api/pull_requests/test_results", params)
		if err != nil {
			return tools.ToolError(err)
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}

func GetPullRequestsHandler(sippy client.Sippy) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		release, err := tools.ResolveRelease(ctx, sippy, req.GetString("release", ""))
		if err != nil {
			return tools.ToolError(err)
		}

		params := map[string]string{
			"release": release,
			"perPage": fmt.Sprintf("%d", req.GetInt("limit", 25)),
			"page":    fmt.Sprintf("%d", req.GetInt("page", 1)),
		}
		if org := req.GetString("org", ""); org != "" {
			params["org"] = org
		}
		if repo := req.GetString("repo", ""); repo != "" {
			params["repo"] = repo
		}

		data, err := sippy.Get(ctx, "/api/pull_requests", params)
		if err != nil {
			return tools.ToolError(err)
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./pkg/tools/domain/... -run TestGetPull -v`
Expected: all 2 tests pass

- [ ] **Step 5: Register in server and commit**

Add to `pkg/server/server.go` in `New()`:

```go
domain.RegisterPullRequestTools(s, sippy)
```

```bash
git add pkg/tools/domain/pullrequests.go pkg/tools/domain/pullrequests_test.go pkg/server/server.go
git commit -m "feat: add pull request tools — get_pull_request_impact, get_pull_requests"
```

---

### Task 14: Proxy Tools

**Files:**
- Create: `pkg/tools/proxy/sippy.go`
- Create: `pkg/tools/proxy/releasecontroller.go`
- Create: `pkg/tools/proxy/searchci.go`
- Create: `pkg/tools/proxy/proxy_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// pkg/tools/proxy/proxy_test.go
package proxy_test

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/jeroche/openshift-ci-mcp/pkg/tools/proxy"
)

type mockClient struct {
	response []byte
}

func (m *mockClient) Get(ctx context.Context, path string, params map[string]string) ([]byte, error) {
	return m.response, nil
}

func (m *mockClient) GetForArch(ctx context.Context, arch, path string, params map[string]string) ([]byte, error) {
	return m.response, nil
}

func (m *mockClient) Search(ctx context.Context, query string, params map[string]string) ([]byte, error) {
	return m.response, nil
}

func TestSippyProxy(t *testing.T) {
	mock := &mockClient{response: []byte(`{"data":"raw"}`)}
	handler := proxy.SippyAPIHandler(mock)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"path": "/api/health",
	}
	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
}

func TestReleaseControllerProxy(t *testing.T) {
	mock := &mockClient{response: []byte(`{"data":"raw"}`)}
	handler := proxy.ReleaseControllerAPIHandler(mock)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"arch": "amd64",
		"path": "/api/v1/releasestream/4.18.0-0.nightly/tags",
	}
	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
}

func TestSearchCIProxy(t *testing.T) {
	mock := &mockClient{response: []byte(`{"results":{}}`)}
	handler := proxy.SearchCIAPIHandler(mock)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"query": "test failure",
	}
	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./pkg/tools/proxy/...`
Expected: compilation failure

- [ ] **Step 3: Implement proxy tools**

```go
// pkg/tools/proxy/sippy.go
package proxy

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/jeroche/openshift-ci-mcp/pkg/client"
	"github.com/jeroche/openshift-ci-mcp/pkg/tools"
)

func RegisterSippyProxy(s *server.MCPServer, sippy client.Sippy) {
	s.AddTool(
		mcp.NewTool("sippy_api",
			mcp.WithDescription("Raw passthrough to any Sippy API endpoint. Returns unmodified upstream response. See https://sippy.dptools.openshift.org/api for available endpoints."),
			mcp.WithString("path", mcp.Required(), mcp.Description("API path (e.g. '/api/jobs')")),
		),
		SippyAPIHandler(sippy),
	)
}

func SippyAPIHandler(sippy client.Sippy) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		path, err := req.RequireString("path")
		if err != nil {
			return tools.InvalidParam("path", "required")
		}

		params := extractParams(req)
		data, err := sippy.Get(ctx, path, params)
		if err != nil {
			return tools.ToolError(err)
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}

func extractParams(req mcp.CallToolRequest) map[string]string {
	result := make(map[string]string)
	args := req.GetArguments()
	if paramsRaw, ok := args["params"].(map[string]any); ok {
		for k, v := range paramsRaw {
			if s, ok := v.(string); ok {
				result[k] = s
			}
		}
	}
	if filterStr, ok := args["filter"].(string); ok && filterStr != "" {
		result["filter"] = filterStr
	}
	return result
}
```

```go
// pkg/tools/proxy/releasecontroller.go
package proxy

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/jeroche/openshift-ci-mcp/pkg/client"
	"github.com/jeroche/openshift-ci-mcp/pkg/tools"
)

func RegisterReleaseControllerProxy(s *server.MCPServer, rc client.ReleaseController) {
	s.AddTool(
		mcp.NewTool("release_controller_api",
			mcp.WithDescription("Raw passthrough to the Release Controller API. Returns unmodified upstream response."),
			mcp.WithString("arch", mcp.Description("Architecture (default: amd64)"), mcp.DefaultString("amd64")),
			mcp.WithString("path", mcp.Required(), mcp.Description("API path (e.g. '/api/v1/releasestream/4.18.0-0.nightly/tags')")),
		),
		ReleaseControllerAPIHandler(rc),
	)
}

func ReleaseControllerAPIHandler(rc client.ReleaseController) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		path, err := req.RequireString("path")
		if err != nil {
			return tools.InvalidParam("path", "required")
		}

		arch := req.GetString("arch", "amd64")
		params := extractParams(req)

		data, err := rc.GetForArch(ctx, arch, path, params)
		if err != nil {
			return tools.ToolError(err)
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}
```

```go
// pkg/tools/proxy/searchci.go
package proxy

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/jeroche/openshift-ci-mcp/pkg/client"
	"github.com/jeroche/openshift-ci-mcp/pkg/tools"
)

func RegisterSearchCIProxy(s *server.MCPServer, search client.SearchCI) {
	s.AddTool(
		mcp.NewTool("search_ci_api",
			mcp.WithDescription("Raw passthrough to Search.CI API. Returns unmodified upstream response."),
			mcp.WithString("query", mcp.Required(), mcp.Description("Search query string")),
		),
		SearchCIAPIHandler(search),
	)
}

func SearchCIAPIHandler(search client.SearchCI) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		query, err := req.RequireString("query")
		if err != nil {
			return tools.InvalidParam("query", "required")
		}

		params := extractParams(req)
		data, err := search.Search(ctx, query, params)
		if err != nil {
			return tools.ToolError(err)
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./pkg/tools/proxy/... -v`
Expected: all 3 tests pass

- [ ] **Step 5: Register in server and commit**

Add to `pkg/server/server.go` in `New()`:

```go
proxy.RegisterSippyProxy(s, sippy)
proxy.RegisterReleaseControllerProxy(s, rc)
proxy.RegisterSearchCIProxy(s, search)
```

Add import: `"github.com/jeroche/openshift-ci-mcp/pkg/tools/proxy"`

```bash
git add pkg/tools/proxy/ pkg/server/server.go
git commit -m "feat: add proxy tools — sippy_api, release_controller_api, search_ci_api"
```

---

### Task 15: HTTP/SSE Transport and CLI Flags

**Files:**
- Modify: `cmd/openshift-ci-mcp/main.go`

- [ ] **Step 1: Update main.go with flag parsing and dual transport**

```go
// cmd/openshift-ci-mcp/main.go
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/mark3labs/mcp-go/server"

	mcpserver "github.com/jeroche/openshift-ci-mcp/pkg/server"
)

func main() {
	transport := flag.String("transport", "stdio", "Transport mode: stdio or http")
	port := flag.Int("port", 8080, "HTTP port (only used with --transport http)")
	timeout := flag.Duration("timeout", 30*time.Second, "Upstream request timeout")
	flag.Parse()

	cfg := mcpserver.DefaultConfig()
	cfg.Timeout = *timeout

	if v := os.Getenv("SIPPY_URL"); v != "" {
		cfg.SippyURL = v
	}
	if v := os.Getenv("RELEASE_CONTROLLER_URL"); v != "" {
		cfg.ReleaseControllerURL = v
	}
	if v := os.Getenv("SEARCH_CI_URL"); v != "" {
		cfg.SearchCIURL = v
	}

	s := mcpserver.New(cfg)

	switch *transport {
	case "stdio":
		if err := server.ServeStdio(s); err != nil {
			fmt.Fprintf(os.Stderr, "server error: %v\n", err)
			os.Exit(1)
		}
	case "http":
		addr := fmt.Sprintf(":%d", *port)
		sseServer := server.NewSSEServer(s,
			server.WithBaseURL(fmt.Sprintf("http://localhost:%d", *port)),
		)
		log.Printf("Starting HTTP/SSE server on %s", addr)
		if err := sseServer.Start(addr); err != nil {
			fmt.Fprintf(os.Stderr, "server error: %v\n", err)
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "unknown transport: %s (use 'stdio' or 'http')\n", *transport)
		os.Exit(1)
	}
}
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./cmd/openshift-ci-mcp/`
Expected: compiles successfully

- [ ] **Step 3: Test stdio mode**

Run: `echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}' | ./openshift-ci-mcp 2>/dev/null | head -c 200`
Expected: JSON response with server info

- [ ] **Step 4: Commit**

```bash
git add cmd/openshift-ci-mcp/main.go
git commit -m "feat: add HTTP/SSE transport mode and CLI flags"
```

---

### Task 16: Container Build

**Files:**
- Create: `Containerfile`
- Create: `Makefile`

- [ ] **Step 1: Create Containerfile**

```dockerfile
FROM golang:1.24 AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /openshift-ci-mcp ./cmd/openshift-ci-mcp

FROM scratch
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /openshift-ci-mcp /openshift-ci-mcp
ENTRYPOINT ["/openshift-ci-mcp"]
```

Note: we copy CA certificates from the builder stage so the binary can make HTTPS requests from scratch.

- [ ] **Step 2: Create Makefile**

```makefile
BINARY := openshift-ci-mcp
IMAGE ?= quay.io/$(USER)/$(BINARY)
VERSION ?= 0.1.0

.PHONY: build test lint image push clean

build:
	go build -o $(BINARY) ./cmd/$(BINARY)

test:
	go test ./...

test-integration:
	go test -tags=integration ./...

lint:
	go vet ./...

image:
	podman build -t $(IMAGE):$(VERSION) -t $(IMAGE):latest -f Containerfile .

push: image
	podman push $(IMAGE):$(VERSION)
	podman push $(IMAGE):latest

clean:
	rm -f $(BINARY)
```

- [ ] **Step 3: Verify build**

Run: `make build && make test`
Expected: binary builds and all tests pass

- [ ] **Step 4: Build container image**

Run: `make image`
Expected: image builds successfully

- [ ] **Step 5: Test container in stdio mode**

Run: `echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}' | podman run -i --rm $(IMAGE):latest 2>/dev/null | head -c 200`
Expected: JSON response with server info

- [ ] **Step 6: Commit**

```bash
git add Containerfile Makefile
git commit -m "feat: add container build and Makefile"
```

---

### Task 17: Integration Tests

**Files:**
- Create: `pkg/client/integration_test.go`

- [ ] **Step 1: Write integration tests**

```go
//go:build integration

// pkg/client/integration_test.go
package client_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/jeroche/openshift-ci-mcp/pkg/client"
)

const integrationRelease = "4.18"

func integrationHTTPClient() *http.Client {
	return &http.Client{Timeout: 30 * time.Second}
}

func TestIntegration_Sippy_GetReleases(t *testing.T) {
	c := client.NewSippy("https://sippy.dptools.openshift.org", integrationHTTPClient())
	data, err := c.Get(context.Background(), "/api/releases", nil)
	if err != nil {
		t.Fatalf("failed to get releases: %v", err)
	}

	var resp map[string]any
	if err := json.Unmarshal(data, &resp); err != nil {
		t.Fatalf("invalid JSON response: %v", err)
	}

	releases, ok := resp["releases"].([]any)
	if !ok || len(releases) == 0 {
		t.Fatal("expected non-empty releases list")
	}
	t.Logf("found %d releases", len(releases))
}

func TestIntegration_Sippy_GetHealth(t *testing.T) {
	c := client.NewSippy("https://sippy.dptools.openshift.org", integrationHTTPClient())
	data, err := c.Get(context.Background(), "/api/health", map[string]string{"release": integrationRelease})
	if err != nil {
		t.Fatalf("failed to get health: %v", err)
	}

	var resp map[string]any
	if err := json.Unmarshal(data, &resp); err != nil {
		t.Fatalf("invalid JSON response: %v", err)
	}

	if _, ok := resp["indicators"]; !ok {
		t.Error("expected indicators in health response")
	}
}

func TestIntegration_Sippy_GetJobs(t *testing.T) {
	c := client.NewSippy("https://sippy.dptools.openshift.org", integrationHTTPClient())
	data, err := c.Get(context.Background(), "/api/jobs", map[string]string{
		"release": integrationRelease,
		"perPage": "5",
	})
	if err != nil {
		t.Fatalf("failed to get jobs: %v", err)
	}

	var resp map[string]any
	if err := json.Unmarshal(data, &resp); err != nil {
		t.Fatalf("invalid JSON response: %v", err)
	}

	rows, ok := resp["rows"].([]any)
	if !ok {
		t.Fatal("expected rows array in jobs response")
	}
	t.Logf("found %d jobs (page 1, limit 5)", len(rows))
}

func TestIntegration_Sippy_GetJobVariants(t *testing.T) {
	c := client.NewSippy("https://sippy.dptools.openshift.org", integrationHTTPClient())
	data, err := c.Get(context.Background(), "/api/job_variants", nil)
	if err != nil {
		t.Fatalf("failed to get job variants: %v", err)
	}

	var resp map[string]any
	if err := json.Unmarshal(data, &resp); err != nil {
		t.Fatalf("invalid JSON response: %v", err)
	}

	variants, ok := resp["variants"].(map[string]any)
	if !ok {
		t.Fatal("expected variants map in response")
	}
	t.Logf("found %d variant dimensions", len(variants))
}

func TestIntegration_ReleaseController_GetTags(t *testing.T) {
	c := client.NewReleaseController("https://amd64.ocp.releases.ci.openshift.org", integrationHTTPClient())
	data, err := c.Get(context.Background(), "/api/v1/releasestream/4.18.0-0.nightly/tags", nil)
	if err != nil {
		t.Fatalf("failed to get tags: %v", err)
	}

	var resp map[string]any
	if err := json.Unmarshal(data, &resp); err != nil {
		t.Fatalf("invalid JSON response: %v", err)
	}

	tags, ok := resp["tags"].([]any)
	if !ok {
		t.Fatal("expected tags array in response")
	}
	t.Logf("found %d payload tags", len(tags))
}
```

- [ ] **Step 2: Run integration tests**

Run: `go test -tags=integration ./pkg/client/... -v -timeout 60s`
Expected: all integration tests pass (requires network access to real APIs)

- [ ] **Step 3: Verify default test run skips integration tests**

Run: `go test ./pkg/client/... -v`
Expected: integration tests not included

- [ ] **Step 4: Commit**

```bash
git add pkg/client/integration_test.go
git commit -m "feat: add integration tests for upstream API connectivity"
```

---

### Task 18: Final Wiring and Verification

**Files:**
- Modify: `pkg/server/server.go`

- [ ] **Step 1: Verify complete server.go with all registrations**

Ensure `pkg/server/server.go` has all tools registered:

```go
package server

import (
	"net/http"
	"time"

	"github.com/mark3labs/mcp-go/server"

	"github.com/jeroche/openshift-ci-mcp/pkg/client"
	"github.com/jeroche/openshift-ci-mcp/pkg/tools/domain"
	"github.com/jeroche/openshift-ci-mcp/pkg/tools/proxy"
)

type Config struct {
	SippyURL             string
	ReleaseControllerURL string
	SearchCIURL          string
	Timeout              time.Duration
}

func DefaultConfig() Config {
	return Config{
		SippyURL:             "https://sippy.dptools.openshift.org",
		ReleaseControllerURL: "https://amd64.ocp.releases.ci.openshift.org",
		SearchCIURL:          "https://search.ci.openshift.org",
		Timeout:              30 * time.Second,
	}
}

func New(cfg Config) *server.MCPServer {
	httpClient := &http.Client{Timeout: cfg.Timeout}

	sippy := client.NewSippy(cfg.SippyURL, httpClient)
	rc := client.NewReleaseController(cfg.ReleaseControllerURL, httpClient)
	search := client.NewSearchCI(cfg.SearchCIURL, httpClient)

	s := server.NewMCPServer(
		"openshift-ci-mcp",
		"0.1.0",
		server.WithToolCapabilities(false),
		server.WithRecovery(),
	)

	domain.RegisterReleaseTools(s, sippy)
	domain.RegisterVariantTools(s, sippy)
	domain.RegisterJobTools(s, sippy)
	domain.RegisterTestTools(s, sippy)
	domain.RegisterComponentTools(s, sippy)
	domain.RegisterPayloadTools(s, sippy, rc)
	domain.RegisterSearchTools(s, search)
	domain.RegisterPullRequestTools(s, sippy)

	proxy.RegisterSippyProxy(s, sippy)
	proxy.RegisterReleaseControllerProxy(s, rc)
	proxy.RegisterSearchCIProxy(s, search)

	return s
}
```

- [ ] **Step 2: Run full test suite**

Run: `make test`
Expected: all tests pass

- [ ] **Step 3: Build and verify tool listing**

Run: `go build -o openshift-ci-mcp ./cmd/openshift-ci-mcp && echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}
{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}' | ./openshift-ci-mcp 2>/dev/null`

Expected: JSON listing 21 tools (18 domain + 3 proxy)

- [ ] **Step 4: Build container and verify**

Run: `make image && echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}' | podman run -i --rm openshift-ci-mcp:latest 2>/dev/null | head -c 200`

Expected: server responds from container

- [ ] **Step 5: Final commit**

```bash
git add pkg/server/server.go
git commit -m "feat: complete tool registration — 21 tools across domain and proxy tiers"
```
