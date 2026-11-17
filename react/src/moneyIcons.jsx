// Icons for the recurring / income / goals pages, same style as icons.jsx
// (stroke-based, 24-unit viewBox, inherits currentColor).
const base = { width: 20, height: 20, viewBox: '0 0 24 24', fill: 'none', stroke: 'currentColor', strokeWidth: 1.8, strokeLinecap: 'round', strokeLinejoin: 'round' }

export const IconRepeat = (p) => (
  <svg {...base} {...p}><path d="M17 2l3 3-3 3" /><path d="M4 11V9a4 4 0 0 1 4-4h12" /><path d="M7 22l-3-3 3-3" /><path d="M20 13v2a4 4 0 0 1-4 4H4" /></svg>
)
export const IconIncome = (p) => (
  <svg {...base} {...p}><rect x="2" y="6" width="20" height="13" rx="2" /><path d="M12 9v7M9 13l3 3 3-3" /></svg>
)
export const IconGoal = (p) => (
  <svg {...base} {...p}><circle cx="12" cy="12" r="9" /><circle cx="12" cy="12" r="5" /><circle cx="12" cy="12" r="1" /></svg>
)
export const IconPause = (p) => (
  <svg {...base} {...p}><path d="M9 5v14M15 5v14" /></svg>
)
export const IconPlay = (p) => (
  <svg {...base} {...p}><path d="M7 4l13 8-13 8V4z" /></svg>
)
