// Package buildinfo carries metadata stamped into the binary by the linker.
package buildinfo

// Version is the release identifier: the git SHA in CI, "dev" otherwise. It is
// injected at link time with
//
//	-ldflags "-X github.com/promix1722/easydnd/internal/buildinfo.Version=<sha>"
//
// GET /v1/version serves this value, and deploy/deploy.sh gates a release on
// finding the SHA there -- so a broken injection means a failed deploy.
//
// Constraints that make -X work; do not "clean these up":
//   - it must stay a package-level var, never a const;
//   - its type must stay plain string;
//   - its initializer must stay a simple string literal;
//   - the value must be read somewhere reachable, or the linker drops the
//     symbol and -X silently becomes a no-op.
var Version = "dev"
