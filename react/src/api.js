const BASE = '/api'

async function req(path, opts = {}) {
  const res = await fetch(BASE + path, {
    credentials: 'include',
    headers: opts.body ? { 'Content-Type': 'application/json' } : undefined,
    ...opts,
  })
  let data = null
  try { data = await res.json() } catch { /* no body */ }
  if (!res.ok) {
    // `status` lets callers tell client errors (4xx: bad input, don't retry)
    // from server errors (5xx) and network failures (no status at all).
    const err = new Error(data?.error || `HTTP ${res.status}`)
    err.status = res.status
    err.data = data
    throw err
  }
  return data
}

// True for failures worth retrying later: network errors (fetch threw, so no
// status) and 5xx. 4xx means the request itself is wrong - retrying won't help.
export function isRetryableError(err) {
  return !err?.status || err.status >= 500
}

// Lightweight app-wide signal so any view showing spending (list, gamification
// badge, ...) can refresh after a save from anywhere (quick add, offline sync).
const SPENDING_CHANGED = 'kinkit:spending-changed'
export function notifySpendingChanged() {
  window.dispatchEvent(new Event(SPENDING_CHANGED))
}
export function onSpendingChanged(cb) {
  window.addEventListener(SPENDING_CHANGED, cb)
  return () => window.removeEventListener(SPENDING_CHANGED, cb)
}

