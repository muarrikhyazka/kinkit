import { useCallback, useSyncExternalStore } from 'react'

// Per-device navigation mode: 'sidebar' (default, every page in a list) or
// 'game' (walk Kiki around a town map). Kept in localStorage and broadcast
// with a window event so Layout, the Settings card and the map stay in step.
export const NAV_MODE_KEY = 'kinkit_nav_mode'
export const NAV_MODE_EVENT = 'kinkit:navmode'
export const NAV_MODES = ['sidebar', 'game']

// Last mode written in this page load; used only when storage is blocked.
let memory = 'sidebar'

export function readNavMode() {
  try {
    return localStorage.getItem(NAV_MODE_KEY) === 'game' ? 'game' : 'sidebar'
  } catch {
    return memory
  }
}

export function writeNavMode(mode) {
  const next = mode === 'game' ? 'game' : 'sidebar'
  memory = next
  try { localStorage.setItem(NAV_MODE_KEY, next) } catch { /* storage blocked: the event below still updates this tab */ }
  window.dispatchEvent(new CustomEvent(NAV_MODE_EVENT, { detail: next }))
}

function subscribe(cb) {
  window.addEventListener(NAV_MODE_EVENT, cb)
  window.addEventListener('storage', cb)
  return () => {
    window.removeEventListener(NAV_MODE_EVENT, cb)
    window.removeEventListener('storage', cb)
  }
}

/** [mode, setMode] with mode 'sidebar' | 'game'. */
export function useNavMode() {
  const mode = useSyncExternalStore(subscribe, readNavMode, () => 'sidebar')
  const setMode = useCallback(next => writeNavMode(next), [])
  return [mode, setMode]
}
