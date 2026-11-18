#!/usr/bin/env python3
"""Regenerate Kinkit's PWA icons in react/public from brand/ (run from anywhere):

    python3 react/scripts/gen-icons.py

- favicon.svg, icon.svg: copies of brand/kinkit-logo.svg (white tile, purple house).
- icon-192.png, icon-512.png: renders of the tile.
- apple-touch-icon.png (180): full-bleed white, purple mark centered (~62%).
- icon-maskable-512.png: full-bleed white, mark inside the safe zone (~55%).

Needs `rsvg-convert` on PATH (librsvg). Without it, use docker:
    docker run --rm -v "$PWD":/w -w /w alpine:3.20 sh -c \
      "apk add --no-cache rsvg-convert python3 >/dev/null && python3 react/scripts/gen-icons.py"
After changing icons, bump the ?v= cache-busters in react/index.html,
react/public/manifest.webmanifest and VERSION/SHELL in react/public/sw.js.
"""
import os
import re
import shutil
import subprocess
import tempfile

HERE = os.path.dirname(os.path.abspath(__file__))
PUBLIC = os.path.normpath(os.path.join(HERE, "..", "public"))
BRAND = os.path.normpath(os.path.join(HERE, "..", "..", "brand"))


def mark_inner():
    svg = open(os.path.join(BRAND, "kinkit-mark.svg")).read()
    return re.search(r"<svg[^>]*>(.*)</svg>", svg, re.S).group(1)


def full_bleed(scale):
    return (
        '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 64 64">'
        '<rect width="64" height="64" fill="#FFFFFF"/>'
        f'<g transform="translate(32 32) scale({scale}) translate(-32 -32)">{mark_inner()}</g></svg>'
    )


def render(src, size, out):
    subprocess.run(["rsvg-convert", "-w", str(size), "-h", str(size), src, "-o", out], check=True)


def main():
    logo = os.path.join(BRAND, "kinkit-logo.svg")
    for name in ("favicon.svg", "icon.svg"):
        shutil.copyfile(logo, os.path.join(PUBLIC, name))
    render(logo, 192, os.path.join(PUBLIC, "icon-192.png"))
    render(logo, 512, os.path.join(PUBLIC, "icon-512.png"))
    with tempfile.TemporaryDirectory() as tmp:
        for scale, size, name in ((0.97, 180, "apple-touch-icon.png"), (0.86, 512, "icon-maskable-512.png")):
            src = os.path.join(tmp, name + ".svg")
            with open(src, "w") as f:
                f.write(full_bleed(scale))
            render(src, size, os.path.join(PUBLIC, name))
    print("icons written to", PUBLIC)


if __name__ == "__main__":
    main()