export const api = {
  ping: () => req('/ping'),
  login: (username, password) => req('/login', { method: 'POST', body: JSON.stringify({ username, password }) }),
  signup: (email, password, display_name) => req('/signup', { method: 'POST', body: JSON.stringify({ email, password, display_name }) }),
  logout: () => req('/logout', { method: 'POST' }),
  me: () => req('/me'),
  updateSettings: (payload) => req('/me/settings', { method: 'PATCH', body: JSON.stringify(payload) }),
  currencies: () => req('/currencies'),
  fxRates: () => req('/fx-rates'),
  integrations: () => req('/integrations'),
  addIntegration: (payload) => req('/integrations', { method: 'POST', body: JSON.stringify(payload) }),
  updateIntegration: (id, payload) => req(`/integrations/${id}`, { method: 'PATCH', body: JSON.stringify(payload) }),
  deleteIntegration: (id) => req(`/integrations/${id}`, { method: 'DELETE' }),
  adminAccounts: () => req('/admin/accounts'),
  adminUpdateAccount: (id, payload) => req(`/admin/accounts/${id}`, { method: 'PATCH', body: JSON.stringify(payload) }),
  adminDeleteAccount: (id) => req(`/admin/accounts/${id}`, { method: 'DELETE' }),
  adminStats: () => req('/admin/stats'),
  adminResetDemoData: () => req('/admin/reset-demo-data', { method: 'POST' }),
  setup: () => req('/setup'),
  addCategory: (payload) => req('/categories', { method: 'POST', body: JSON.stringify(payload) }),
  updateCategory: (id, payload) => req(`/categories/${id}`, { method: 'PATCH', body: JSON.stringify(payload) }),
  deleteCategory: (id, force) => req(`/categories/${id}${force ? '?force=1' : ''}`, { method: 'DELETE' }),
  addAccount: (payload) => req('/accounts', { method: 'POST', body: JSON.stringify(payload) }),
  updateAccount: (id, payload) => req(`/accounts/${id}`, { method: 'PATCH', body: JSON.stringify(payload) }),
  deleteAccount: (id) => req(`/accounts/${id}`, { method: 'DELETE' }),
  addIncomeSource: (payload) => req('/income-sources', { method: 'POST', body: JSON.stringify(payload) }),
  updateIncomeSource: (id, payload) => req(`/income-sources/${id}`, { method: 'PATCH', body: JSON.stringify(payload) }),
  deleteIncomeSource: (id, force) => req(`/income-sources/${id}${force ? '?force=1' : ''}`, { method: 'DELETE' }),
  trend: (year, month, count = 6, valueMax = 'all') => req(`/trend?year=${year}&month=${month}&count=${count}${valueMax !== 'all' ? `&value_max=${valueMax}` : ''}`),
  budgeting: (year) => req(`/budgeting/${year}`),
  setBudgetIncome: (payload) => req('/budgeting/income', { method: 'POST', body: JSON.stringify(payload) }),
  setBudgetExpense: (payload) => req('/budgeting/expense', { method: 'POST', body: JSON.stringify(payload) }),
  spending: (year, month, page = 1, perPage = 20, opts = {}) => {
    const params = new URLSearchParams({ year, month, page, per_page: perPage })
    if (opts.search) params.set('search', opts.search)
    if (opts.category) params.set('category', opts.category)
    if (opts.sort) params.set('sort', opts.sort)
    if (opts.order) params.set('order', opts.order)
    if (opts.valueMax && opts.valueMax !== 'all') params.set('value_max', opts.valueMax)
    return req(`/spending?${params.toString()}`)
  },
  addSpending: (payload) => req('/spending', { method: 'POST', body: JSON.stringify(payload) }),
  updateSpending: (id, payload) => req(`/spending/${id}`, { method: 'PATCH', body: JSON.stringify(payload) }),
  deleteSpending: (id) => req(`/spending/${id}`, { method: 'DELETE' }),
  assets: () => req('/assets'),
  addAsset: (payload) => req('/assets', { method: 'POST', body: JSON.stringify(payload) }),
  updateAsset: (id, payload) => req(`/assets/${id}`, { method: 'PATCH', body: JSON.stringify(payload) }),
  deleteAsset: (id) => req(`/assets/${id}`, { method: 'DELETE' }),
  setAssetTarget: (payload) => req('/assets/target', { method: 'POST', body: JSON.stringify(payload) }),
  report: (year, month, valueMax = 'all') => req(`/report/${year}/${month}${valueMax !== 'all' ? `?value_max=${valueMax}` : ''}`),
  reportYear: (year, valueMax = 'all') => req(`/report/year/${year}${valueMax !== 'all' ? `?value_max=${valueMax}` : ''}`),
  reportMonthCompare: (year, month, valueMax = 'all') => req(`/report/month-compare/${year}/${month}${valueMax !== 'all' ? `?value_max=${valueMax}` : ''}`),
  patternsYear: (year, valueMax = 'all') => req(`/patterns/year/${year}${valueMax !== 'all' ? `?value_max=${valueMax}` : ''}`),
  weeklyReport: (year, month, valueMax = 'all') => req(`/report/weekly/${year}/${month}${valueMax !== 'all' ? `?value_max=${valueMax}` : ''}`),
  years: (valueMax = 'all') => req(`/years${valueMax !== 'all' ? `?value_max=${valueMax}` : ''}`),
  yearsByCategory: (valueMax = 'all') => req(`/years/by-category${valueMax !== 'all' ? `?value_max=${valueMax}` : ''}`),
  yearsMonthly: (valueMax = 'all') => req(`/years/monthly${valueMax !== 'all' ? `?value_max=${valueMax}` : ''}`),
  yearsDayOfMonthAvg: (valueMax = 'all') => req(`/years/day-of-month-avg${valueMax !== 'all' ? `?value_max=${valueMax}` : ''}`),
  yearsDatemonthAvg: (valueMax = 'all') => req(`/years/datemonth-avg${valueMax !== 'all' ? `?value_max=${valueMax}` : ''}`),
  summary: (year, month) => req(`/summary?year=${year}&month=${month}`),
  gamification: () => req('/gamification'),
  // --- family --- (household members / roles / allowance)
  family: () => req('/family'),
  updateHousehold: (payload) => req('/family', { method: 'PATCH', body: JSON.stringify(payload) }),
  addMember: (payload) => req('/family/members', { method: 'POST', body: JSON.stringify(payload) }),
  updateMember: (id, payload) => req(`/family/members/${id}`, { method: 'PATCH', body: JSON.stringify(payload) }),
  deleteMember: (id) => req(`/family/members/${id}`, { method: 'DELETE' }),
  memberAllowance: (id) => req(`/family/members/${id}/allowance`),
  changePassword: (payload) => req('/me/password', { method: 'POST', body: JSON.stringify(payload) }),
  // --- end family ---
  // --- money --- (recurring / income / goals / budget actuals / alerts)
  recurring: () => req('/recurring'),
  addRecurring: (payload) => req('/recurring', { method: 'POST', body: JSON.stringify(payload) }),
  updateRecurring: (id, payload) => req(`/recurring/${id}`, { method: 'PATCH', body: JSON.stringify(payload) }),
  deleteRecurring: (id) => req(`/recurring/${id}`, { method: 'DELETE' }),
  income: (year, month) => req(`/income?year=${year}${month ? `&month=${month}` : ''}`),
  addIncome: (payload) => req('/income', { method: 'POST', body: JSON.stringify(payload) }),
  updateIncome: (id, payload) => req(`/income/${id}`, { method: 'PATCH', body: JSON.stringify(payload) }),
  deleteIncome: (id) => req(`/income/${id}`, { method: 'DELETE' }),
  goals: () => req('/goals'),
  addGoal: (payload) => req('/goals', { method: 'POST', body: JSON.stringify(payload) }),
  updateGoal: (id, payload) => req(`/goals/${id}`, { method: 'PATCH', body: JSON.stringify(payload) }),
  deleteGoal: (id) => req(`/goals/${id}`, { method: 'DELETE' }),
  contributeGoal: (id, amount) => req(`/goals/${id}/contribute`, { method: 'POST', body: JSON.stringify({ amount }) }),
  budgetActuals: (year) => req(`/budgeting/${year}/actuals`),
  alertSettings: () => req('/alerts/settings'),
  updateAlertSettings: (payload) => req('/alerts/settings', { method: 'PATCH', body: JSON.stringify(payload) }),
  // --- end money ---
}

