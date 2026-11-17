import { createContext, useCallback, useContext, useEffect, useRef, useState } from 'react'
import { api, isRetryableError, notifySpendingChanged } from './api'

const OfflineContext = createContext(null)
const QUEUE_KEY = 'kinkit_offline_queue'
const FAILED_KEY = 'kinkit_offline_failed'
const LEGACY_QUEUE_KEY = 'accountamily_offline_queue' // pre-rename key, migrated once below

function readList(key) {
  try { return JSON.parse(localStorage.getItem(key) || '[]') } catch { return [] }
}
function writeList(key, list) {
  try { localStorage.setItem(key, JSON.stringify(list)) } catch { /* storage full/blocked */ }
}
const loadQueue = () => readList(QUEUE_KEY)
const saveQueue = q => writeList(QUEUE_KEY, q)

// One-time migration: move anything still sitting under the old app name.
;(function migrateLegacyQueue() {
  try {
    const legacy = localStorage.getItem(LEGACY_QUEUE_KEY)
    if (legacy === null) return
    const old = JSON.parse(legacy || '[]')
    const current = loadQueue()
    const seen = new Set(current.map(q => q.client_id))
    saveQueue([...current, ...(Array.isArray(old) ? old : []).filter(q => !seen.has(q.client_id))])
    localStorage.removeItem(LEGACY_QUEUE_KEY)
  } catch { /* corrupt legacy data - leave it alone */ }
})()

// Ask the service worker for a Background Sync ("kinkit-sync") so queued
// entries replay as soon as connectivity returns, even if this tab is in the
// background. Where unsupported, the 15s poll + "online" event cover it.
function requestBackgroundSync() {
  try {
    navigator.serviceWorker?.ready
      .then(reg => reg.sync?.register('kinkit-sync'))
      .catch(() => { /* not supported / denied */ })
  } catch { /* no service worker */ }
}

function uuid() {
  return crypto.randomUUID ? crypto.randomUUID() : `${Date.now()}-${Math.random().toString(16).slice(2)}`
}

export function OfflineProvider({ children }) {
  const [serverOnline, setServerOnline] = useState(true)
  const [queueSize, setQueueSize] = useState(() => loadQueue().length)
  const [failedItems, setFailedItems] = useState(() => readList(FAILED_KEY))
  const syncing = useRef(false)

  async function checkServer() {
    if (!navigator.onLine) { setServerOnline(false); return false }
    try {
      await api.ping()
      setServerOnline(true)
      return true
    } catch {
      setServerOnline(false)
      return false
    }
  }

  // Replays queued items in order. A 4xx (server rejected the data itself)
  // moves the item to the "failed" list and continues - retrying would never
  // succeed and would block everything behind it. Network errors / 5xx stop
  // the flush; it resumes on the next tick.
  async function flushQueue() {
    if (syncing.current) return
    syncing.current = true
    let synced = 0
    try {
      for (const item of loadQueue()) {
        try {
          await api.addSpending(item)
          synced++
        } catch (err) {
          if (isRetryableError(err)) break
          const failed = [...readList(FAILED_KEY), { ...item, error: err.message, failed_at: new Date().toISOString() }]
          writeList(FAILED_KEY, failed)
          setFailedItems(failed)
        }
        const rest = loadQueue().filter(q => q.client_id !== item.client_id)
        saveQueue(rest)
        setQueueSize(rest.length)
      }
    } finally {
      syncing.current = false
      if (synced > 0) notifySpendingChanged()
    }
  }

  useEffect(() => {
    let mounted = true
    async function tick() {
      const online = await checkServer()
      if (online && mounted) await flushQueue()
    }
    const onOffline = () => setServerOnline(false)
    // The queue persists in localStorage, so it survives reloads / app
    // restarts; replay it on start, on reconnect, when the app comes back to
    // the foreground and when the service worker's Background Sync fires.
    const onVisible = () => { if (document.visibilityState === 'visible' && loadQueue().length) tick() }
    const onSwMessage = e => { if (e.data?.type === 'kinkit-sync') tick() }
    tick()
    const interval = setInterval(tick, 15000)
    window.addEventListener('online', tick)
    window.addEventListener('offline', onOffline)
    document.addEventListener('visibilitychange', onVisible)
    navigator.serviceWorker?.addEventListener('message', onSwMessage)
    return () => {
      mounted = false
      clearInterval(interval)
      window.removeEventListener('online', tick)
      window.removeEventListener('offline', onOffline)
      document.removeEventListener('visibilitychange', onVisible)
      navigator.serviceWorker?.removeEventListener('message', onSwMessage)
    }
  }, [])

  // Try live; queue locally only when the server is unreachable or erroring
  // (network failure / 5xx). A 4xx is a real validation error - throw it so
  // the form can show it instead of queueing something that will never sync.
  async function addSpendingResilient(payload) {
    const withId = { ...payload, client_id: payload.client_id || uuid() }
    if (serverOnline) {
      try {
        return await api.addSpending(withId)
      } catch (err) {
        if (!isRetryableError(err)) throw err
        setServerOnline(false)
      }
    }
    const queue = loadQueue()
    queue.push(withId)
    saveQueue(queue)
    setQueueSize(queue.length)
    requestBackgroundSync()
    return { ok: true, queued: true }
  }

  // dismissFailed(client_id) drops one; dismissFailed() clears them all.
  const dismissFailed = useCallback((clientId) => {
    const next = clientId ? readList(FAILED_KEY).filter(f => f.client_id !== clientId) : []
    writeList(FAILED_KEY, next)
    setFailedItems(next)
  }, [])

  return (
    <OfflineContext.Provider value={{ serverOnline, queueSize, addSpendingResilient, failedItems, dismissFailed }}>
      {children}
    </OfflineContext.Provider>
  )
}

export function useOffline() {
  return useContext(OfflineContext)
}
