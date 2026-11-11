package auth

import "regexp"

// Userinfo may hold a token alone (Forgejo) or user:password, and a password
// with an unescaped "@" must not leave its tail behind, hence everything up
// to the last "@" before the path.
var credentialRe = regexp.MustCompile(`([a-zA-Z][a-zA-Z0-9+.-]*://)[^/\s]+@`)

func RedactURLCredentials(s string) string {
	return credentialRe.ReplaceAllString(s, "$1***@")
}