// Currency formatting WITH real FX conversion. Amounts are always stored in
// IDR; _fxRates (fetched once on app load, see fetchFxRates/AuthContext)
// holds { IDR: 1, USD: 0.000055, ... } from the backend's cached FX rates.
// formatRupiah converts IDR -> selected currency before formatting.
let _currency = { code: 'IDR', symbol: 'Rp', locale: 'id-ID' }
let _fxRates = { IDR: 1 }
const CURRENCY_META = {
  IDR: { symbol: 'Rp', locale: 'id-ID' }, USD: { symbol: '$', locale: 'en-US' },
  EUR: { symbol: '\u20ac', locale: 'de-DE' }, GBP: { symbol: '\u00a3', locale: 'en-GB' },
  SGD: { symbol: 'S$', locale: 'en-SG' }, MYR: { symbol: 'RM', locale: 'ms-MY' },
  JPY: { symbol: '\u00a5', locale: 'ja-JP' }, AUD: { symbol: 'A$', locale: 'en-AU' },
  INR: { symbol: '\u20b9', locale: 'en-IN' },
}
export function setCurrency(code) {
  const meta = CURRENCY_META[code] || CURRENCY_META.IDR
  _currency = { code: code || 'IDR', ...meta }
}
// Call once on app load (AuthContext) to populate the FX table used below.
export async function fetchFxRates() {
  try {
    const d = await api.fxRates()
    if (d?.rates) _fxRates = d.rates
  } catch { /* keep previous/default rates (IDR-only) on failure */ }
}
export function formatRupiah(n) {
  const idr = Number(n) || 0
  const rate = _fxRates[_currency.code] ?? (_currency.code === 'IDR' ? 1 : null)
  // ponytail: no rate cached/fetched yet (or code unknown) -> show as IDR rather than a wrong number
  const v = Math.round(rate != null ? idr * rate : idr)
  const symbol = rate != null ? _currency.symbol : CURRENCY_META.IDR.symbol
  const locale = rate != null ? _currency.locale : CURRENCY_META.IDR.locale
  return symbol + v.toLocaleString(locale)
}

