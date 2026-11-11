package validate

import (
	"context"
	"errors"
	"net"
	"testing"
)

type fakeResolver map[string][]string

func (f fakeResolver) LookupIPAddr(_ context.Context, host string) ([]net.IPAddr, error) {
	ips, ok := f[host]
	if !ok {
		return nil, &net.DNSError{Err: "no such host", Name: host, IsNotFound: true}
	}
	addrs := make([]net.IPAddr, 0, len(ips))
	for _, ip := range ips {
		addrs = append(addrs, net.IPAddr{IP: net.ParseIP(ip)})
	}
	return addrs, nil
}

var resolver = fakeResolver{
	"git.example.com":             {"93.184.215.14"},
	"v6.example.com":              {"2606:2800:21f:cb07:6820:80da:af6b:8b2c"},
	"loopback.example.com":        {"127.0.0.1"},
	"private.example.com":         {"10.0.0.5"},
	"mixed.example.com":           {"93.184.215.14", "192.168.1.10"},
	"metadata.example.com":        {"169.254.169.254"},
	"cgnat.example.com":           {"100.64.0.1"},
	"mapped.example.com":          {"::ffff:10.0.0.5"},
	"ula.example.com":             {"fd00::1"},
	"acme.svc.cluster.local":      {"10.96.0.10"},
	"multicast.example.com":       {"239.255.255.250"},
	"empty.example.com":           {},
	"10.0.0.5.nip.io":             {"10.0.0.5"},
	"localhost":                   {"127.0.0.1", "::1"},
	"unspecified.example.com":     {"0.0.0.0"},
	"thisnetwork.example.com":     {"0.1.2.3"},
	"linklocal6.example.com":      {"fe80::1"},
	"publicmapped.example.com":    {"::ffff:93.184.215.14"},
	"multicast6.example.com":      {"ff02::1"},
	"privatemixed6.example.com":   {"2606:2800:21f:cb07:6820:80da:af6b:8b2c", "fc00::1"},
	"loopback6only.example.com":   {"::1"},
	"cgnatmapped.example.com":     {"::ffff:100.100.0.1"},
	"linklocalmapped.example.com": {"::ffff:169.254.1.1"},
}

