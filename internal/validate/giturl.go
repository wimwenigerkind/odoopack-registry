package validate

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"regexp"
	"strings"
	"time"
)

var scpStyleRe = regexp.MustCompile(`^([^@:/\s]+)@([^@:/\s]+):([^/].*)$`)

// numericLabelRe matches a label that inet_aton, and with it git's resolver,
// reads as a number: decimal, octal with a leading zero, or hex.
var numericLabelRe = regexp.MustCompile(`^(0[xX][0-9a-fA-F]*|[0-9]+)$`)

const lookupTimeout = 5 * time.Second

// ErrHostNotAllowed covers both a blocked address and a failed lookup, so
// the message does not tell a caller which internal names exist.
var ErrHostNotAllowed = errors.New("git host is not allowed or cannot be resolved")

// Resolver is the part of net.Resolver the host check needs.
type Resolver interface {
	LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error)
}

var cgnat = netip.MustParsePrefix("100.64.0.0/10")

// thisNetwork is 0.0.0.0/8; Linux connects 0.0.0.0 to the local host, and no
// address in the range is routable.
var thisNetwork = netip.MustParsePrefix("0.0.0.0/8")

func GitURL(ctx context.Context, raw string) error {
	return gitURL(ctx, raw, net.DefaultResolver)
}

// GitHost checks only the host of a URL the registry is about to hand to
// git. The URL passed validation when it was stored, but the name may
// resolve differently by now.
func GitHost(ctx context.Context, raw string) error {
	return gitHost(ctx, raw, net.DefaultResolver)
}

func gitURL(ctx context.Context, raw string, r Resolver) error {
	u, err := parseGitURL(raw)
	if err != nil {
		return err
	}
	if u.User != nil {
		// ssh needs a user name and it carries no secret; a password, or any
		// userinfo on https, would keep a credential in a stored, displayed URL.
		if _, hasPassword := u.User.Password(); hasPassword || strings.ToLower(u.Scheme) != "ssh" {
			return fmt.Errorf("git URL must not contain credentials; connect an integration instead")
		}
	}
	return checkHost(ctx, u.Hostname(), r)
}

func gitHost(ctx context.Context, raw string, r Resolver) error {
	u, err := parseGitURL(raw)
	if err != nil {
		return err
	}
	return checkHost(ctx, u.Hostname(), r)
}

func parseGitURL(raw string) (*url.URL, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return nil, fmt.Errorf("git URL is required")
	}
	if !strings.Contains(s, "://") {
		if m := scpStyleRe.FindStringSubmatch(s); m != nil {
			s = "ssh://" + m[1] + "@" + m[2] + "/" + m[3]
		}
	}
	u, err := url.Parse(s)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}
	switch strings.ToLower(u.Scheme) {
	case "https", "ssh":
	default:
		return nil, fmt.Errorf("unsupported git URL scheme %q, only https:// and ssh:// are allowed", u.Scheme)
	}
	if u.Hostname() == "" {
		return nil, fmt.Errorf("git URL missing host")
	}
	return u, nil
}

func checkHost(ctx context.Context, host string, r Resolver) error {
	host = strings.TrimRight(host, ".")
	if addr, err := netip.ParseAddr(host); err == nil {
		if blockedAddr(addr) {
			return ErrHostNotAllowed
		}
		return nil
	}
	// netip accepts only dotted-quad IPv4, but git hands the name to
	// getaddrinfo, which also reads 2130706433, 0x7f.1 or 0177.0.0.1 as
	// 127.0.0.1. No real host name ends in a numeric label, so such hosts
	// are rejected rather than normalized with a parser that could disagree
	// with the system's.
	labels := strings.Split(host, ".")
	if numericLabelRe.MatchString(labels[len(labels)-1]) {
		return fmt.Errorf("numeric git host %q must be written as a dotted-quad IPv4 address", host)
	}

	ctx, cancel := context.WithTimeout(ctx, lookupTimeout)
	defer cancel()
	addrs, err := r.LookupIPAddr(ctx, host)
	if err != nil || len(addrs) == 0 {
		return ErrHostNotAllowed
	}
	// Every address must pass: git may connect to any of them.
	for _, a := range addrs {
		addr, ok := netip.AddrFromSlice(a.IP)
		if !ok || blockedAddr(addr) {
			return ErrHostNotAllowed
		}
	}
	return nil
}

func blockedAddr(addr netip.Addr) bool {
	addr = addr.Unmap()
	return addr.IsLoopback() ||
		addr.IsPrivate() ||
		addr.IsLinkLocalUnicast() ||
		addr.IsUnspecified() ||
		addr.IsMulticast() ||
		cgnat.Contains(addr) ||
		thisNetwork.Contains(addr)
}
