#!/usr/bin/env bash
#
# Copy a private rule pack to the production server. Run BY HAND, from a
# machine that has the pack -- it is never in this repository and no tag deploy
# carries it.
#
#   deploy/push-private-pack.sh ~/projects/easydnd-2014/pack [ssh-host]
#
# The host defaults to `easydnd` in ~/.ssh/config and has to be a login that
# can sudo: the pack lands root:easydnd, readable by the service and by nobody
# else, which the `deploy` account cannot arrange.
#
# The pack goes to /opt/easydnd/private-packs/<pack id>/, outside releases/, so
# no deploy replaces it and no prune removes it. The server only loads it once
# its path is in EASYDND_PRIVATE_PACK_FILES in /etc/easydnd/prod.env -- a line
# to add once, AFTER the first push, because a path that does not exist is a
# startup error. This script says so when it is the first push.
#
# A pack the server cannot load would take the service down at the restart, so
# two things stand in the way: the pack is compiled here first, against this
# checkout's SRD, and the previous copy is kept until the restarted service
# answers -- if it does not, the old copy goes back and the service restarts on
# it.
set -euo pipefail

PACK="${1:?usage: push-private-pack.sh <pack-dir> [ssh-host]}"
HOST="${2:-easydnd}"
PORT="${PORT:-8080}"
cd "$(dirname "$0")/.."

[ -f "$PACK/pack-manifest.json" ] || { echo "no pack-manifest.json in $PACK"; exit 1; }
# The manifest's own id is its first "id"; the dependencies' come after it.
ID="$(sed -n 's/.*"id": *"\([a-z0-9-]*\)".*/\1/p' "$PACK/pack-manifest.json" | head -1)"
[ -n "$ID" ] || { echo "could not read the pack id from $PACK/pack-manifest.json"; exit 1; }

echo "compiling $ID against data/pack/srd-5.1"
go run ./cmd/pack -in "data/pack/srd-5.1,$PACK" >/dev/null

TARBALL="$(mktemp)"
trap 'rm -f "$TARBALL"' EXIT
tar -czf "$TARBALL" -C "$PACK" .
scp -q "$TARBALL" "$HOST:/tmp/private-pack-$ID.tgz"

ssh "$HOST" "ID='$ID' PORT='$PORT' bash -s" <<'REMOTE'
set -euo pipefail
ROOT=/opt/easydnd/private-packs
DIR="$ROOT/$ID"
TGZ="/tmp/private-pack-$ID.tgz"

sudo install -d -o root -g easydnd -m 750 "$ROOT"
sudo rm -rf "$DIR.new"
sudo install -d -o root -g easydnd -m 750 "$DIR.new"
sudo tar -xzf "$TGZ" -C "$DIR.new" --no-same-owner
sudo chown -R root:easydnd "$DIR.new"
sudo chmod -R u=rwX,g=rX,o= "$DIR.new"
rm -f "$TGZ"

if ! sudo test -d "$DIR"; then
    sudo mv "$DIR.new" "$DIR"
    echo "installed $DIR -- first push, service NOT restarted."
    echo "add to /etc/easydnd/prod.env, then: sudo supervisorctl restart easydnd"
    echo "  EASYDND_PRIVATE_PACK_FILES=$DIR"
    exit 0
fi

healthy() {
    for _ in $(seq 1 15); do
        if curl -fsS -m 3 "http://127.0.0.1:$PORT/v1/version" >/dev/null 2>&1; then return 0; fi
        sleep 1
    done
    return 1
}

sudo rm -rf "$DIR.old"
sudo mv "$DIR" "$DIR.old"
sudo mv "$DIR.new" "$DIR"
sudo supervisorctl restart easydnd || true
if healthy; then
    sudo rm -rf "$DIR.old"
    echo "updated $DIR, service healthy"
    exit 0
fi

echo "service did not come back on the new pack -- restoring the previous copy"
sudo rm -rf "$DIR"
sudo mv "$DIR.old" "$DIR"
sudo supervisorctl restart easydnd || true
healthy && echo "restored" || echo "RESTORE FAILED -- see /var/log/easydnd/err.log"
exit 1
REMOTE