func TestGitURL(t *testing.T) {
	tests := []struct {
		name string
		url  string
		ok   bool
	}{
		{"public https", "https://git.example.com/acme/addons.git", true},
		{"public ssh", "ssh://git@git.example.com/acme/addons.git", true},
		{"public scp style", "git@git.example.com:acme/addons.git", true},
		{"public IPv6 name", "https://v6.example.com/acme/addons.git", true},
		{"public IPv4 literal", "https://93.184.215.14/acme/addons.git", true},
		{"public IPv6 literal", "https://[2606:2800:21f:cb07:6820:80da:af6b:8b2c]/acme/addons.git", true},
		{"public IPv4-mapped name", "https://publicmapped.example.com/acme/addons.git", true},
		{"trailing dot", "https://git.example.com./acme/addons.git", true},

		{"unsupported scheme", "http://git.example.com/acme/addons.git", false},
		{"file scheme", "file:///srv/acme.git", false},
		{"missing host", "https:///acme/addons.git", false},
		{"https user", "https://acme@git.example.com/acme/addons.git", false},
		{"https user and token", "https://acme:s3cret@git.example.com/acme/addons.git", false},
		{"ssh password", "ssh://git:s3cret@git.example.com/acme/addons.git", false},

		{"loopback name", "https://loopback.example.com/r.git", false},
		{"localhost", "https://localhost/r.git", false},
		{"loopback IPv4 literal", "https://127.0.0.1/r.git", false},
		{"loopback IPv4 literal, trailing dot", "https://127.0.0.1./r.git", false},
		{"loopback IPv6 literal", "https://[::1]/r.git", false},
		{"loopback IPv6 only", "https://loopback6only.example.com/r.git", false},
		{"loopback ssh", "git@loopback.example.com:acme/r.git", false},
		{"private name", "https://private.example.com/r.git", false},
		{"private IPv4 literal", "https://192.168.0.1/r.git", false},
		{"private 172.16/12", "https://172.20.0.1/r.git", false},
		{"cluster service name", "https://acme.svc.cluster.local/r.git", false},
		{"nip.io name", "https://10.0.0.5.nip.io/r.git", false},
		{"unique local IPv6 name", "https://ula.example.com/r.git", false},
		{"unique local IPv6 literal", "https://[fd12:3456::1]/r.git", false},
		{"link-local metadata name", "https://metadata.example.com/r.git", false},
		{"link-local IPv4 literal", "https://169.254.169.254/r.git", false},
		{"link-local IPv6 name", "https://linklocal6.example.com/r.git", false},
		{"link-local IPv6 literal with zone", "https://[fe80::1%25en0]/r.git", false},
		{"CGNAT name", "https://cgnat.example.com/r.git", false},
		{"CGNAT literal", "https://100.127.255.254/r.git", false},
		{"unspecified name", "https://unspecified.example.com/r.git", false},
		{"unspecified IPv4 literal", "https://0.0.0.0/r.git", false},
		{"unspecified IPv6 literal", "https://[::]/r.git", false},
		{"this network", "https://thisnetwork.example.com/r.git", false},
		{"multicast name", "https://multicast.example.com/r.git", false},
		{"multicast IPv6 name", "https://multicast6.example.com/r.git", false},
		{"multicast IPv4 literal", "https://224.0.0.1/r.git", false},
		{"mixed public and private", "https://mixed.example.com/r.git", false},
		{"mixed public and unique local IPv6", "https://privatemixed6.example.com/r.git", false},
		{"IPv4-mapped private name", "https://mapped.example.com/r.git", false},
		{"IPv4-mapped CGNAT name", "https://cgnatmapped.example.com/r.git", false},
		{"IPv4-mapped link-local name", "https://linklocalmapped.example.com/r.git", false},
		{"IPv4-mapped loopback literal", "https://[::ffff:127.0.0.1]/r.git", false},
		{"IPv4-mapped private literal, hex form", "https://[::ffff:a00:5]/r.git", false},
		{"IPv4-mapped unspecified literal", "https://[::ffff:0.0.0.0]/r.git", false},
		{"decimal IPv4", "https://2130706433/r.git", false},
		{"hex IPv4", "https://0x7f000001/r.git", false},
		{"hex and short IPv4", "https://0x7f.1/r.git", false},
		{"octal IPv4", "https://0177.0.0.1/r.git", false},
		{"short IPv4", "https://127.1/r.git", false},
		{"decimal IPv4 over ssh", "git@2130706433:acme/r.git", false},
		{"unresolvable name", "https://nothing.example.com/r.git", false},
		{"name without addresses", "https://empty.example.com/r.git", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := gitURL(context.Background(), tt.url, resolver)
			if tt.ok && err != nil {
				t.Fatalf("gitURL(%q) = %v, want nil", tt.url, err)
			}
			if !tt.ok && err == nil {
				t.Fatalf("gitURL(%q) = nil, want an error", tt.url)
			}
		})
	}
}

// A blocked address and a name that does not resolve must look the same to
// the caller, or the error tells which internal names exist.
func TestGitURLDoesNotTellBlockedFromUnresolvable(t *testing.T) {
	for _, u := range []string{
		"https://acme.svc.cluster.local/r.git",
		"https://nothing.example.com/r.git",
	} {
		if err := gitURL(context.Background(), u, resolver); !errors.Is(err, ErrHostNotAllowed) {
			t.Errorf("gitURL(%q) = %v, want ErrHostNotAllowed", u, err)
		}
	}
}

// The check before a git operation looks only at the host: a remote may carry
// a user name the integration added.
func TestGitHost(t *testing.T) {
	if err := gitHost(context.Background(), "https://acme@git.example.com/r.git", resolver); err != nil {
		t.Errorf("public host: %v", err)
	}
	if err := gitHost(context.Background(), "https://private.example.com/r.git", resolver); !errors.Is(err, ErrHostNotAllowed) {
		t.Errorf("private host: %v, want ErrHostNotAllowed", err)
	}
}
