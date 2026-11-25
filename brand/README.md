# Kinkit brand

Single source of truth for the logo. Every surface (web, phone app, TV apps, landing page) must use
these files or exact renders of them. Change the logo here first, then re-export everywhere.

- `kinkit-logo.svg`: app icon: WHITE rounded tile (16/64 corner radius, hairline #E0E7FF border so it
  stays visible on white) with a PURPLE (#4F46E5) house outline and purple dot.
- `kinkit-mark.svg`: the purple house mark alone (e.g. Android adaptive-icon foreground on a white
  background).
- Where the logo sits on purple (top bar, sidebar, TV banner) it is the white tile with the purple mark.
- Colours: purple #4F46E5 (primary), white #FFFFFF, hairline #E0E7FF.
- Mascot: see `mascot*.svg` (when present) and this file's mascot section.

## Mascot: Kiki

Kiki is a small house-spirit: a soft, rounded house-shaped body in Kinkit purple whose roof eaves
echo the logo's roof line, a white face patch, big glossy eyes, pink cheeks and stubby arms. The
3D look is pure SVG: a radial body gradient (#8B93FA highlight to #3730A3 edge), a clipped rim
light and ambient-occlusion gradient, a blurred specular highlight and a blurred contact shadow.
Amber (#FBBF24) is used only for the coin.

| File | Pose | Use for |
|---|---|---|
| `mascot.svg` | idle, waving | greetings (login, Today header) |
| `mascot-celebrate.svg` | arms up, holding a coin with a small house on it | level-ups, badges, approvals |
| `mascot-relax.svg` | eyes closed, arms on belly, "z z" | "all done" / empty-today states |
| `mascot-think.svg` | glancing up, hand to chin, thought dots | no data yet, errors, empty lists |

- ViewBox 256x256, transparent background; reads clearly from 48px to 320px.
- PNG renders (transparent): `react/public/mascot/<name>-512.png` and `<name>-256.png`
  (served on the web at `/mascot/<name>-512.png`), plus copies of the SVGs in `react/public/mascot/`.
- Web: `react/src/Mascot.jsx` renders the same markup inline (`<Mascot pose="idle|celebrate|relax|think" size animate wave title />`)
  and `react/src/mascot.css` animates it (breathe/bob with squash-stretch, blink, wave, celebrate hops,
  coin wobble); all motion is off under `prefers-reduced-motion`.
- SVG parts carry classes for animation on other platforms: `.mk-shadow`, `.mk-bob`, `.mk-body`,
  `.mk-eyes`, `.mk-mouth`, `.mk-arm-l`, `.mk-arm-r` (`.mk-wave` on the waving arm), `.mk-coin`,
  `.mk-shine`, `.mk-zzz`, `.mk-dots`.
- The SVG files and the `DEFS`/`POSES` block in `Mascot.jsx` hold identical markup; if you change
  the art, update both and re-render the PNGs (e.g. `rsvg-convert -w 512 -h 512 brand/mascot.svg -o react/public/mascot/mascot-512.png`).
- Keep it purposeful: use Kiki in a few moments (greeting, empty state, celebration), not on every page.
