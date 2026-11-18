import { useState } from 'react'
import PageHeader from '../PageHeader.jsx'
import { SkeletonCard } from '../Skeleton.jsx'
import { useToast } from '../Toast.jsx'
import { useApi } from '../useApi'
import { useAuth, canManage } from '../AuthContext.jsx'
import { mealApi } from '../api'
import { IconClose } from '../icons.jsx'
import '../mealplan.css'

/**
 * Home › Meals: the weekly meal plan (one main dish + one side per day, the
 * same all week), the household's must-use ingredients, and when the plan
 * is posted to the family chat. Backed by backend-go/meal_plan.go; the chat
 * bot has the same data (/menu, /ingredients, /schedule).
 */
const DAYS = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat']

function fmtDate(iso, opts) {
  const [y, m, d] = iso.split('-').map(Number)
  return new Date(Date.UTC(y, m - 1, d)).toLocaleDateString(undefined, { timeZone: 'UTC', ...opts })
}

export default function MealPlan() {
  const { user } = useAuth()
  const manager = canManage(user)
  const toast = useToast()
  const [week, setWeek] = useState('next')
  const plan = useApi(() => mealApi.plan(week), [week])
  const ings = useApi(() => mealApi.ingredients(), [])
  const [name, setName] = useState('')
  const [busy, setBusy] = useState(false)

  const addIngredient = async (e) => {
    e.preventDefault()
    if (!name.trim()) return
    setBusy(true)
    try {
      const r = await mealApi.addIngredient(name)
      ings.setData({ items: r.items })
      setName('')
      plan.reload()
    } catch (err) { toast.error(err.message) }
    setBusy(false)
  }

  const removeIngredient = async (it) => {
    try {
      await mealApi.deleteIngredient(it.id)
      ings.setData(d => ({ items: (d?.items || []).filter(x => x.id !== it.id) }))
      plan.reload()
    } catch (err) { toast.error(err.message) }
  }

  const p = plan.data?.plan
  return (
    <div className="meal-page">
      <PageHeader
        title="Meals"
        subtitle={p ? `${fmtDate(p.week_start, { day: 'numeric', month: 'short' })} - ${fmtDate(p.week_end, { day: 'numeric', month: 'short', year: 'numeric' })}` : undefined}
        actions={
          <div className="meal-week-toggle" role="group" aria-label="Which week">
            <button className={week === 'this' ? '' : 'secondary'} onClick={() => setWeek('this')} aria-pressed={week === 'this'}>This week</button>
            <button className={week === 'next' ? '' : 'secondary'} onClick={() => setWeek('next')} aria-pressed={week === 'next'}>Next week</button>
          </div>
        }
      />
      {plan.error && <div className="error-box">{plan.error}</div>}

      <section className="card" aria-labelledby="meal-plan-h">
        <h2 id="meal-plan-h">Weekly menu</h2>
        <p className="muted meal-note">
          One main dish and one side a day, at most two fried dishes a week. The menu stays the same all week and
          changes when you edit the must-use ingredients.
          {p?.promo_count > 0 && ' Dishes using ingredients on promo this week are preferred.'}
        </p>
        {!p ? <SkeletonCard /> : (
          <ul className="meal-days">
            {p.days.map(d => (
              <li key={d.date} className="meal-day">
                <div className="meal-day-head">
                  <strong>{d.day}</strong>
                  <span className="muted">{fmtDate(d.date, { day: 'numeric', month: 'short' })}</span>
                </div>
                <div className="meal-dish"><span className="meal-label">Main</span> {d.protein}{d.fried && <span className="badge meal-fried">fried</span>}</div>
                <div className="meal-dish"><span className="meal-label">Side</span> {d.veggie}</div>
                {d.promo && (
                  <div className="meal-promo muted">
                    Promo: <a href={d.promo.link} target="_blank" rel="noreferrer">{d.promo.name}</a> {d.promo.price} ({d.promo.store})
                  </div>
                )}
              </li>
            ))}
          </ul>
        )}
        {p?.missing?.length > 0 && <p className="muted meal-note">No dish in the menu catalog uses: {p.missing.join(', ')}.</p>}
      </section>

      <section className="card" aria-labelledby="meal-ing-h">
        <h2 id="meal-ing-h">Must-use ingredients</h2>
        <p className="muted meal-note">Something left over or that you want to cook this week? Every ingredient here gets a dish in the menu.</p>
        {ings.error && <div className="error-box">{ings.error}</div>}
        <ul className="meal-chips">
          {(ings.data?.items || []).map(it => (
            <li key={it.id} className="badge meal-chip">
              {it.name}
              {manager && (
                <button className="icon-btn" onClick={() => removeIngredient(it)} aria-label={`Remove ${it.name}`}>
                  <IconClose width={10} height={10} />
                </button>
              )}
            </li>
          ))}
          {ings.data && ings.data.items.length === 0 && <li className="muted">None yet.</li>}
        </ul>
        {manager && (
          <form className="meal-add" onSubmit={addIngredient}>
            <label className="sr-only" htmlFor="meal-ing-input">Ingredient</label>
            <input id="meal-ing-input" value={name} onChange={e => setName(e.target.value)} placeholder="e.g. chicken, tempe, spinach" maxLength={60} />
            <button type="submit" disabled={busy || !name.trim()}>Add</button>
          </form>
        )}
        <p className="muted meal-note">From chat: <kbd>/addingredient chicken</kbd>, <kbd>/removeingredient chicken</kbd>, <kbd>/ingredients</kbd>.</p>
      </section>

      {manager && <ChatSchedule />}
    </div>
  )
}

function ChatSchedule() {
  const toast = useToast()
  const { data, error, setData } = useApi(() => mealApi.schedule(), [])
  const [saving, setSaving] = useState(false)
  if (error) return <div className="error-box">{error}</div>
  if (!data) return <SkeletonCard />
  const s = data.schedule

  const save = async (patch) => {
    setSaving(true)
    try {
      const r = await mealApi.setSchedule(patch)
      setData(r)
    } catch (err) { toast.error(err.message) }
    setSaving(false)
  }
  const toggleDay = (d) => {
    const days = s.days.includes(d) ? s.days.filter(x => x !== d) : [...s.days, d].sort()
    if (days.length) save({ days })
  }

  return (
    <section className="card" aria-labelledby="meal-sched-h">
      <h2 id="meal-sched-h">Post to the family chat</h2>
      <label className="meal-switch">
        <input type="checkbox" checked={s.enabled} disabled={saving} onChange={e => save({ enabled: e.target.checked })} />
        Post next week's menu to the connected Telegram/Discord chat
      </label>
      <div className="weekday-picker meal-days-pick" role="group" aria-label="Days to post">
        {DAYS.map((label, d) => (
          <button key={label} type="button" className={s.days.includes(d) ? '' : 'secondary'} aria-pressed={s.days.includes(d)}
            disabled={saving} onClick={() => toggleDay(d)}>{label}</button>
        ))}
      </div>
      <div className="meal-time-row">
        <label htmlFor="meal-time">Time ({data.timezone})</label>
        <input id="meal-time" type="time" value={s.time} disabled={saving}
          onChange={e => e.target.value && save({ time: e.target.value })} />
      </div>
      <p className="muted meal-note">
        {s.enabled && s.next_run ? `Next post: ${s.next_run}.` : 'Not posting.'}
        {s.last_run ? ` Last post: ${s.last_run}.` : ''} Needs a chat bot with a chat id under Settings › Bots.
        From chat: <kbd>/schedule meal fri 06:00</kbd>.
      </p>
    </section>
  )
}
