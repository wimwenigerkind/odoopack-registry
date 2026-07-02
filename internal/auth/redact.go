package auth

import "regexp"

var credentialRe = regexp.MustCompile(`(https?://)[^:\s/@]+:[^@\s]+@`)

func RedactURLCredentials(s string) string {
	return credentialRe.ReplaceAllString(s, "$1***@")
}