export const MONTHS = ['Jan','Feb','Mar','Apr','May','Jun','Jul','Aug','Sep','Oct','Nov','Dec']
// "Today" in UTC as YYYY-MM-DD - computed at call time, never at module load.
export const todayISO = () => new Date().toISOString().slice(0, 10)

export const MONTH_NAMES = ['January','February','March','April','May','June','July','August','September','October','November','December']

// --- data --- (import/export, receipts, dashboard layout, TV share links)
// Multipart upload: let the browser set the multipart Content-Type/boundary.
async function upload(path, formData, method = 'POST') {
  const res = await fetch(BASE + path, { method, credentials: 'include', body: formData })
  let data = null
  try { data = await res.json() } catch { /* no body */ }
  if (!res.ok) {
    const err = new Error(data?.error || `HTTP ${res.status}`)
    err.status = res.status
    err.data = data
    throw err
  }
  return data
}
export const dataApi = {
  // format: 'csv' or 'xlsx'
  exportSpendingUrl: (from, to, format = 'csv') => {
    const p = new URLSearchParams()
    if (from) p.set('from', from)
    if (to) p.set('to', to)
    const qs = p.toString()
    return `${BASE}/export/spending.${format}${qs ? `?${qs}` : ''}`
  },
  // format: 'csv' or 'xlsx'
  exportIncomeUrl: (format = 'csv') => `${BASE}/export/income.${format}`,
  // format: 'json' or 'xlsx'
  exportAllUrl: (format = 'json') => `${BASE}/export/all.${format}`,
  importPresets: () => req('/import/presets'),
  // fields: { options, category_map, overrides } (objects, JSON-encoded here)
  importSpending: (file, fields = {}, dryRun = false) => {
    const fd = new FormData()
    fd.append('file', file)
    for (const [k, v] of Object.entries(fields)) if (v != null) fd.append(k, JSON.stringify(v))
    return upload(`/import/spending${dryRun ? '?dry_run=1' : ''}`, fd)
  },
  importBatches: () => req('/import/batches'),
  undoImport: (batch) => req(`/import/batches/${encodeURIComponent(batch)}`, { method: 'DELETE' }),
  receiptUrl: (id, bust = '') => `${BASE}/spending/${id}/receipt${bust ? `?v=${bust}` : ''}`,
  uploadReceipt: (id, file) => {
    const fd = new FormData()
    fd.append('file', file)
    return upload(`/spending/${id}/receipt`, fd)
  },
  deleteReceipt: (id) => req(`/spending/${id}/receipt`, { method: 'DELETE' }),
  dashboardLayout: () => req('/dashboard-layout'),
  saveDashboardLayout: (layout) => req('/dashboard-layout', { method: 'PUT', body: JSON.stringify({ layout }) }),
  tvTokens: () => req('/tv-tokens'),
  createTvToken: (name, board_id) => req('/tv-tokens', { method: 'POST', body: JSON.stringify({ name, board_id: board_id || null }) }),
  updateTvToken: (id, payload) => req(`/tv-tokens/${id}`, { method: 'PATCH', body: JSON.stringify(payload) }),
  revokeTvToken: (id) => req(`/tv-tokens/${id}`, { method: 'DELETE' }),
  tvData: (k) => req(`/tv/data?k=${encodeURIComponent(k)}`),
  spendingCount: () => req('/spending?per_page=1'),
}
// --- end data ---

