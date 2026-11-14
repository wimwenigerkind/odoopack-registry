// Package buildinfo holds the release the binaries were built from.
package buildinfo

// Version is set at build time through
// -ldflags "-X github.com/wimwenigerkind/odoopack-registry/internal/buildinfo.Version=1.2.3":
// the release (1.2.3) for a tag build, main-<sha> for a main build.
var Version = "dev"
