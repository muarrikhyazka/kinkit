// Chart / category colours built from the brand palette (Persian Blue #16284C,
// Cream #EFE8DA, Amber #FFB92E) plus desaturated companions. Mid-tones on
// purpose so every swatch shows on both the cream (light) and navy (dark) cards.

// Deterministic color per category name (not main_category) - consistent across pages.
// Ordered so neighbours alternate warm / cool, keeping categories distinguishable.
const PALETTE = [
  '#FFB92E', // amber
  '#4A6FA5', // steel blue
  '#B5793A', // bronze
  '#8C9BB5', // slate
  '#C9A86A', // sand
  '#2E4A7A', // deep blue
  '#E0C48A', // wheat
  '#6F8F7A', // sage
  '#A65E5E', // muted rose
  '#7A9CC6', // sky
  '#D98E4A', // copper
  '#5E6E8C', // dusk
  '#9C8AA5', // mauve
]

export function categoryColor(name) {
  if (!name) return '#8C9BB5'
  let hash = 0
  for (let i = 0; i < name.length; i++) hash = (hash * 31 + name.charCodeAt(i)) >>> 0
  return PALETTE[hash % PALETTE.length]
}

const isDarkTheme = () => typeof document !== 'undefined' && document.documentElement.getAttribute('data-theme') === 'dark'

// Small, visually-distinct series palette (for comparing few series side by side, e.g. weeks/years)
// unlike categoryColor's hash-based pick, this guarantees high contrast between adjacent picks.
// The first series is navy on light themes and cream on dark (navy would vanish on a navy card).
const SERIES_PALETTE = ['#16284C', '#FFB92E', '#4A6FA5', '#B5793A', '#8C9BB5', '#6F8F7A', '#A65E5E', '#C9A86A']

export function seriesColor(index) {
  const i = index % SERIES_PALETTE.length
  if (i === 0 && isDarkTheme()) return '#EFE8DA'
  return SERIES_PALETTE[i]
}

/** Axis text / grid / slice-border colours for Chart.js, read from the theme tokens. */
export function chartTheme(onDark = false) {
  if (onDark) return { text: '#EFE8DA', grid: '#26396B', card: '#16284C' }
  const css = typeof document !== 'undefined' ? getComputedStyle(document.documentElement) : null
  const v = (name, fallback) => (css && css.getPropertyValue(name).trim()) || fallback
  return { text: v('--text', '#16284C'), grid: v('--border', '#DDD3C0'), card: v('--card', '#FBF8F1') }
}

/** Navy or white label text, whichever reads better on the given hex fill. */
export function readableOn(hex) {
  const m = /^#?([0-9a-f]{6})$/i.exec(hex || '')
  if (!m) return '#fff'
  const n = parseInt(m[1], 16)
  const lin = c => { c /= 255; return c <= 0.03928 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4 }
  const L = 0.2126 * lin(n >> 16) + 0.7152 * lin((n >> 8) & 255) + 0.0722 * lin(n & 255)
  // contrast vs white = 1.05/(L+.05); vs navy (L~0.021) = (L+.05)/0.071
  return 1.05 / (L + 0.05) >= (L + 0.05) / 0.071 ? '#fff' : '#16284C'
}
