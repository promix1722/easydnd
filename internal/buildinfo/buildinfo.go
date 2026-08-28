// Package buildinfo carries metadata stamped into the binary by the linker.
package buildinfo

// Version is the release identifier: the tag on a release built by the deploy
// pipeline (`v1.0.4`), a short commit SHA on every other build including a
// local one of that same commit, and "dev" when nothing was injected at all.
// See deploy/release-version.sh for why being in the pipeline is the thing
// that earns a tag. It is stamped at link time with
//
//	-ldflags "-X github.com/promix1722/easydnd/internal/buildinfo.Version=v1.0.4"
//
// GET /v1/version serves this value and middleware.AppVersion puts it on every
// response, so it is what a browser compares against its own bundle to decide
// whether it is running code from a release that is no longer deployed.
// deploy/deploy.sh gates a release on finding it -- so a broken injection
// means a failed deploy, which is the point: the alternative is a binary that
// silently reports "dev" forever.
//
// Constraints that make -X work; do not "clean these up":
//   - it must stay a package-level var, never a const;
//   - its type must stay plain string;
//   - its initializer must stay a simple string literal;
//   - the value must be read somewhere reachable, or the linker drops the
//     symbol and -X silently becomes a no-op.
var Version = "dev"
