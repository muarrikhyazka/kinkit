// Isometric "Kinkit household" scene for the login hero, rendered with three.js.
//
//   const scene = createLoginScene(canvas, { reducedMotion })  // -> { dispose } | null (no WebGL)
//
// Everything is built from simple primitives (boxes, cylinders, spheres, tori)
// with flat brand colours. All motion is a function of the loop phase
// (2*PI*t/LOOP_SECONDS) with integer multipliers, so the animation repeats
// seamlessly like a GIF. This module imports three.js, so it is loaded with a
// dynamic import() from Login.jsx and ends up in its own chunk.
import {
  WebGLRenderer, Scene, OrthographicCamera, Group, Mesh,
  BoxGeometry, CylinderGeometry, SphereGeometry, TorusGeometry, ConeGeometry, OctahedronGeometry,
  ExtrudeGeometry, Shape, PlaneGeometry, CanvasTexture,
  MeshStandardMaterial, MeshBasicMaterial, DirectionalLight, HemisphereLight,
  PCFSoftShadowMap, SRGBColorSpace,
} from 'three'

const LOOP_SECONDS = 12
const TAU = Math.PI * 2

// Brand palette (react/src/index.css)
const C = {
  purple: 0x4f46e5, purpleDark: 0x4338ca, purpleDeep: 0x312e81, indigo: 0x818cf8, indigoLight: 0xc7d2fe,
  tile: 0xdfe5ff, white: 0xf8f9ff, ink: 0x1e1b4b,
  pink: 0xf472b6, pinkLight: 0xf9a8d4, amber: 0xfbbf24, amberDark: 0xf59e0b, teal: 0x14b8a6, tealDark: 0x0f766e,
  green: 0x34d399, red: 0xfb7185,
}

const frac = x => x - Math.floor(x)
const smooth = (a, b, x) => { const t = Math.min(1, Math.max(0, (x - a) / (b - a))); return t * t * (3 - 2 * t) }

