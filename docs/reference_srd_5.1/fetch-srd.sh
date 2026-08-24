#!/usr/bin/env bash
# Downloads the SRD 5.1 / 5e SRD sources evaluated in README.md into docs/reference_srd_5.1/data/.
# Re-run to refresh. Requires curl + python3.
set -euo pipefail

DEST="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/data"
mkdir -p "$DEST/5e-database-2014-en/schemas" "$DEST/cc-srd5"

DB_RAW="https://raw.githubusercontent.com/5e-bits/5e-database/main/src/2014"
API="https://api.github.com/repos/5e-bits/5e-database/contents/src/2014"

echo "==> 5e-bits/5e-database  src/2014/en  (structured JSON)"
curl -sSf "$API/en" \
  | python3 -c 'import sys,json;[print(f["name"]) for f in json.load(sys.stdin) if f["name"].endswith(".json")]' \
  | while read -r f; do
      echo "    $f"
      curl -sSf -o "$DEST/5e-database-2014-en/$f" "$DB_RAW/en/$f"
    done

echo "==> 5e-bits/5e-database  src/2014/schemas  (TypeScript/Mongoose schemas)"
curl -sSf "$API/schemas" \
  | python3 -c 'import sys,json;[print(f["name"]) for f in json.load(sys.stdin) if f["name"].endswith(".ts")]' \
  | while read -r f; do
      curl -sSf -o "$DEST/5e-database-2014-en/schemas/$f" "$DB_RAW/schemas/$f"
    done

# schemas/*.ts import from src/schemas/common.ts, one level up — grab it too.
curl -sSf -o "$DEST/5e-database-2014-en/schemas/_common.ts" \
  https://raw.githubusercontent.com/5e-bits/5e-database/main/src/schemas/common.ts

echo "==> gabrielrega/cc-srd5  (CC-BY-4.0 SRD 5.1 prose, Markdown)"
curl -sSf -o "$DEST/cc-srd5/cc-srd5.md"          https://raw.githubusercontent.com/gabrielrega/cc-srd5/main/cc-srd5.md
curl -sSf -o "$DEST/cc-srd5/changes-50-to-51.md" https://raw.githubusercontent.com/gabrielrega/cc-srd5/main/changes-50-to-51.md
curl -sSf -o "$DEST/cc-srd5/LICENSING.md"        https://raw.githubusercontent.com/gabrielrega/cc-srd5/main/licensing/LICENSING.md
curl -sSf -o "$DEST/cc-srd5/CC-BY-4.0.txt"       https://raw.githubusercontent.com/gabrielrega/cc-srd5/main/licensing/CC-BY-4.0.txt

echo "==> done. Tree:"
du -sh "$DEST"/* 2>/dev/null || true
