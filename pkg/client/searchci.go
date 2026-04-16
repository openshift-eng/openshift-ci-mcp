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
	return c.doRequest(ctx, u.String())
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
	return c.doRequest(ctx, u.String())
}

func (c *searchCIClient) doRequest(ctx context.Context, rawURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
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
