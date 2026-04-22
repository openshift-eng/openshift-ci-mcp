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
		Timeout:              60 * time.Second,
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
