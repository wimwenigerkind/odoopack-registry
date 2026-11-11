package auth

import "testing"

func TestRedactURLCredentials(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		{"token only", "git clone https://gto_secret@code.example.com/o/r.git@main: exit status 128", "git clone https://***@code.example.com/o/r.git@main: exit status 128"},
		{"user and password", "fatal: unable to access 'https://oauth2:ghp_secret@github.com/o/r/'", "fatal: unable to access 'https://***@github.com/o/r/'"},
		{"password with at sign", "https://x-token-auth:se@cret@bitbucket.org/o/r", "https://***@bitbucket.org/o/r"},
		{"other scheme", "ssh://git:secret@host/o/r", "ssh://***@host/o/r"},
		{"multiple urls", "https://a:one@x.org/r and http://two@y.org/r", "https://***@x.org/r and http://***@y.org/r"},
		{"no credentials", "https://github.com/o/r.git@main", "https://github.com/o/r.git@main"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := RedactURLCredentials(tc.in); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}
