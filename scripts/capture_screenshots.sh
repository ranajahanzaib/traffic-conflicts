#!/usr/bin/env bash
# Capture high-resolution screenshots of the running Traffic Conflict Atlas
set -euo pipefail

CHROME="${CHROME_BIN:-/Applications/Google Chrome.app/Contents/MacOS/Google Chrome}"
APP_URL="${1:-http://127.0.0.1:8080}"
OUT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../docs/screenshots" && pwd)"

mkdir -p "$OUT_DIR"

if [ ! -x "$CHROME" ]; then
  echo "Error: Google Chrome binary not found at '$CHROME'." >&2
  exit 1
fi

routes=(
  "overview.png:"
  "replay-laboratory.png:#replay"
  "scenario-explorer.png:#explore"
  "analytics-distribution.png:#analytics"
  "conflict-relationships.png:#relationships"
  "data-quality.png:#quality"
  "datasets.png:#datasets"
  "pet-methodology.png:#methodology"
)

for item in "${routes[@]}"; do
  file="${item%%:*}"
  hash="${item##*:}"
  echo "Capturing $file ($hash)..."
  "$CHROME" --headless=old \
    --run-all-compositor-stages-before-draw \
    --virtual-time-budget=2500 \
    --disable-gpu \
    --window-size=1440,900 \
    --screenshot="$OUT_DIR/$file" \
    "$APP_URL/$hash" 2>/dev/null
done

echo "Successfully captured all screenshots into $OUT_DIR"