// --- lists --- (shared shopping lists, staples, finish shopping)
const jsonBody = (method, body) => ({ method, body: JSON.stringify(body ?? {}) })
export const listsApi = {
  lists: (archived = false) => req(`/lists${archived ? '?archived=1' : ''}`),
  createList: (payload) => req('/lists', jsonBody('POST', payload)),
  list: (id) => req(`/lists/${id}`),
  updateList: (id, payload) => req(`/lists/${id}`, jsonBody('PATCH', payload)),
  deleteList: (id) => req(`/lists/${id}`, { method: 'DELETE' }),
  // payload: { text } (bulk "eggs, 2 milk") | { name, qty, note, category_id, est_price }
  addItems: (id, payload) => req(`/lists/${id}/items`, jsonBody('POST', payload)),
  updateItem: (id, itemId, payload) => req(`/lists/${id}/items/${itemId}`, jsonBody('PATCH', payload)),
  deleteItem: (id, itemId) => req(`/lists/${id}/items/${itemId}`, { method: 'DELETE' }),
  reorder: (id, ids) => req(`/lists/${id}/reorder`, jsonBody('POST', { ids })),
  clearChecked: (id) => req(`/lists/${id}/clear-checked`, { method: 'POST' }),
  // payload: { client_id, total_amount, category?, description?, items?: [ids] }
  finish: (id, payload) => req(`/lists/${id}/finish`, jsonBody('POST', payload)),
  suggestions: (q, listId) => req(`/lists/suggestions?q=${encodeURIComponent(q || '')}${listId ? `&list=${listId}` : ''}`),
  staples: () => req('/staples'),
  createStaple: (payload) => req('/staples', jsonBody('POST', payload)),
  updateStaple: (id, payload) => req(`/staples/${id}`, jsonBody('PATCH', payload)),
  deleteStaple: (id) => req(`/staples/${id}`, { method: 'DELETE' }),
}
// --- end lists ---

// --- chores --- (chores, approvals, rewards). {id} in complete/approve/
// reject/undo/photo is the INSTANCE id; update/remove take the chore id.
export const choresApi = {
  list: () => req('/chores'),
  create: (payload) => req('/chores', { method: 'POST', body: JSON.stringify(payload) }),
  update: (id, payload) => req(`/chores/${id}`, { method: 'PATCH', body: JSON.stringify(payload) }),
  remove: (id) => req(`/chores/${id}`, { method: 'DELETE' }),
  today: () => req('/chores/today'),
  instances: (from, to, member) => {
    const p = new URLSearchParams()
    if (from) p.set('from', from)
    if (to) p.set('to', to)
    if (member) p.set('member', member)
    return req(`/chores/instances?${p.toString()}`)
  },
  complete: (id, { note = '', photo = null } = {}) => {
    if (!photo) return req(`/chores/${id}/complete`, { method: 'POST', body: JSON.stringify({ note }) })
    const fd = new FormData()
    fd.append('photo', photo)
    if (note) fd.append('note', note)
    return upload(`/chores/${id}/complete`, fd)
  },
  approve: (id) => req(`/chores/${id}/approve`, { method: 'POST', body: '{}' }),
  reject: (id, note) => req(`/chores/${id}/reject`, { method: 'POST', body: JSON.stringify({ note: note || '' }) }),
  undo: (id) => req(`/chores/${id}/undo`, { method: 'POST', body: '{}' }),
  photoUrl: (id) => `${BASE}/chores/${id}/photo`,
}
// --- end chores ---

// --- reminders --- (reminders, Today page, household time zone, digest, chat linking)
// Occurrence ids (done/snooze) come from upcoming() items and Today items of kind "reminder".
export const remindersApi = {
  list: () => req('/reminders'),
  create: (payload) => req('/reminders', { method: 'POST', body: JSON.stringify(payload) }),
  update: (id, payload) => req(`/reminders/${id}`, { method: 'PATCH', body: JSON.stringify(payload) }),
  remove: (id) => req(`/reminders/${id}`, { method: 'DELETE' }),
  upcoming: (days = 7) => req(`/reminders/upcoming?days=${days}`),
  done: (occurrenceId) => req(`/reminders/occurrences/${occurrenceId}/done`, { method: 'POST', body: '{}' }),
  snooze: (occurrenceId, { minutes, until } = {}) =>
    req(`/reminders/occurrences/${occurrenceId}/snooze`, { method: 'POST', body: JSON.stringify(until ? { until } : { minutes: minutes || 60 }) }),
  chatLink: () => req('/reminders/chat-link'),
  createLinkCode: () => req('/reminders/link-code', { method: 'POST', body: '{}' }),
  unlinkChat: (platform) => req(`/reminders/chat-link/${platform}`, { method: 'DELETE' }),
  today: () => req('/today'),
  timezone: () => req('/household/timezone'),
  setTimezone: (timezone) => req('/household/timezone', { method: 'PATCH', body: JSON.stringify({ timezone }) }),
  digest: () => req('/household/digest'),
  setDigest: (payload) => req('/household/digest', { method: 'PATCH', body: JSON.stringify(payload) }),
}
// --- end reminders ---

