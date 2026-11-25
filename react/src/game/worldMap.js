// Pure town-map logic for game mode (no React, no DOM). Tiles are integer
// (x, y) cells; every building is a rectangle with its door on the bottom row,
// centred. The tile below a door must always be walkable (checked by
// worldMapSelfCheck below and by the unit check in the WP3 verification).

export const COLS = 24
export const ROWS = 16
export const SPAWN = { x: 12, y: 5 }

export const DIRS = {
  up: { dx: 0, dy: -1 },
  down: { dx: 0, dy: 1 },
  left: { dx: -1, dy: 0 },
  right: { dx: 1, dy: 0 },
}

// path is the route path in routes.jsx ('' = Today). kind names the building
// style and is also the plain-words description of what it is in the town.
export const BUILDINGS = [
  { path: 'calendar', kind: 'clock tower', x: 2, y: 1, w: 4, h: 3 },
  { path: '', kind: 'home', x: 9, y: 1, w: 6, h: 4 },
  { path: 'reminders', kind: 'post office', x: 18, y: 1, w: 4, h: 3 },
  { path: 'dashboard', kind: 'bank', x: 1, y: 6, w: 4, h: 3 },
  { path: 'transactions', kind: 'market', x: 6, y: 6, w: 4, h: 3 },
  { path: 'family', kind: 'stage', x: 10, y: 7, w: 4, h: 2 },
  { path: 'budget', kind: 'ledger office', x: 15, y: 6, w: 4, h: 3 },
  { path: 'savings', kind: 'vault', x: 20, y: 6, w: 3, h: 3 },
  { path: 'chores', kind: 'barn', x: 1, y: 11, w: 4, h: 3 },
  { path: 'lists', kind: 'store', x: 6, y: 11, w: 4, h: 3 },
  { path: 'meals', kind: 'kitchen', x: 14, y: 11, w: 4, h: 3 },
  { path: 'widgets', kind: 'workshop', x: 19, y: 11, w: 4, h: 3 },
  { path: 'settings', kind: 'shed', x: 10, y: 12, w: 2, h: 2 },
  { path: 'docs', kind: 'library', x: 12, y: 12, w: 2, h: 2, always: true },
]

// Decorative only (roads are walkable like everything that is not solid).
export const ROAD_ROWS = [5, 9, 10, 14]
export const ROAD_X = [1, 22]
export const ROAD_COL = { x: 12, y0: 5, y1: 14 }
export const POND = { x: 6, y: 1, w: 2, h: 2 }
export const TREES = [{ x: 16, y: 1 }, { x: 16, y: 3 }]

export const doorOf = b => ({ x: b.x + Math.floor(b.w / 2), y: b.y + b.h - 1 })
export const belowDoor = b => { const d = doorOf(b); return { x: d.x, y: d.y + 1 } }

const inBuilding = (b, x, y) => x >= b.x && x < b.x + b.w && y >= b.y && y < b.y + b.h
const isBorder = (x, y) => x === 0 || y === 0 || x === COLS - 1 || y === ROWS - 1

/**
 * Buildings the player can see: routes the role may open (pass the set of
 * route paths from navRoutes) plus the ones marked always (Help). The rest
 * become fenced empty plots so the town layout never shifts between roles.
 */
export function splitBuildings(allowedPaths) {
  const allow = new Set(allowedPaths)
  const visible = []
  const plots = []
  for (const b of BUILDINGS) (b.always || allow.has(b.path) ? visible : plots).push(b)
  return { visible, plots }
}

/**
 * grid[y][x] is true when the tile is solid. Solid: border trees, decorative
 * trees, the pond, every footprint of a visible building except its door tile,
 * and every tile of a plot (a plot has no door).
 */
