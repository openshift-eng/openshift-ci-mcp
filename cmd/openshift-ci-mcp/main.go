package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/mark3labs/mcp-go/server"

	mcpserver "github.com/openshift-eng/openshift-ci-mcp/pkg/server"
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
