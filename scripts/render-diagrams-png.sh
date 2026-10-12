#!/usr/bin/env bash
# Renders docs/diagrams/*.d2 -> report/figures/*.png via Chrome headless
# (real browser SVG+webfont rendering; d2's own PNG export needs a
# Playwright browser download this environment can't reach) then
# autocrops surrounding whitespace. Local report-build tooling only --
# not part of the Go system or its dependency policy.
set -euo pipefail
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

CHROME="/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
DIAGRAMS=(context container component dynamic state deployment)

mkdir -p report/figures

for d in "${DIAGRAMS[@]}"; do
  "$CHROME" --headless --disable-gpu \
    --screenshot="$ROOT_DIR/report/figures/$d.png" \
    --window-size=2400,2400 \
    --default-background-color=FFFFFFFF \
    "file://$ROOT_DIR/docs/figures/$d.svg" >/dev/null 2>&1
  echo "rendered $d"
done

python3 - "${DIAGRAMS[@]}" <<'EOF'
import sys
from PIL import Image, ImageChops

for name in sys.argv[1:]:
    path = f"report/figures/{name}.png"
    img = Image.open(path).convert("RGB")
    bg = Image.new("RGB", img.size, (255, 255, 255))
    diff = ImageChops.difference(img, bg)
    bbox = diff.getbbox()
    if bbox:
        pad = 20
        l, t, r, b = bbox
        l = max(0, l - pad)
        t = max(0, t - pad)
        r = min(img.width, r + pad)
        b = min(img.height, b + pad)
        img.crop((l, t, r, b)).save(path)
        print("cropped", name, (l, t, r, b))
    else:
        print("no content bbox found for", name)
EOF
