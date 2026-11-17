#!/usr/bin/env python3
"""Generate Kinkit's PWA icons into react/public (run from anywhere):

    python3 react/scripts/gen-icons.py

Draws a white "K" on the brand indigo, as SVG (favicon/icon.svg) and PNG
(192, 512, maskable 512, apple-touch 180). Needs Pillow for the PNGs.
"""
import os

PUBLIC = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "public")
BG_TOP, BG_BOTTOM, FG = (99, 102, 241), (79, 70, 229), (255, 255, 255)

# "K" in a 512x512 design space
BAR = [(150, 116), (218, 116), (218, 396), (150, 396)]
UPPER = [(218, 236), (218, 324), (378, 116), (290, 116)]
LOWER = [(230, 304), (280, 236), (384, 396), (300, 396)]
SHAPES = [BAR, UPPER, LOWER]


def svg(rounded=True):
    pts = lambda poly: " ".join(f"{x},{y}" for x, y in poly)
    polys = "\n  ".join(f'<polygon points="{pts(p)}" fill="#fff"/>' for p in SHAPES)
    rx = ' rx="112"' if rounded else ""
    return f"""<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 512 512">
  <defs><linearGradient id="g" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stop-color="#6366f1"/><stop offset="1" stop-color="#4f46e5"/></linearGradient></defs>
  <rect width="512" height="512"{rx} fill="url(#g)"/>
  {polys}
</svg>
"""


def png(size, path, maskable=False, rounded=True):
    from PIL import Image, ImageDraw

    ss = 4  # supersample for smooth edges
    S = size * ss
    img = Image.new("RGBA", (S, S), (0, 0, 0, 0))
    grad = Image.new("RGBA", (S, S))
    gd = ImageDraw.Draw(grad)
    for y in range(S):
        t = y / (S - 1)
        gd.line([(0, y), (S, y)], fill=tuple(round(a + (b - a) * t) for a, b in zip(BG_TOP, BG_BOTTOM)) + (255,))
    mask = Image.new("L", (S, S), 0)
    md = ImageDraw.Draw(mask)
    if rounded and not maskable:
        md.rounded_rectangle([0, 0, S - 1, S - 1], radius=int(S * 112 / 512), fill=255)
    else:
        md.rectangle([0, 0, S, S], fill=255)
    img.paste(grad, (0, 0), mask)
    d = ImageDraw.Draw(img)
    scale = 0.72 if maskable else 1.0  # maskable: keep the glyph inside the safe zone
    off = S * (1 - scale) / 2
    for poly in SHAPES:
        d.polygon([(off + x * S / 512 * scale, off + y * S / 512 * scale) for x, y in poly], fill=FG + (255,))
    img.resize((size, size), Image.LANCZOS).save(path, optimize=True)


def main():
    with open(os.path.join(PUBLIC, "icon.svg"), "w") as f:
        f.write(svg())
    with open(os.path.join(PUBLIC, "favicon.svg"), "w") as f:
        f.write(svg())
    png(192, os.path.join(PUBLIC, "icon-192.png"))
    png(512, os.path.join(PUBLIC, "icon-512.png"))
    png(512, os.path.join(PUBLIC, "icon-maskable-512.png"), maskable=True)
    png(180, os.path.join(PUBLIC, "apple-touch-icon.png"), rounded=False)
    print("icons written to", os.path.normpath(PUBLIC))


if __name__ == "__main__":
    main()
