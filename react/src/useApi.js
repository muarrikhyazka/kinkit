import { useCallback, useEffect, useState, useSyncExternalStore } from 'react'
import { api } from './api'

/**
 * Tiny data-fetching hook.
 *   const { data, error, loading, reload, setData } = useApi(() => api.assets(), [])
 * - re-runs `fn` whenever `deps` change (or reload() is called)
 * - keeps the previous `data` while re-fetching (no flicker), so show a
 *   skeleton only when `loading && data === undefined`
 * - ignores responses from stale runs
 * - setData(valueOrUpdater) for local optimistic edits
 */
export function useApi(fn, deps = []) {
  const [state, setState] = useState({ data: undefined, error: '', loading: true })
  const [nonce, setNonce] = useState(0)

  useEffect(() => {
    let alive = true
    setState(s => (s.loading && !s.error ? s : { ...s, loading: true, error: '' }))
    Promise.resolve()
      .then(fn)
      .then(
        data => { if (alive) setState({ data, error: '', loading: false }) },
        err => { if (alive) setState(s => ({ ...s, error: err?.message || String(err), loading: false })) },
      )
    return () => { alive = false }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [...deps, nonce])

  const reload = useCallback(() => setNonce(n => n + 1), [])
  const setData = useCallback(
    updater => setState(s => ({ ...s, data: typeof updater === 'function' ? updater(s.data) : updater })),
    [],
  )
  return { ...state, reload, setData }
}

/** Debounced copy of a value (e.g. a search box) - avoids a request per keystroke. */
export function useDebounced(value, ms = 250) {
  const [v, setV] = useState(value)
  useEffect(() => {
    const t = setTimeout(() => setV(value), ms)
    return () => clearTimeout(t)
  }, [value, ms])
  return v
}

// ---------- shared, cached /api/setup (categories / accounts / income sources) ----------
// Fetched once and shared by every component that calls useSetup(). Call
// invalidateSetup() (or the hook's reload()) after editing master data, and
// clearSetupCache() on login/logout so another user's data never leaks.
let setupState = { data: null, error: '' }
let setupPromise = null
const listeners = new Set()

function emit(next) {
  setupState = next
  listeners.forEach(l => l())
}

function fetchSetup() {
  if (!setupPromise) {
    const p = api.setup().then(
      data => { if (setupPromise === p) emit({ data, error: '' }); return data },
      err => { if (setupPromise === p) { setupPromise = null; emit({ ...setupState, error: err.message }) } },
    )
    setupPromise = p
  }
  return setupPromise
}

export function invalidateSetup() {
  setupPromise = null
  return fetchSetup()
}

export function clearSetupCache() {
  setupPromise = null
  emit({ data: null, error: '' })
}

function subscribe(cb) {
  listeners.add(cb)
  return () => listeners.delete(cb)
}

export function useSetup() {
  const state = useSyncExternalStore(subscribe, () => setupState)
  useEffect(() => { if (!setupState.data && !setupPromise) fetchSetup() }, [])
  const data = state.data
  return {
    setup: data,
    categories: data?.categories || [],
    accounts: data?.accounts || [],
    incomeSources: data?.income_sources || [],
    error: state.error,
    loading: !data && !state.error,
    reload: invalidateSetup,
  }
}
