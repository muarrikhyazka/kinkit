import { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { useAuth } from '../AuthContext.jsx'
import Mascot from '../Mascot.jsx'
import { APP_ROUTES, navRoutes, routeTo } from '../routes.jsx'
import { IconGuide, IconChevronUp, IconChevronDown, IconChevronLeft, IconChevronRight } from '../icons.jsx'
import {
  COLS, ROWS, SPAWN, POND, TREES, ROAD_ROWS, ROAD_X, ROAD_COL,
  splitBuildings, buildGrid, isSolid, step, doorAhead, doorOf, belowDoor,
} from './worldMap.js'
import './game.css'

export const WORLD_POS_KEY = 'kinkit_world_pos'
const MOVE_GAP_MS = 120
const REPEAT_MS = 160
const TILE_MIN = 32
const TILE_MAX = 48
const MAP_MIN_H = 240

const KEY_DIRS = {
  ArrowUp: 'up', ArrowDown: 'down', ArrowLeft: 'left', ArrowRight: 'right',
  w: 'up', s: 'down', a: 'left', d: 'right',
}
const PAD = [
  { dir: 'up', label: 'Move up', Icon: IconChevronUp },
  { dir: 'left', label: 'Move left', Icon: IconChevronLeft },
  { dir: 'right', label: 'Move right', Icon: IconChevronRight },
  { dir: 'down', label: 'Move down', Icon: IconChevronDown },
]

function readSavedPos() {
  try {
    const raw = JSON.parse(sessionStorage.getItem(WORLD_POS_KEY) || 'null')
    if (raw && Number.isInteger(raw.x) && Number.isInteger(raw.y)) return { x: raw.x, y: raw.y }
  } catch { /* storage blocked or malformed: start at the spawn tile */ }
  return null
}

// Keys must never walk Kiki while the person is typing, holding a modifier, or has a dialog open.
function keyIsBlocked(e) {
  if (e.ctrlKey || e.metaKey || e.altKey) return true
  const t = e.target
  if (t && (t.isContentEditable || /^(INPUT|TEXTAREA|SELECT)$/.test(t.tagName))) return true
  return !!document.querySelector('[role="dialog"]')
}
const onControl = t => !!(t && t.closest && t.closest('a[href], button, summary, [role="button"]'))

export default function GameWorld() {
  const { user } = useAuth()
  const navigate = useNavigate()

  const { buildings, plots } = useMemo(() => {
    const allowed = navRoutes('tenant', user).map(r => r.path)
    const { visible, plots: hidden } = splitBuildings(allowed)
    const decorate = b => {
      if (b.path === 'docs') return { ...b, label: 'Help', Icon: IconGuide, to: '/docs' }
      const r = APP_ROUTES.find(x => x.path === b.path)
      return { ...b, label: r ? r.label : b.path, Icon: r ? r.Icon : IconGuide, to: r ? routeTo(r) : '/' + b.path }
    }
    return { buildings: visible.map(decorate), plots: hidden }
  }, [user])
  const grid = useMemo(() => buildGrid(buildings, plots), [buildings, plots])

  const [pos, setPos] = useState(() => {
    const saved = readSavedPos()
    return saved && !isSolid(buildGrid(buildings, plots), saved.x, saved.y) ? saved : SPAWN
  })
  const [facing, setFacing] = useState(() => (readSavedPos() ? 'up' : 'down'))
  const world = useRef({ pos, facing, grid, buildings })
  world.current = { pos, facing, grid, buildings }
  const lastKeyMove = useRef(0)
  const holdTimer = useRef(null)

  const stageRef = useRef(null)
  const padRef = useRef(null)
  const [stageW, setStageW] = useState(0)
  const [room, setRoom] = useState(null) // height left for the map once the pad and the page padding are accounted for
  const [animated, setAnimated] = useState(false) // no glide on the first paint, only on later moves and resizes
  useLayoutEffect(() => {
    const el = stageRef.current
    const pad = padRef.current
    if (!el || !pad) return undefined
    const measure = () => {
      setStageW(el.clientWidth)
      const top = el.getBoundingClientRect().top + window.scrollY
      const padBlock = pad.offsetHeight + (parseFloat(getComputedStyle(pad).marginTop) || 0)
      const page = el.closest('main')
      const below = page ? parseFloat(getComputedStyle(page).paddingBottom) || 0 : 0
      setRoom(Math.floor(window.innerHeight - top - padBlock - below))
    }
    measure()
    const raf = requestAnimationFrame(() => setAnimated(true))
    // the HUD and heading above the map change height with the viewport, so watch the page, not just the stage
    const ro = typeof ResizeObserver === 'function' ? new ResizeObserver(measure) : null
    ro?.observe(el)
    if (el.closest('main')) ro?.observe(el.closest('main'))
    window.addEventListener('resize', measure)
    window.addEventListener('orientationchange', measure)
    return () => {
      cancelAnimationFrame(raf); ro?.disconnect()
      window.removeEventListener('resize', measure); window.removeEventListener('orientationchange', measure)
    }
  }, [])
  const T = Math.max(TILE_MIN, Math.min(TILE_MAX, Math.floor((stageW || COLS * TILE_MIN) / COLS)))
  const worldW = COLS * T
  const worldH = ROWS * T
  const viewW = Math.min(worldW, stageW || worldW)
  // On very short screens MAP_MIN_H wins over the available room; the pad is sticky then, so it stays reachable.
  const viewH = Math.min(worldH, Math.max(MAP_MIN_H, room ?? MAP_MIN_H))
  const camX = Math.max(0, Math.min(worldW - viewW, (pos.x + 0.5) * T - viewW / 2))
  const camY = Math.max(0, Math.min(worldH - viewH, (pos.y + 0.5) * T - viewH / 2))

  const go = useCallback(b => {
    try { sessionStorage.setItem(WORLD_POS_KEY, JSON.stringify(belowDoor(b))) } catch { /* storage blocked: next visit starts at the spawn tile */ }
    navigate(b.to)
  }, [navigate])

  const move = useCallback(dir => {
    const w = world.current
    const r = step(w.pos, dir, w.grid, w.buildings)
    if (r.enter) { go(r.enter); return }
    world.current = { ...w, facing: r.facing, pos: r.pos }
    setFacing(r.facing)
    if (r.pos !== w.pos) setPos(r.pos)
  }, [go])

  useEffect(() => {
    const onKey = e => {
      if (keyIsBlocked(e)) return
      const dir = KEY_DIRS[e.key] || KEY_DIRS[e.key.length === 1 ? e.key.toLowerCase() : '']
      if (dir) {
        e.preventDefault()
        const now = performance.now()
        if (now - lastKeyMove.current < MOVE_GAP_MS) return
        lastKeyMove.current = now
        move(dir)
        return
      }
      if ((e.key === 'Enter' || e.key === ' ') && !onControl(e.target)) {
        const b = doorAhead(world.current.pos, world.current.facing, world.current.buildings)
        if (b) { e.preventDefault(); go(b) }
      }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [move, go])

  const stopHold = useCallback(() => {
    if (holdTimer.current) { clearInterval(holdTimer.current); holdTimer.current = null }
  }, [])
  useEffect(() => {
    window.addEventListener('blur', stopHold)
    return () => { window.removeEventListener('blur', stopHold); stopHold() }
  }, [stopHold])
  const startHold = dir => e => {
    if (e.pointerType === 'mouse' && e.button !== 0) return
    stopHold()
    move(dir)
    holdTimer.current = setInterval(() => move(dir), REPEAT_MS)
  }

  // Tabbing to a building walks Kiki to its door so the camera follows keyboard focus.
  const onBuildingFocus = b => e => {
    let kbd = false
    try { kbd = e.currentTarget.matches(':focus-visible') } catch { /* very old browser: skip */ }
    if (!kbd) return
    const p = belowDoor(b)
    world.current = { ...world.current, pos: p, facing: 'up' }
    setPos(p)
    setFacing('up')
  }
  const onBuildingClick = b => () => {
    try { sessionStorage.setItem(WORLD_POS_KEY, JSON.stringify(belowDoor(b))) } catch { /* ignore */ }
  }

  const ahead = doorAhead(pos, facing, buildings)
  const S = T * 1.6 // Kiki is drawn larger than a tile; its feet sit on the tile

  return (
    <div className={`gw-page${animated ? ' gw-animated' : ''}`}>
      <div className="gw-head">
        <h1>Town map</h1>
        <p className="muted gw-hint">Arrow keys or the pad to walk. Walk into a door to open a page.</p>
        <p className="gw-status" role="status" aria-live="polite">{ahead ? `${ahead.label}: walk up or press Enter to open` : ''}</p>
      </div>

      <div className="gw-stage" ref={stageRef}>
        <div className="gw-viewport" style={{ width: viewW, height: viewH }} onScroll={e => { e.currentTarget.scrollLeft = 0; e.currentTarget.scrollTop = 0 }}>
          <div className="gw-world" style={{ width: worldW, height: worldH, transform: `translate(${-camX}px, ${-camY}px)` }}>
            <div aria-hidden="true">
              {ROAD_ROWS.filter(y => y !== 10).map(y => (
                <div key={`r${y}`} className="gw-road" style={{ left: ROAD_X[0] * T, top: y * T, width: (ROAD_X[1] - ROAD_X[0] + 1) * T, height: (y === 9 ? 2 : 1) * T }} />
              ))}
              <div className="gw-road" style={{ left: ROAD_COL.x * T, top: ROAD_COL.y0 * T, width: T, height: (ROAD_COL.y1 - ROAD_COL.y0 + 1) * T }} />
            </div>

            <div aria-hidden="true">
              {Array.from({ length: COLS * ROWS }, (_, i) => ({ x: i % COLS, y: Math.floor(i / COLS) }))
                .filter(t => t.x === 0 || t.y === 0 || t.x === COLS - 1 || t.y === ROWS - 1)
                .map(t => <span key={`b${t.x}-${t.y}`} className="gw-tree" style={{ left: t.x * T, top: t.y * T, width: T, height: T }} />)}
              {TREES.map(t => <span key={`t${t.x}-${t.y}`} className="gw-tree" style={{ left: t.x * T, top: t.y * T, width: T, height: T }} />)}
              <span className="gw-pond" style={{ left: POND.x * T, top: POND.y * T, width: POND.w * T, height: POND.h * T }} />
            </div>

            {plots.map(b => (
              <span key={b.path} className="gw-plot" aria-hidden="true" style={{ left: b.x * T, top: b.y * T, width: b.w * T, height: b.h * T }} />
            ))}

            {buildings.map(b => {
              const door = doorOf(b)
              const roofH = Math.round(T * (b.h <= 2 ? 0.55 : 0.75))
              const showIcon = b.w >= 3 && b.h >= 3
              const iconPx = Math.min(28, Math.round(T * 0.6))
              const doorH = Math.round(T * 0.75)
              return (
                <Link
                  key={b.path || 'today'}
                  to={b.to}
                  className="gw-building"
                  data-kind={b.kind}
                  aria-label={`Open ${b.label}`}
                  style={{ left: b.x * T, top: b.y * T, width: b.w * T, height: b.h * T }}
                  onFocus={onBuildingFocus(b)}
                  onClick={onBuildingClick(b)}
                >
                  <span className="gw-roof" style={{ height: roofH }} aria-hidden="true" />
                  <span className="gw-wall" style={{ top: roofH }} aria-hidden="true" />
                  <span className="gw-chip" style={{ top: roofH + 3 }} aria-hidden="true">{b.label}</span>
                  {showIcon && <b.Icon className="gw-icon" width={iconPx} height={iconPx} style={{ marginLeft: -iconPx / 2, bottom: doorH + 4 }} aria-hidden="true" />}
                  <span className="gw-door" style={{ left: (door.x - b.x) * T + T * 0.2, width: T * 0.6, height: doorH }} aria-hidden="true" />
                </Link>
              )
            })}

            <div className="gw-player" aria-hidden="true" style={{ width: T, height: T, transform: `translate(${pos.x * T}px, ${pos.y * T}px)` }}>
              <div className="gw-kiki" style={{ width: S, height: S, left: (T - S) / 2, top: T - 2 - S * 0.88, transform: facing === 'left' ? 'scaleX(-1)' : undefined }}>
                <Mascot pose="idle" size={S} animate={false} />
              </div>
            </div>
          </div>
        </div>
      </div>

      <div className="gw-pad" ref={padRef} role="group" aria-label="Move Kiki">
        {PAD.map(({ dir, label, Icon }) => (
          <button
            key={dir}
            type="button"
            className={`gw-pad-btn gw-pad-${dir}`}
            aria-label={label}
            title={label}
            onPointerDown={startHold(dir)}
            onPointerUp={stopHold}
            onPointerLeave={stopHold}
            onPointerCancel={stopHold}
            onContextMenu={e => e.preventDefault()}
            onClick={e => { if (e.detail === 0) move(dir) }}
          >
            <Icon width={24} height={24} aria-hidden="true" />
          </button>
        ))}
      </div>
    </div>
  )
}
