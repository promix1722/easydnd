#!/bin/bash
# Activate a release: unpack the frontend, atomic symlink swap, restart under supervisor,
# health-gate, roll back on failure. Runs on the server as the `deploy` user.
# Argument: the release directory name (git SHA).
#
# The directory name and the release identifier are two different things now.
# Directories stay keyed by commit SHA, because that is unique per build and
# cannot collide when a tag is moved; the identifier the binary reports is the
# tag (v1.0.4). So the health gate cannot be derived from the argument -- it is
# read from releases/<sha>/VERSION, which the deploy workflow writes.
#
# The binary and the frontend live in the same releases/<sha> directory and are swapped by the
# same symlink, so nginx and supervisor always agree on which release is live and a rollback
# reverts both halves together.
#
# NOTE: `set -e` deliberately does NOT cover the supervisorctl restarts. A broken binary makes
# `supervisorctl restart` exit non-zero ("spawn error"), which under errexit killed this script
# before the health check and rollback could run -- turning a failed deploy into an outage.
set -euo pipefail

SHA="${1:?usage: deploy.sh <git-sha>}"
ROOT=/opt/easydnd
NEW="$ROOT/releases/$SHA"
LINK="$ROOT/current"
# The health check's own port, not the app's: the app reads http.port from
# CONFIG below. Keep the two in sync by hand -- nothing links them any more.
PORT="${PORT:-8080}"
CONFIG=/etc/easydnd/config.yaml
KEEP=5

[ -x "$NEW/easydnd" ] || { echo "no executable at $NEW/easydnd"; exit 1; }

# A release is a directory, not a file: the binary, the SRD compendium and the
# frontend bundle all have to be present. Every check below runs before the
# symlink swap on purpose -- a half-uploaded release that goes live is an
# outage that the health gate then has to undo, and for the frontend it is an
# outage the API-side gate cannot even see.

# The SRD compendium is read from disk at startup, so a release without it
# refuses to boot.
[ -d "$NEW/data/srd_5.1" ] || { echo "no SRD data at $NEW/data/srd_5.1"; exit 1; }

# Same for the config file, which now carries every setting including the
# session signing key. It is not part of a release -- it is installed once by
# hand -- so this catches the case where supervisor was pointed at a path that
# was never created. `-e` and not `-r`: the file is 640 root:easydnd and this
# script runs as `deploy`, which is deliberately not allowed to read it.
#
# This needs /etc/easydnd to be mode 751, not 750: testing a path requires
# execute permission on every parent directory, so a 750 directory fails this
# check with EACCES while the file sits there perfectly readable by the service
# account. That is how the v0.5.0 deploy failed. See deploy/config.example.yaml.
[ -e "$CONFIG" ] || {
    echo "no config at $CONFIG -- install it from deploy/config.example.yaml first"
    exit 1
}

# nginx serves $LINK/web, so a bundle that unpacked badly would go live as a
# blank site. Re-running is safe: the tarball is gone the second time, and
# web/ remains.
if [ -f "$NEW/web.tar.gz" ]; then
    rm -rf "$NEW/web"
    mkdir -p "$NEW/web"
    tar -xzf "$NEW/web.tar.gz" -C "$NEW/web"
    rm -f "$NEW/web.tar.gz"
fi
[ -f "$NEW/web/index.html" ] || { echo "no frontend at $NEW/web/index.html"; exit 1; }

# The release identifier, which is what the health gate below looks for. It is
# required rather than defaulted: a release whose VERSION file never arrived
# would fall back to the directory name, fail the gate for a reason that has
# nothing to do with the binary, and roll back with a misleading message.
[ -f "$NEW/VERSION" ] || {
    echo "no release id at $NEW/VERSION -- the deploy workflow writes it beside the binary"
    exit 1
}

# The identifier a given release reports, recorded with the release because it
# can no longer be derived from the directory name.
#
# The fallback is for rollback targets that predate this file. Every release
# built under the old scheme reported its own directory name, so the basename
# is exactly right for those and wrong for nothing -- and the first deploy
# after this change has precisely such a release behind it.
version_of() {
    if [ -f "$1/VERSION" ]; then
        tr -d '[:space:]' < "$1/VERSION"
    else
        basename "$1"
    fi
}
VERSION="$(version_of "$NEW")"

PREV=""
if [ -L "$LINK" ]; then PREV="$(readlink -f "$LINK")"; fi

swap() {
    # ln -sfn on an existing symlink-to-directory is not atomic; create then rename.
    ln -sfn "$1" "$LINK.tmp"
    mv -T "$LINK.tmp" "$LINK"
}

restart() {
    # never let a failed restart abort the script -- rollback depends on reaching the code below
    sudo /usr/bin/supervisorctl restart easydnd || true
}

# Matches the JSON field rather than searching the body for the identifier
# loose. Two reasons, both of which arrived with tags: `v1.0.4` is a substring
# of `v1.0.40`, so a bare match would call a release healthy that is not; and
# grep reads its pattern as a regex, in which `.` matches anything -- so -F,
# without which `v1.0.4` would be satisfied by `v1X0Y4`.
health() {
    local want="\"version\":\"$1\""
    for _ in $(seq 1 15); do
        if curl -fsS -m 3 "http://127.0.0.1:$PORT/v1/version" 2>/dev/null | grep -qF "$want"; then
            return 0
        fi
        sleep 1
    done
    return 1
}

echo "activating $VERSION from $SHA (previous: ${PREV:-none})"
swap "$NEW"
restart

if health "$VERSION"; then
    echo "healthy: $VERSION"
else
    echo "health check failed for $VERSION (release $SHA)" >&2
    # Only the binary is required of a rollback target. The release preceding
    # the frontend rollout has no web/ at all, and rolling back to a working
    # API with no site still beats staying on a release that serves neither.
    if [ -n "$PREV" ] && [ -x "$PREV/easydnd" ]; then
        echo "rolling back to $PREV" >&2
        swap "$PREV"
        restart
        if health "$(version_of "$PREV")"; then
            echo "rollback healthy - site is up on the previous release" >&2
        else
            echo "ROLLBACK ALSO UNHEALTHY - service is down" >&2
        fi
    else
        echo "no previous release to roll back to" >&2
    fi
    exit 1
fi

# prune old releases, keeping the current one and the most recent $KEEP
cd "$ROOT/releases"
ls -1dt */ 2>/dev/null | tail -n +$((KEEP + 1)) | while read -r d; do
    d="${d%/}"
    [ "$d" = "$SHA" ] && continue
    echo "pruning old release $d"
    rm -rf -- "$d"
done
echo "done"
