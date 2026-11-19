// Colours for widget boards: per-widget fill and whole-board background (#rrggbb, lower-case).
// '' means "default theme look". Mirrors mobile/lib/widget_colors.dart and the TV viewers.

export const PALETTE = [
  { name: 'Default', hex: '' },
  { name: 'Lavender', hex: '#e0e7ff' },
  { name: 'Sky', hex: '#bae6fd' },
  { name: 'Mint', hex: '#bbf7d0' },
  { name: 'Sun', hex: '#fde68a' },
  { name: 'Peach', hex: '#fed7aa' },
  { name: 'Rose', hex: '#fecdd3' },
  { name: 'Indigo', hex: '#4f46e5' },
  { name: 'Teal', hex: '#0f766e' },
  { name: 'Slate', hex: '#1e293b' },
]

const DARK_TEXT = '#1b1f2b'
const LIGHT_TEXT = '#ffffff'

export function normalizeHex(v) {
  const s = String(v || '').trim().toLowerCase()
  if (/^#[0-9a-f]{6}$/.test(s)) return s
  if (/^#[0-9a-f]{3}$/.test(s)) return '#' + s[1] + s[1] + s[2] + s[2] + s[3] + s[3]
  return ''
}

export function parseHex(hex) {
  const h = normalizeHex(hex)
  if (!h) return null
  return [parseInt(h.slice(1, 3), 16), parseInt(h.slice(3, 5), 16), parseInt(h.slice(5, 7), 16)]
}

function toHex(rgb) {
  return '#' + rgb.map(n => Math.max(0, Math.min(255, Math.round(n))).toString(16).padStart(2, '0')).join('')
}

/** WCAG relative luminance, 0 (black) .. 1 (white). */
export function luminance(hex) {
  const rgb = parseHex(hex)
  if (!rgb) return 1
  const [r, g, b] = rgb.map(v => { const c = v / 255; return c <= 0.03928 ? c / 12.92 : Math.pow((c + 0.055) / 1.055, 2.4) })
  return 0.2126 * r + 0.7152 * g + 0.0722 * b
}

/** Text colour (white or near-black) with the better contrast on `hex`. */
export function readableOn(hex) {
  const l = luminance(hex)
  const darkContrast = (l + 0.05) / (luminance(DARK_TEXT) + 0.05)
  const lightContrast = 1.05 / (l + 0.05)
  return darkContrast >= lightContrast ? DARK_TEXT : LIGHT_TEXT
}

/** Blend `a` towards `b` by t (0..1). */
export function mix(a, b, t) {
  const x = parseHex(a), y = parseHex(b)
  if (!x || !y) return a
  return toHex(x.map((v, i) => v + (y[i] - v) * t))
}

/** CSS variables for a widget card drawn on `hex` ('' = none: the theme look applies). */
export function cardVars(hex) {
  const bg = normalizeHex(hex)
  if (!bg) return {}
  const fg = readableOn(bg)
  const lightBg = fg === DARK_TEXT
  return {
    '--w-bg': bg,
    '--w-fg': fg,
    '--w-muted': mix(fg, bg, 0.32),
    '--w-line': mix(bg, fg, 0.18),
    '--w-track': mix(bg, fg, 0.16),
    '--w-accent': fg,
    '--w-danger': lightBg ? '#b91c1c' : '#fecaca',
    '--w-border': mix(bg, fg, 0.14),
  }
}

/** Inline style for the area behind a board (page stage, full screen, TV). */
export function backdropStyle(hex) {
  const bg = normalizeHex(hex)
  if (!bg) return undefined
  const fg = readableOn(bg)
  return { background: bg, color: fg, '--stage-muted': mix(fg, bg, 0.32) }
}
