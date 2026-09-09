package netguard

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"
)

func parseURL(rawURL string) (*url.URL, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("URL scheme %q is not allowed (only http/https)", u.Scheme)
	}
	return u, nil
}

// dialGuarded resolves addr and only dials if every candidate IP is
// acceptable. Checking all resolved addresses prevents partial-DNS-rebind
// attacks where the resolver rotates between a public and a private IP.
func dialGuarded(ctx context.Context, network, addr string, allowPrivate bool) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, fmt.Errorf("invalid dial address %q: %w", addr, err)
	}
	if strings.TrimSpace(host) == "" {
		return nil, fmt.Errorf("dial address %q has no host", addr)
	}

	resolver := &net.Resolver{}
	ips, err := resolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve %q: %w", host, err)
	}
	if len(ips) == 0 {
		return nil, fmt.Errorf("no addresses resolved for %q", host)
	}

	var candidates []net.IP
	for _, ip := range ips {
		if err := checkIP(ip.IP, allowPrivate); err != nil {
			return nil, err
		}
		candidates = append(candidates, ip.IP)
	}

	var lastErr error
	for _, ip := range candidates {
		dialer := &net.Dialer{Timeout: 10 * time.Second}
		conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
		if err == nil {
			return conn, nil
		}
		lastErr = err
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no dialed connection for %q", addr)
	}
	return nil, lastErr
}

func checkIP(ip net.IP, allowPrivate bool) error {
	if ip == nil {
		return fmt.Errorf("nil address")
	}
	if allowPrivate {
		return nil
	}
	// Normalize IPv4-mapped IPv6 so the checks below behave consistently.
	if v4 := ip.To4(); v4 != nil {
		ip = v4
	}
	blocked := ""
	switch {
	case ip.IsLoopback():
		blocked = "loopback"
	case ip.IsPrivate():
		blocked = "private"
	case ip.IsLinkLocalUnicast(), ip.IsLinkLocalMulticast():
		blocked = "link-local"
	case ip.IsUnspecified():
		blocked = "unspecified"
	case ip.IsMulticast():
		blocked = "multicast"
	case !ip.IsGlobalUnicast():
		// Catch-all for reserved/broadcast/otherwise non-routable ranges.
		blocked = "non-global-unicast"
	}
	if blocked != "" {
		return fmt.Errorf("data source target %s is a %s address and was blocked (SSRF protection)", ip, blocked)
	}
	return nil
}