// --- meal plan --- (weekly meal plan, must-use ingredients, chat post schedule,
// learned chat-bot spending keywords; merged from the household "rumahbot")
export const mealApi = {
  plan: (week) => req(`/meal-plan${week === 'this' ? '?week=this' : ''}`),
  ingredients: () => req('/meal-plan/ingredients'),
  addIngredient: (name) => req('/meal-plan/ingredients', { method: 'POST', body: JSON.stringify({ name }) }),
  deleteIngredient: (id) => req(`/meal-plan/ingredients/${id}`, { method: 'DELETE' }),
  schedule: () => req('/meal-plan/schedule'),
  // payload: { enabled?, days?: [0-6, 0=Sunday], time?: 'HH:MM', start_date?: 'YYYY-MM-DD' | '' }
  setSchedule: (payload) => req('/meal-plan/schedule', { method: 'PUT', body: JSON.stringify(payload) }),
  keywords: () => req('/bot-keywords'),
  saveKeyword: (keyword, category) => req('/bot-keywords', { method: 'POST', body: JSON.stringify({ keyword, category }) }),
  deleteKeyword: (id) => req(`/bot-keywords/${id}`, { method: 'DELETE' }),
}
// --- end meal plan ---

// --- calendar --- (activities + calendar range with the reminders overlay)
export const calendarApi = {
  range: (from, to, include) => req(`/calendar?from=${from}&to=${to}${include ? `&include=${include}` : ''}`),
  create: (payload) => req('/activities', { method: 'POST', body: JSON.stringify(payload) }),
  update: (id, payload) => req(`/activities/${id}`, { method: 'PATCH', body: JSON.stringify(payload) }),
  remove: (id, on) => req(`/activities/${id}${on ? `?on=${on}` : ''}`, { method: 'DELETE' }),
}
// --- end calendar ---

// --- meal proposals --- (anyone proposes, owner/parent decide)
export const mealProposalsApi = {
  list: (status) => req(`/meal-plan/proposals${status ? `?status=${status}` : ''}`),
  propose: (payload) => req('/meal-plan/proposals', { method: 'POST', body: JSON.stringify(payload) }),
  approve: (id, payload) => req(`/meal-plan/proposals/${id}/approve`, { method: 'POST', body: JSON.stringify(payload || {}) }),
  decline: (id, payload) => req(`/meal-plan/proposals/${id}/decline`, { method: 'POST', body: JSON.stringify(payload || {}) }),
  remove: (id) => req(`/meal-plan/proposals/${id}`, { method: 'DELETE' }),
}
// --- end meal proposals ---

// --- widgets --- (widget boards for phone / TV / car / monitor)
export const widgetsApi = {
  catalog: () => req('/widgets/catalog'),
  boards: () => req('/widgets/boards'),
  createBoard: (payload) => req('/widgets/boards', { method: 'POST', body: JSON.stringify(payload) }),
  updateBoard: (id, payload) => req(`/widgets/boards/${id}`, { method: 'PATCH', body: JSON.stringify(payload) }),
  removeBoard: (id) => req(`/widgets/boards/${id}`, { method: 'DELETE' }),
  data: (types) => req(`/widgets/data${types?.length ? `?types=${types.join(',')}` : ''}`),
}
// --- end widgets ---
