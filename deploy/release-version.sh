#!/bin/bash
# Print the release identifier for this build, and nothing else.
#
# ONE DEFINITION, THREE CALLERS: the Makefile's VERSION for local builds, the
# two build jobs in .github/workflows/deploy.yml, and the public gates in its
# restart job. They have to agree -- the binary, the bundle and every check on
# them are all comparing this string -- and the cheapest way to guarantee that
# is to have one place that decides it.
#
# A tag push through the release pipeline is a release, and is named by its
# tag: `v1.0.4` is what someone reads in the UI and quotes in a bug report.
# EVERYTHING ELSE GETS A SHORT COMMIT SHA -- a `ci/*` dry run, a
# workflow_dispatch from a branch, `make dev`, `make verify`, a hand-run
# `make build/release` -- because none of those is a release and none of them
# should claim to be one.
#
# Note what that rules out: a laptop build of a commit that happens to sit on a
# tag still reports the SHA. It is tempting to answer `v1.0.4` there, and it
# would be wrong -- the working tree may differ from the tag, nothing has been
# through CI, and a version string that says `v1.0.4` while being none of the
# things a release is makes every later bug report ambiguous. Being in the
# pipeline is the thing that makes a build a release, and `GITHUB_REF` is how
# this knows.
#
# The release *directory* is still keyed by the full commit SHA; that is a
# different thing and deliberately so. See deploy/deploy.sh.
set -euo pipefail

# In CI the ref is authoritative, and `git describe` is not: actions/checkout
# fetches shallowly and cannot be relied on to have the tags describe needs.
if [ -n "${GITHUB_REF:-}" ]; then
    case "$GITHUB_REF" in
        refs/tags/v*) printf '%s\n' "${GITHUB_REF_NAME}"; exit 0 ;;
    esac
    printf '%s\n' "${GITHUB_SHA:0:7}"
    exit 0
fi

# Outside CI, always the commit. No `git describe` here at all: the tag is the
# pipeline's to award, and asking git whether this commit happens to carry one
# is asking the wrong question.
#
# An `if` rather than `cmd && exit 0`, because under `set -e` a failing
# left-hand side of `&&` would take the whole script down instead of falling
# through to the next case.
if v="$(git rev-parse --short HEAD 2>/dev/null)"; then
    printf '%s\n' "$v"
else
    # No git at all: a tarball export, or a container with the .git stripped.
    printf 'dev\n'
fi
