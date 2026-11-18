/* Kinkit service worker.
 *
 * - App shell (index.html, icons, manifest): precached; navigations are
 *   network-first with the cached shell as the offline fallback.
 * - /assets/* (Vite content-hashed): cache-first - a new deploy has new names.
 * - /api/* GET: network-first; the last good JSON answer is served when
 *   offline so pages still show data. Never cached: ping (offline detection),
 *   receipts, exports, TV data, anything marked no-store. The API cache is
 *   wiped on login/logout/signup so one user's data never shows for another.
 * - Writes are never intercepted: offline spending is queued by the app
 *   (OfflineContext, localStorage) and replayed when back online. The
 *   Background Sync "kinkit-sync" event just tells open tabs to replay now.
 *
 * Served with Cache-Control: no-cache (spa.go) so updates roll out on the
 * next load. Bump VERSION to force-refresh the shell cache.
 */
const VERSION = 'v2' // bump when shell files (icons, manifest) change
const SHELL_CACHE = `kinkit-shell-${VERSION}`
const ASSET_CACHE = 'kinkit-assets'
const API_CACHE = 'kinkit-api'
const KEEP = [SHELL_CACHE, ASSET_CACHE, API_CACHE]
const SHELL = ['/', '/manifest.webmanifest?v=2', '/favicon.svg?v=2', '/icon.svg?v=2', '/icon-192.png?v=2', '/icon-512.png?v=2']
const MAX_ASSETS = 80

self.addEventListener('install', event => {
  event.waitUntil(
    caches.open(SHELL_CACHE)
      .then(c => c.addAll(SHELL.map(u => new Request(u, { cache: 'reload' }))))
      .then(() => self.skipWaiting()),
  )
})

self.addEventListener('activate', event => {
  event.waitUntil(
    caches.keys()
      .then(keys => Promise.all(keys.filter(k => k.startsWith('kinkit-') && !KEEP.includes(k)).map(k => caches.delete(k))))
      .then(() => self.clients.claim()),
  )
})

const NO_CACHE_API = [/^\/api\/ping$/, /\/receipt$/, /^\/api\/export\//, /^\/api\/tv\//, /^\/api\/import\//, /^\/api\/(login|logout|signup)$/]
const SESSION_CHANGE = /^\/api\/(login|logout|signup)$/

async function trimCache(name, max) {
  const c = await caches.open(name)
  const keys = await c.keys()
  for (let i = 0; i < keys.length - max; i++) await c.delete(keys[i])
}

async function networkFirstApi(request) {
  const cache = await caches.open(API_CACHE)
  try {
    const res = await fetch(request)
    const cc = res.headers.get('Cache-Control') || ''
    const type = res.headers.get('Content-Type') || ''
    if (res.ok && type.includes('application/json') && !cc.includes('no-store')) {
      cache.put(request, res.clone())
    }
    return res
  } catch (err) {
    const hit = await cache.match(request)
    if (hit) return hit
    return new Response(JSON.stringify({ error: 'offline' }), { status: 503, headers: { 'Content-Type': 'application/json' } })
  }
}

async function navigation(request) {
  const cache = await caches.open(SHELL_CACHE)
  try {
    const res = await fetch(request)
    if (res.ok && (res.headers.get('Content-Type') || '').includes('text/html')) cache.put('/', res.clone())
    return res
  } catch {
    return (await cache.match('/')) || Response.error()
  }
}

async function cacheFirst(request) {
  const cache = await caches.open(ASSET_CACHE)
  const hit = await cache.match(request)
  if (hit) return hit
  const res = await fetch(request)
  if (res.ok) {
    await cache.put(request, res.clone())
    trimCache(ASSET_CACHE, MAX_ASSETS)
  }
  return res
}

async function staleWhileRevalidate(request) {
  const cache = await caches.open(SHELL_CACHE)
  const hit = await cache.match(request)
  const fresh = fetch(request).then(res => { if (res.ok) cache.put(request, res.clone()); return res }).catch(() => hit)
  return hit || fresh
}

self.addEventListener('fetch', event => {
  const { request } = event
  const url = new URL(request.url)
  if (url.origin !== self.location.origin) return // CDN (OCR library) etc.: browser default

  if (request.method !== 'GET') {
    if (SESSION_CHANGE.test(url.pathname)) event.waitUntil(caches.delete(API_CACHE))
    return
  }
  if (url.pathname.startsWith('/api/')) {
    if (NO_CACHE_API.some(re => re.test(url.pathname))) return
    event.respondWith(networkFirstApi(request))
    return
  }
  if (request.mode === 'navigate') {
    event.respondWith(navigation(request))
    return
  }
  if (url.pathname.startsWith('/assets/')) {
    event.respondWith(cacheFirst(request))
    return
  }
  if (url.pathname === '/sw.js') return
  event.respondWith(staleWhileRevalidate(request))
})

// Background Sync: the queue lives in the page's localStorage, so ask any
// open Kinkit tab to replay it.
self.addEventListener('sync', event => {
  if (event.tag !== 'kinkit-sync') return
  event.waitUntil(
    self.clients.matchAll({ type: 'window', includeUncontrolled: true })
      .then(list => list.forEach(c => c.postMessage({ type: 'kinkit-sync' }))),
  )
})

self.addEventListener('message', event => {
  if (event.data?.type === 'clear-api-cache') event.waitUntil(caches.delete(API_CACHE))
  if (event.data?.type === 'skip-waiting') self.skipWaiting()
})
