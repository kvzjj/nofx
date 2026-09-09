// Package netguard provides an HTTP client for fetching user-configured data
// source URLs (quant data, OI rankings, external feeds). All outbound requests
// are validated at dial time so the backend cannot be turned into an SSRF
// proxy against localhost, private networks, or cloud metadata endpoints.
package netguard

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// DefaultMaxResponseBytes caps how much of a data-source response is read
// into memory (2 MiB is far beyond any legitimate JSON market-data payload).
const DefaultMaxResponseBytes = 2 << 20

const maxRedirects = 3

// Options configures a guarded client.
type Options struct {
	// Timeout is the overall request timeout. Values <= 0 fall back to 15s.
	Timeout time.Duration
	// AllowPrivate permits loopback/private/link-local targets. Off by
	// default; intended for self-hosted data services on a LAN.
	AllowPrivate bool
}

// AllowPrivateFromEnv reports whether private-network data sources were
// explicitly enabled via NOFX_ALLOW_PRIVATE_DATA_SOURCES=true.
func AllowPrivateFromEnv() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv("NOFX_ALLOW_PRIVATE_DATA_SOURCES")), "true")
}

// CheckURL performs static validation of a configured URL: only http/https
// schemes are allowed. Host reachability is enforced later, at dial time.
func CheckURL(rawURL string) error {
	if strings.TrimSpace(rawURL) == "" {
		return fmt.Errorf("URL is empty")
	}
	u, err := parseURL(rawURL)
	if err != nil {
		return err
	}
	if u.Host == "" {
		return fmt.Errorf("URL %q has no host", rawURL)
	}
	return nil
}

// NewClient returns an *http.Client whose transport refuses to dial
// loopback/private/link-local/multicast addresses (unless allowed) and
// follows at most a few redirects, each re-validated the same way.
// Validating at dial time (instead of only resolving up front) also
// neutralizes DNS-rebinding tricks.
func NewClient(opts Options) *http.Client {
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	transport := &http.Transport{
		Proxy: nil, // never route data-source fetches through ambient env proxies
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return dialGuarded(ctx, network, addr, opts.AllowPrivate)
		},
	}
	return &http.Client{
		Transport: transport,
		Timeout:   timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= maxRedirects {
				return fmt.Errorf("stopped after %d redirects", maxRedirects)
			}
			if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
				return fmt.Errorf("redirect to unsupported scheme %q blocked", req.URL.Scheme)
			}
			return nil
		},
	}
}

// ReadLimited reads at most maxBytes from r and fails if the body is larger.
// A what label is included in error messages for diagnosability.
func ReadLimited(r io.Reader, maxBytes int64, what string) ([]byte, error) {
	if maxBytes <= 0 {
		maxBytes = DefaultMaxResponseBytes
	}
	body, err := io.ReadAll(io.LimitReader(r, maxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("failed to read %s response: %w", what, err)
	}
	if int64(len(body)) > maxBytes {
		return nil, fmt.Errorf("%s response exceeds %d bytes limit", what, maxBytes)
	}
	return body, nil
}

// ResponseSizeLimit returns the configured maximum response size, overridable
// via NOFX_MAX_DATA_RESPONSE_BYTES for unusual deployments.
func ResponseSizeLimit() int64 {
	if v := strings.TrimSpace(os.Getenv("NOFX_MAX_DATA_RESPONSE_BYTES")); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 {
			return n
		}
	}
	return DefaultMaxResponseBytes
}
