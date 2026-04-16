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