export function buildGrid(visible, plots = BUILDINGS.filter(b => !visible.includes(b))) {
  const grid = Array.from({ length: ROWS }, (_, y) => Array.from({ length: COLS }, (_, x) => isBorder(x, y)))
  for (const t of TREES) grid[t.y][t.x] = true
  for (let y = POND.y; y < POND.y + POND.h; y++) for (let x = POND.x; x < POND.x + POND.w; x++) grid[y][x] = true
  for (const b of [...visible, ...plots]) {
    const door = visible.includes(b) ? doorOf(b) : null
    for (let y = b.y; y < b.y + b.h; y++) {
      for (let x = b.x; x < b.x + b.w; x++) {
        if (door && x === door.x && y === door.y) continue
        grid[y][x] = true
      }
    }
  }
  return grid
}

export const isSolid = (grid, x, y) => x < 0 || y < 0 || x >= COLS || y >= ROWS || grid[y][x]

/** The visible building whose door tile is (x, y), or null. */
export function doorAt(x, y, buildings) {
  for (const b of buildings) {
    const d = doorOf(b)
    if (d.x === x && d.y === y) return b
  }
  return null
}

/**
 * One step. Always updates facing; moves only when the target tile is free.
 * Moving onto a door tile moves there and reports the building in `enter`.
 */
export function step(pos, dir, grid, buildings) {
  const d = DIRS[dir]
  if (!d) return { pos, facing: 'down', enter: null }
  const nx = pos.x + d.dx
  const ny = pos.y + d.dy
  if (isSolid(grid, nx, ny)) return { pos, facing: dir, enter: null }
  return { pos: { x: nx, y: ny }, facing: dir, enter: doorAt(nx, ny, buildings) }
}

/** The building whose door is the tile directly in front of the player, or null. */
export function doorAhead(pos, facing, buildings) {
  const d = DIRS[facing]
  return d ? doorAt(pos.x + d.dx, pos.y + d.dy, buildings) : null
}

/** Problems with the layout (empty array when it is sound). Used by the check script. */
export function worldMapSelfCheck() {
  const problems = []
  const grid = buildGrid(BUILDINGS, [])
  for (const b of BUILDINGS) {
    const d = doorOf(b)
    const below = belowDoor(b)
    if (isSolid(grid, below.x, below.y)) problems.push(`${b.path || '/'}: tile below the door (${below.x},${below.y}) is not walkable`)
    if (d.x < b.x || d.x >= b.x + b.w || d.y !== b.y + b.h - 1) problems.push(`${b.path || '/'}: door is outside its footprint`)
    if (b.x < 1 || b.y < 1 || b.x + b.w > COLS - 1 || b.y + b.h > ROWS - 1) problems.push(`${b.path || '/'}: footprint touches the border`)
    for (const o of BUILDINGS) {
      if (o === b || o.path < b.path) continue
      const overlap = b.x < o.x + o.w && o.x < b.x + b.w && b.y < o.y + o.h && o.y < b.y + b.h
      if (overlap) problems.push(`${b.path || '/'} overlaps ${o.path || '/'}`)
    }
    for (const t of TREES) if (inBuilding(b, t.x, t.y)) problems.push(`tree (${t.x},${t.y}) inside ${b.path || '/'}`)
    const pondHit = b.x < POND.x + POND.w && POND.x < b.x + b.w && b.y < POND.y + POND.h && POND.y < b.y + b.h
    if (pondHit) problems.push(`pond overlaps ${b.path || '/'}`)
  }
  if (isSolid(grid, SPAWN.x, SPAWN.y)) problems.push('spawn tile is not walkable')
  // every door tile must be reachable from the spawn (flood fill, doors count as walkable)
  const seen = new Set([`${SPAWN.x},${SPAWN.y}`])
  const queue = [SPAWN]
  while (queue.length) {
    const p = queue.shift()
    for (const d of Object.values(DIRS)) {
      const nx = p.x + d.dx
      const ny = p.y + d.dy
      const k = `${nx},${ny}`
      if (!seen.has(k) && !isSolid(grid, nx, ny)) { seen.add(k); queue.push({ x: nx, y: ny }) }
    }
  }
  for (const b of BUILDINGS) {
    const d = doorOf(b)
    if (!seen.has(`${d.x},${d.y}`)) problems.push(`${b.path || '/'}: door not reachable from the spawn`)
  }
  return problems
}
