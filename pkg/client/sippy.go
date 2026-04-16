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