export function createLoginScene(canvas, { reducedMotion = false } = {}) {
  let renderer
  try {
    renderer = new WebGLRenderer({ canvas, antialias: true, alpha: true, powerPreference: 'low-power' })
  } catch {
    return null // WebGL unavailable: the page keeps its gradient hero
  }
  renderer.setClearColor(0x000000, 0)
  renderer.outputColorSpace = SRGBColorSpace
  renderer.shadowMap.enabled = true
  renderer.shadowMap.type = PCFSoftShadowMap

  const scene = new Scene()
  const camera = new OrthographicCamera(-1, 1, 1, -1, -100, 100)
  // true isometric: rotate 45 degrees around Y, elevate atan(1/sqrt2) = 35.264 degrees
  const TARGET_Y = 1.9
  camera.position.set(20, 20 + TARGET_Y, 20)
  camera.lookAt(0, TARGET_Y, 0)

  // ---- lights ----
  scene.add(new HemisphereLight(0xffffff, 0x8b8ff0, 1.9))
  const sun = new DirectionalLight(0xffffff, 2.2)
  sun.position.set(-5, 12, 6)
  sun.castShadow = true
  sun.shadow.mapSize.set(1024, 1024)
  const sc = sun.shadow.camera
  sc.left = -10; sc.right = 10; sc.top = 10; sc.bottom = -10; sc.near = 1; sc.far = 40
  sun.shadow.bias = -0.0006
  sun.shadow.normalBias = 0.03
  sun.shadow.radius = 4
  scene.add(sun)

  // ---- helpers ----
  const mats = new Map()
  const mat = (color, o = {}) => {
    const key = `${color}|${o.rough ?? ''}|${o.metal ?? ''}|${o.emissive ?? ''}`
    if (!mats.has(key)) {
      mats.set(key, new MeshStandardMaterial({
        color, roughness: o.rough ?? 0.6, metalness: o.metal ?? 0.04, flatShading: !!o.flat,
        emissive: o.emissive ?? 0x000000, emissiveIntensity: o.emissive ? 0.6 : 0,
      }))
    }
    return mats.get(key)
  }
  const add = (parent, geo, material, x = 0, y = 0, z = 0, o = {}) => {
    const m = new Mesh(geo, material)
    m.position.set(x, y, z)
    if (o.rx) m.rotation.x = o.rx
    if (o.ry) m.rotation.y = o.ry
    if (o.rz) m.rotation.z = o.rz
    if (o.s) m.scale.set(...o.s)
    m.castShadow = o.cast !== false
    m.receiveShadow = o.receive !== false
    parent.add(m)
    return m
  }
  const box = (w, h, d) => new BoxGeometry(w, h, d)
  const cyl = (rt, rb, h, seg = 32) => new CylinderGeometry(rt, rb, h, seg)

  const world = new Group()
  scene.add(world)
  const tick = [] // per-frame animators: (phase, t) => void
  const bob = (obj, baseY, amp, k = 1, offset = 0) => tick.push(p => { obj.position.y = baseY + Math.sin(p * k + offset) * amp })

  // ---- floating island ----
  const island = new Group()
  world.add(island)
  const tileTop = mat(C.tile, { rough: 0.85 })
  const tileSide = mat(C.indigo)
  const tileSideDark = mat(C.purpleDark)
  add(island, box(9, 0.6, 9), [tileSide, tileSideDark, tileTop, tileSide, tileSide, tileSideDark], 0, -0.3, 0)
  const under = add(island, new CylinderGeometry(4.2 * Math.SQRT2, 1.5 * Math.SQRT2, 2.2, 4), mat(C.purpleDeep, { flat: true }), 0, -1.7, 0, { ry: Math.PI / 4 })
  under.receiveShadow = false
  // paving path from the door to the front edge
  add(island, box(1.1, 0.04, 3.6), mat(C.white, { rough: 0.9 }), -1.2, 0.02, 2.7, { cast: false })
  add(island, box(3.6, 0.04, 1.1), mat(C.white, { rough: 0.9 }), 0.9, 0.02, 3.4, { cast: false })
  bob(island, 0, 0.18, 1)

  // ---- the house ----
  const house = new Group()
  house.position.set(-1.2, 0, -1.2)
  island.add(house)
  add(house, box(3.2, 2.2, 3.0), mat(C.white, { rough: 0.8 }), 0, 1.1, 0)
  const roof = add(house, new ConeGeometry(2.95, 1.9, 4), mat(C.purpleDark, { flat: true, rough: 0.7 }), 0, 3.15, 0, { ry: Math.PI / 4, s: [1, 1, 0.94] })
  roof.castShadow = true
  add(house, box(0.55, 1.2, 0.55), mat(C.purpleDark), 0.85, 3.3, -0.5)
  add(house, box(0.7, 0.12, 0.7), mat(C.purpleDeep), 0.85, 3.95, -0.5)
  // door (+z face) and windows (+z and +x faces)
  add(house, box(0.85, 1.4, 0.12), mat(C.amberDark), -0.55, 0.7, 1.53)
  add(house, new SphereGeometry(0.07, 12, 12), mat(C.white), -0.3, 0.7, 1.62)
  const glow = mat(0xfde68a, { emissive: 0xfbbf24 })
  const frameM = mat(C.indigoLight)
  add(house, box(0.9, 0.9, 0.1), frameM, 0.75, 1.3, 1.51)
  add(house, box(0.7, 0.7, 0.12), glow, 0.75, 1.3, 1.53)
  add(house, box(0.1, 0.9, 0.9), frameM, 1.61, 1.3, 0.0)
  add(house, box(0.12, 0.7, 0.7), glow, 1.63, 1.3, 0.0)
  add(house, box(0.1, 0.9, 0.9), frameM, 1.61, 1.3, -0.95)
  add(house, box(0.12, 0.7, 0.7), glow, 1.63, 1.3, -0.95)
  tick.push(p => { glow.emissiveIntensity = 0.55 + 0.25 * Math.sin(p * 2) })
  // chimney smoke: three puffs rising and fading, repeating every loop/3
  const puffM = []
  for (let i = 0; i < 3; i++) {
    const m = new MeshBasicMaterial({ color: 0xffffff, transparent: true, opacity: 0.8, depthWrite: false })
    puffM.push(m)
    const puff = add(house, new SphereGeometry(0.3, 16, 16), m, 0.85, 4.1, -0.5, { cast: false, receive: false })
    tick.push(p => {
      const u = frac(p / TAU * 3 + i / 3)
      puff.position.set(0.85 + u * 0.7, 4.1 + u * 1.7, -0.5 - u * 0.4)
      puff.scale.setScalar(0.5 + u * 0.9)
      m.opacity = 0.75 * Math.sin(Math.PI * u) ** 1.2
    })
  }

  // ---- bushes ----
  const leaf = mat(C.teal, { rough: 0.8 })
  const leafDark = mat(C.tealDark, { rough: 0.8 })
  const bush = (x, z, r = 0.55) => {
    add(island, new SphereGeometry(r, 20, 16), leaf, x, r * 0.8, z)
    add(island, new SphereGeometry(r * 0.7, 20, 16), leafDark, x + r * 0.7, r * 0.55, z + r * 0.4)
  }
  bush(-3.7, 0.9); bush(-3.6, -2.7, 0.65); bush(0.9, -3.8); bush(3.7, -3.6, 0.5)

  // ---- coin stacks (money) ----
  const coinM = mat(C.amber, { metal: 0.35, rough: 0.35 })
  const coinInner = mat(C.amberDark, { metal: 0.35, rough: 0.4 })
  const coin = (parent, x, y, z, r = 0.55) => {
    const g = new Group()
    g.position.set(x, y, z)
    add(g, cyl(r, r, 0.16), coinM)
    add(g, cyl(r * 0.68, r * 0.68, 0.18), coinInner)
    parent.add(g)
    return g
  }
  const stack = (x, z, n) => { for (let i = 0; i < n; i++) coin(island, x + (i % 2) * 0.04, 0.08 + i * 0.16, z) }
  stack(3.4, 1.4, 5)
  stack(2.6, 2.3, 3)
  stack(3.5, 2.4, 2)
  // an upright coin spinning above the stacks
  const flyCoin = new Group()
  flyCoin.position.set(3.4, 2.7, 1.4)
  world.add(flyCoin)
  const flyFace = coin(flyCoin, 0, 0, 0, 0.5)
  flyFace.rotation.x = Math.PI / 2
  tick.push(p => { flyCoin.position.y = 2.7 + Math.sin(p) * 0.3 + island.position.y; flyCoin.rotation.y = p * 2 })

  // ---- piggy bank (savings) ----
  const pig = new Group()
  pig.position.set(0.5, 0.0, 3.5)
  island.add(pig)
  const pinkM = mat(C.pinkLight, { rough: 0.5 })
  const pinkD = mat(C.pink, { rough: 0.5 })
  add(pig, new SphereGeometry(0.85, 28, 20), pinkM, 0, 0.95, 0, { s: [1.15, 0.92, 0.92] })
  add(pig, cyl(0.3, 0.3, 0.3), pinkD, 1.1, 0.95, 0, { rz: Math.PI / 2 })
  add(pig, new SphereGeometry(0.05, 8, 8), mat(C.ink), 1.27, 1.03, 0.09)
  add(pig, new SphereGeometry(0.05, 8, 8), mat(C.ink), 1.27, 1.03, -0.09)
  add(pig, new ConeGeometry(0.2, 0.34, 4), pinkD, 0.6, 1.7, 0.4, { rz: -0.3, rx: 0.3 })
  add(pig, new ConeGeometry(0.2, 0.34, 4), pinkD, 0.6, 1.7, -0.4, { rz: -0.3, rx: -0.3 })
  for (const [lx, lz] of [[0.6, 0.4], [0.6, -0.4], [-0.5, 0.4], [-0.5, -0.4]]) add(pig, cyl(0.17, 0.2, 0.34, 14), pinkD, lx, 0.17, lz)
  add(pig, new TorusGeometry(0.17, 0.045, 8, 16), pinkD, -1.0, 1.1, 0, { ry: Math.PI / 2 })
  add(pig, box(0.5, 0.05, 0.1), mat(C.ink), 0, 1.84, 0, { cast: false })
  tick.push(p => { pig.rotation.y = Math.sin(p) * 0.18; pig.scale.y = 1 + Math.sin(p * 2) * 0.015 })
  // coin dropping into the slot, twice per loop
  const drop = coin(island, 0.5, 3.4, 3.5, 0.34)
  tick.push(p => {
    const u = frac(p / TAU * 2)
    drop.position.set(0.5, 1.85 + (1 - smooth(0, 0.8, u)) * 1.7, 3.5)
    drop.rotation.y = u * TAU * 2
    drop.scale.setScalar(u > 0.8 ? 0.001 : 1)
  })

  // ---- shopping basket (lists) ----
  const basket = new Group()
  basket.position.set(3.5, 0, -1.8)
  island.add(basket)
  add(basket, new CylinderGeometry(1.05, 0.78, 0.95, 4), mat(C.teal, { flat: true, rough: 0.7 }), 0, 0.48, 0, { ry: Math.PI / 4 })
  add(basket, new CylinderGeometry(1.1, 1.1, 0.1, 4), mat(C.tealDark, { flat: true }), 0, 0.97, 0, { ry: Math.PI / 4 })
  const handle = add(basket, new TorusGeometry(0.78, 0.06, 10, 28, Math.PI), mat(C.tealDark), 0, 0.98, 0, { ry: Math.PI / 4 })
  handle.castShadow = true
  const apple = add(basket, new SphereGeometry(0.32, 18, 14), mat(C.red), -0.3, 1.1, 0.15)
  const orange = add(basket, new SphereGeometry(0.3, 18, 14), mat(C.amberDark), 0.25, 1.1, 0.35)
  const carton = add(basket, box(0.4, 0.8, 0.4), mat(C.white), 0.2, 1.3, -0.25)
  add(carton, box(0.42, 0.18, 0.42), mat(C.purple), 0, 0.25, 0)
  const leek = add(basket, new CylinderGeometry(0.05, 0.12, 1.2, 8), mat(C.green), -0.45, 1.45, -0.2, { rz: -0.25 })
  bob(apple, 1.1, 0.06, 2, 0); bob(orange, 1.1, 0.06, 2, 2); bob(carton, 1.3, 0.05, 2, 4); bob(leek, 1.45, 0.04, 2, 1)
  tick.push(p => { basket.rotation.y = Math.sin(p + 1) * 0.1 })

  // ---- TV / widget screen ----
  const tv = new Group()
  tv.position.set(-2.4, 0, 3.7)
  tv.rotation.y = Math.PI / 4
  island.add(tv)
  add(tv, cyl(0.6, 0.7, 0.12), mat(C.ink), 0, 0.06, 0)
  add(tv, box(0.22, 0.7, 0.18), mat(C.ink), 0, 0.45, 0)
  add(tv, box(2.5, 1.55, 0.16), mat(C.ink), 0, 1.5, 0)
  const screenTex = makeScreenTexture()
  const screenM = new MeshBasicMaterial({ map: screenTex })
  add(tv, new PlaneGeometry(2.3, 1.35), screenM, 0, 1.5, 0.09, { cast: false, receive: false })
  const sweepM = new MeshBasicMaterial({ color: 0xffffff, transparent: true, opacity: 0.12, depthWrite: false })
  const sweep = add(tv, new PlaneGeometry(0.5, 1.35), sweepM, 0, 1.5, 0.095, { cast: false, receive: false })
  tick.push(p => { sweep.position.x = Math.sin(p) * 1.0; sweepM.opacity = 0.1 + 0.07 * Math.cos(p) })

  // ---- floating: calendar ----
  const calendar = new Group()
  calendar.position.set(2.4, 4.6, -3.6)
  world.add(calendar)
  const cal = new Group()
  calendar.add(cal)
  add(cal, box(1.9, 2.0, 0.22), mat(C.white, { rough: 0.5 }), 0, 0, 0)
  add(cal, box(1.9, 0.55, 0.26), mat(C.pink, { rough: 0.5 }), 0, 0.72, 0)
  add(cal, cyl(0.07, 0.07, 0.4, 10), mat(C.purpleDeep), -0.5, 1.02, 0, { rx: Math.PI / 2 })
  add(cal, cyl(0.07, 0.07, 0.4, 10), mat(C.purpleDeep), 0.5, 1.02, 0, { rx: Math.PI / 2 })
  const dayM = mat(C.indigoLight)
  for (let r = 0; r < 3; r++) for (let c = 0; c < 3; c++) {
    const hot = r === 1 && c === 1
    add(cal, box(0.32, 0.26, 0.06), hot ? mat(C.purple) : dayM, -0.52 + c * 0.52, 0.25 - r * 0.42, 0.13, { cast: false })
  }
  cal.rotation.y = Math.PI / 4
  tick.push(p => { calendar.position.y = 4.6 + Math.sin(p + 0.5) * 0.28; cal.rotation.y = Math.PI / 4 + Math.sin(p) * 0.22; cal.rotation.z = Math.sin(p * 2) * 0.03 })

  // ---- floating: checklist (chores) ----
  const check = new Group()
  check.position.set(-4.0, 4.3, 0.4)
  world.add(check)
  const sheet = new Group()
  check.add(sheet)
  add(sheet, box(1.8, 2.3, 0.2), mat(C.white, { rough: 0.5 }), 0, 0, 0)
  add(sheet, box(0.7, 0.28, 0.26), mat(C.amberDark), 0, 1.1, 0)
  for (let i = 0; i < 3; i++) {
    const y = 0.5 - i * 0.55
    add(sheet, box(0.34, 0.34, 0.06), i < 2 ? mat(C.green) : mat(C.indigoLight), -0.5, y, 0.12, { cast: false })
    add(sheet, box(0.85, 0.1, 0.05), mat(C.indigo), 0.25, y, 0.12, { cast: false })
  }
  sheet.rotation.y = Math.PI / 4
  tick.push(p => { check.position.y = 4.3 + Math.sin(p + 2.2) * 0.3; sheet.rotation.y = Math.PI / 4 + Math.sin(p + 1) * 0.22 })

  // ---- floating: medal (points & badges) ----
  const medal = new Group()
  medal.position.set(-5.4, 3.0, 3.4)
  world.add(medal)
  const spin = new Group()
  medal.add(spin)
  add(spin, cyl(0.85, 0.85, 0.18), mat(C.amber, { metal: 0.35, rough: 0.35 }), 0, 0, 0, { rx: Math.PI / 2 })
  add(spin, new TorusGeometry(0.85, 0.08, 12, 40), mat(C.amberDark, { metal: 0.35 }), 0, 0, 0)
  const star = starGeometry(0.55, 0.24)
  add(spin, star, mat(C.white, { rough: 0.4 }), 0, 0, 0.1, { cast: false })
  add(spin, star, mat(C.white, { rough: 0.4 }), 0, 0, -0.1 - 0.12, { ry: Math.PI, cast: false })
  const ribbon = new Group()
  medal.add(ribbon)
  add(ribbon, box(0.34, 1.0, 0.06), mat(C.purple), -0.22, 1.3, 0, { rz: 0.35 })
  add(ribbon, box(0.34, 1.0, 0.06), mat(C.pink), 0.22, 1.3, 0, { rz: -0.35 })
  ribbon.rotation.y = Math.PI / 4
  medal.scale.setScalar(0.9)
  tick.push(p => { medal.position.y = 3.0 + Math.sin(p + 3) * 0.3; spin.rotation.y = Math.PI / 4 + p })

  // ---- floating: clock (reminders) ----
  const clock = new Group()
  clock.position.set(-4.7, 1.9, -3.2)
  world.add(clock)
  const face = new Group()
  clock.add(face)
  face.rotation.y = Math.PI / 4
  add(face, cyl(1.0, 1.0, 0.22), mat(C.white, { rough: 0.5 }), 0, 0, 0, { rx: Math.PI / 2 })
  add(face, new TorusGeometry(1.0, 0.1, 12, 40), mat(C.purple), 0, 0, 0)
  for (let i = 0; i < 12; i++) {
    const a = i / 12 * TAU
    add(face, box(0.06, i % 3 ? 0.1 : 0.2, 0.05), mat(C.purpleDeep), Math.sin(a) * 0.78, Math.cos(a) * 0.78, 0.13, { rz: -a, cast: false })
  }
  const hand = (len, w, color, z) => {
    const g = new Group()
    g.position.z = z
    face.add(g)
    add(g, box(w, len, 0.04), mat(color), 0, len / 2 - 0.08, 0, { cast: false })
    return g
  }
  const hourHand = hand(0.5, 0.1, C.ink, 0.14)
  const minuteHand = hand(0.72, 0.07, C.ink, 0.16)
  const secondHand = hand(0.78, 0.035, C.pink, 0.18)
  add(face, cyl(0.07, 0.07, 0.06, 12), mat(C.pink), 0, 0, 0.19, { rx: Math.PI / 2, cast: false })
  tick.push(p => {
    clock.position.y = 1.9 + Math.sin(p + 4) * 0.28
    face.rotation.y = Math.PI / 4 + Math.sin(p + 2) * 0.2
    hourHand.rotation.z = -(p / 12) - 1.0
    minuteHand.rotation.z = -(p * 1) - 0.3 // one lap per loop
    secondHand.rotation.z = -(p * 3)
  })

  // ---- sparkles orbiting the island ----
  const sparkGeo = new OctahedronGeometry(0.18)
  const sparkCols = [C.pinkLight, C.amber, 0xffffff, C.indigoLight, C.green]
  for (let i = 0; i < 9; i++) {
    const m = add(world, sparkGeo, mat(sparkCols[i % sparkCols.length], { rough: 0.3 }), 0, 0, 0, { cast: false, receive: false })
    const r = 5.8 + (i % 3) * 0.7
    const a0 = (i / 9) * TAU
    const dir = i % 2 ? -1 : 1
    const y0 = 1.2 + ((i * 37) % 10) * 0.55
    tick.push(p => {
      const a = a0 + dir * p
      m.position.set(Math.cos(a) * r, y0 + Math.sin(p * 2 + i) * 0.35, Math.sin(a) * r)
      m.rotation.set(p * 2, p * 3, 0)
      m.scale.setScalar(0.8 + 0.5 * Math.sin(p * 2 + i * 1.7) ** 2)
    })
  }

  // ---- render loop ----
  let width = 0
  let height = 0
  const resize = () => {
    const w = canvas.clientWidth
    const h = canvas.clientHeight
    if (!w || !h) return
    width = w; height = h
    renderer.setPixelRatio(Math.min(window.devicePixelRatio || 1, 2))
    renderer.setSize(w, h, false)
    const aspect = w / h
    // keep the whole island + props in frame; the desktop hero reserves room for the headline above
    const compact = w < 560
    const viewH = compact ? Math.max(12.4, 14.5 / aspect) : Math.max(21, 15 / aspect)
    const shift = compact ? 0.4 : viewH * 0.035 // positive moves the scene down
    camera.left = -viewH * aspect / 2
    camera.right = viewH * aspect / 2
    camera.top = viewH / 2 + shift
    camera.bottom = -viewH / 2 + shift
    camera.updateProjectionMatrix()
    sun.shadow.mapSize.set(w < 500 ? 512 : 1024, w < 500 ? 512 : 1024)
    if (sun.shadow.map) { sun.shadow.map.dispose(); sun.shadow.map = null }
    draw(clockT)
  }

  let clockT = reducedMotion ? LOOP_SECONDS * 0.18 : 0
  const draw = t => {
    if (!width) return
    const p = (t / LOOP_SECONDS) * TAU
    for (const fn of tick) fn(p, t)
    renderer.render(scene, camera)
  }

  let raf = 0
  let last = 0
  let onScreen = true
  let disposed = false
  const running = () => !reducedMotion && onScreen && !document.hidden && !disposed
  const frame = now => {
    raf = 0
    if (!running()) { last = 0; return }
    if (last) clockT = (clockT + Math.min(0.1, (now - last) / 1000)) % LOOP_SECONDS
    last = now
    draw(clockT)
    raf = requestAnimationFrame(frame)
  }
  const kick = () => { if (!raf && running()) { last = 0; raf = requestAnimationFrame(frame) } }

  const ro = new ResizeObserver(() => { resize(); kick() })
  ro.observe(canvas)
  const io = new IntersectionObserver(entries => { onScreen = entries[entries.length - 1].isIntersecting; kick() })
  io.observe(canvas)
  const onVis = () => kick()
  document.addEventListener('visibilitychange', onVis)
  resize()
  kick()

  return {
    dispose() {
      disposed = true
      if (raf) cancelAnimationFrame(raf)
      ro.disconnect()
      io.disconnect()
      document.removeEventListener('visibilitychange', onVis)
      scene.traverse(o => {
        if (o.geometry) o.geometry.dispose()
        if (o.material) (Array.isArray(o.material) ? o.material : [o.material]).forEach(m => { m.map?.dispose(); m.dispose() })
      })
      mats.forEach(m => m.dispose())
      sun.shadow.map?.dispose()
      renderer.dispose()
      renderer.forceContextLoss()
    },
  }
}

