package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/openshift-eng/openshift-ci-mcp/pkg/client"
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
