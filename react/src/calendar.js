// Date + event helpers shared by the Calendar page and the Reminders calendar view.
// Dates are local wall-clock "YYYY-MM-DD" strings (the household's time zone).

export const pad = n => String(n).padStart(2, '0')
export const toISO = d => `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`
export const parseISO = s => { const [y, m, d] = s.split('-').map(Number); return new Date(y, m - 1, d) }
export const addDays = (iso, n) => { const d = parseISO(iso); d.setDate(d.getDate() + n); return toISO(d) }
export const addMonths = (iso, n) => { const d = parseISO(iso); d.setDate(1); d.setMonth(d.getMonth() + n); return toISO(d) }
/** Monday of the week containing iso. */
export const startOfWeek = iso => { const d = parseISO(iso); d.setDate(d.getDate() - ((d.getDay() + 6) % 7)); return toISO(d) }
export const daysBetween = (a, b) => Math.round((parseISO(b) - parseISO(a)) / 864e5)
export const WEEKDAY_SHORT = ['Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat', 'Sun']
/** Weekday numbers as stored by the API (0 = Sunday), in Monday-first display order. */
export const WEEKDAY_NUMS = [1, 2, 3, 4, 5, 6, 0]
export const monthTitle = iso => parseISO(iso).toLocaleDateString(undefined, { month: 'long', year: 'numeric' })
export const dayTitle = iso => parseISO(iso).toLocaleDateString(undefined, { weekday: 'long', day: 'numeric', month: 'long', year: 'numeric' })
export const dayShort = iso => parseISO(iso).toLocaleDateString(undefined, { weekday: 'short', day: 'numeric', month: 'short' })
export const hhmm = s => { const [h, m] = (s || '0:0').split(':').map(Number); return h * 60 + m }
export const fmtMin = m => `${pad(Math.floor(m / 60) % 24)}:${pad(m % 60)}`

/** The 6x7 (Mon-first) grid of ISO dates that shows the month containing iso. */
export function monthGridDays(iso) {
  const first = iso.slice(0, 8) + '01'
  const start = startOfWeek(first)
  return Array.from({ length: 42 }, (_, i) => addDays(start, i))
}

/** Local date + minutes-of-day of an instant in an IANA time zone. */
export function zoned(iso, tz) {
  const parts = new Intl.DateTimeFormat('en-CA', {
    timeZone: tz || undefined, year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', hourCycle: 'h23',
  }).formatToParts(new Date(iso))
  const g = t => parts.find(p => p.type === t).value
  return { date: `${g('year')}-${g('month')}-${g('day')}`, minutes: Number(g('hour')) * 60 + Number(g('minute')) }
}

/** Palette keys accepted by the API; css colors are defined in calendar.css (--cal-<key>). */
export const ACTIVITY_COLORS = ['indigo', 'rose', 'amber', 'emerald', 'sky', 'violet', 'slate']

/** Turn a GET /api/calendar response into flat, drawable items. */
export function toItems(cal, { activities = true, reminders = true } = {}) {
  const items = []
  if (activities) {
    for (const a of cal?.activities || []) {
      const multi = a.occurs_on !== a.occurs_until
      items.push({
        key: `a${a.id}-${a.occurs_on}`, kind: 'activity', title: a.title, color: a.color, date: a.occurs_on, endDate: a.occurs_until,
        allDay: a.all_day || multi, startMin: hhmm(a.start_time), endMin: hhmm(a.end_time), raw: a,
      })
    }
  }
  if (reminders) {
    for (const o of cal?.reminders || []) {
      const eff = o.status === 'snoozed' && o.snoozed_until ? o.snoozed_until : o.due_at
      const z = zoned(eff, cal.timezone)
      items.push({
        key: `r${o.id}`, kind: 'reminder', title: o.title, color: 'reminder', date: z.date, endDate: z.date, allDay: false,
        startMin: z.minutes, endMin: Math.min(z.minutes + 30, 24 * 60), done: o.status === 'done', raw: o,
      })
    }
  }
  return items
}

export const itemsOnDay = (items, iso) => items
  .filter(i => i.date <= iso && iso <= i.endDate)
  .sort((a, b) => (a.allDay === b.allDay ? a.startMin - b.startMin : a.allDay ? -1 : 1))

export function itemTime(i, day) {
  if (i.kind === 'activity' && i.allDay) {
    if (i.date !== i.endDate && i.raw.all_day === false) return day === i.date ? `from ${i.raw.start_time}` : day === i.endDate ? `until ${i.raw.end_time}` : 'All day'
    return 'All day'
  }
  return i.kind === 'activity' ? `${fmtMin(i.startMin)}–${fmtMin(i.endMin)}` : fmtMin(i.startMin)
}

/** Side-by-side lanes for overlapping timed items on one day: [{item, lane, lanes}]. */
export function layoutDay(timed) {
  const sorted = [...timed].sort((a, b) => a.startMin - b.startMin || b.endMin - a.endMin)
  const out = []
  let cluster = []
  let clusterEnd = -1
  const flush = () => {
    const lanesEnd = []
    const placed = cluster.map(item => {
      let lane = lanesEnd.findIndex(e => e <= item.startMin)
      if (lane < 0) { lane = lanesEnd.length; lanesEnd.push(0) }
      lanesEnd[lane] = Math.max(item.endMin, item.startMin + 20)
      return { item, lane }
    })
    placed.forEach(p => out.push({ ...p, lanes: lanesEnd.length }))
    cluster = []
  }
  for (const item of sorted) {
    if (cluster.length && item.startMin >= clusterEnd) { flush(); clusterEnd = -1 }
    cluster.push(item)
    clusterEnd = Math.max(clusterEnd, item.endMin, item.startMin + 20)
  }
  if (cluster.length) flush()
  return out
}
