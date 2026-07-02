package validate

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

func GitURL(raw string) error {
	s := strings.TrimSpace(raw)
	if s == "" {
		return fmt.Errorf("git URL is required")
	}
	u, err := url.Parse(s)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	switch strings.ToLower(u.Scheme) {
	case "https", "ssh":
	default:
		return fmt.Errorf("unsupported git URL scheme %q — only https:// and ssh:// are allowed", u.Scheme)
	}
	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("git URL missing host")
	}
	if isBlockedHost(host) {
		return fmt.Errorf("host %q is not allowed", host)
	}
	return nil
}

func isBlockedHost(host string) bool {
	h := strings.ToLower(host)
	switch h {
	case "localhost", "metadata.google.internal", "metadata", "instance-data":
		return true
	}
	if ip := net.ParseIP(h); ip != nil {
		if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsPrivate() || ip.IsUnspecified() {
			return true
		}
	}
	return false
}