// A flat 5-point star, extruded a little.
function starGeometry(outer, inner) {
  const s = new Shape()
  for (let i = 0; i < 10; i++) {
    const a = (i / 10) * TAU + Math.PI / 2
    const r = i % 2 ? inner : outer
    const x = Math.cos(a) * r
    const y = Math.sin(a) * r
    if (i === 0) s.moveTo(x, y); else s.lineTo(x, y)
  }
  s.closePath()
  return new ExtrudeGeometry(s, { depth: 0.06, bevelEnabled: false })
}

// Tiny dashboard drawn on a canvas, used as the TV screen.
function makeScreenTexture() {
  const c = document.createElement('canvas')
  c.width = 460; c.height = 270
  const g = c.getContext('2d')
  const bg = g.createLinearGradient(0, 0, 460, 270)
  bg.addColorStop(0, '#4f46e5'); bg.addColorStop(1, '#312e81')
  g.fillStyle = bg; g.fillRect(0, 0, 460, 270)
  g.fillStyle = 'rgba(255,255,255,.95)'; g.font = '700 26px system-ui, sans-serif'; g.fillText('Our home', 26, 46)
  g.fillStyle = 'rgba(255,255,255,.6)'; g.font = '500 16px system-ui, sans-serif'; g.fillText('Today', 26, 70)
  const card = (x, y, w, h, color) => { g.fillStyle = color; g.beginPath(); g.roundRect(x, y, w, h, 14); g.fill() }
  card(26, 92, 190, 150, 'rgba(255,255,255,.14)')
  card(236, 92, 198, 70, 'rgba(255,255,255,.14)')
  card(236, 172, 198, 70, 'rgba(255,255,255,.14)')
  const bars = [0.5, 0.8, 0.35, 0.95, 0.65, 0.75]
  bars.forEach((v, i) => { g.fillStyle = i === 3 ? '#fbbf24' : '#a5b4fc'; g.fillRect(44 + i * 28, 226 - v * 110, 18, v * 110) })
  g.strokeStyle = '#34d399'; g.lineWidth = 10; g.beginPath(); g.arc(280, 127, 22, -Math.PI / 2, Math.PI * 0.9); g.stroke()
  g.fillStyle = '#fff'; g.font = '700 22px system-ui, sans-serif'; g.fillText('72%', 322, 135)
  g.fillStyle = '#f472b6'; g.beginPath(); g.arc(262, 207, 11, 0, TAU); g.fill()
  g.fillStyle = 'rgba(255,255,255,.9)'; g.fillRect(286, 196, 120, 8); g.fillStyle = 'rgba(255,255,255,.45)'; g.fillRect(286, 212, 80, 8)
  const t = new CanvasTexture(c)
  t.colorSpace = SRGBColorSpace
  return t
}
