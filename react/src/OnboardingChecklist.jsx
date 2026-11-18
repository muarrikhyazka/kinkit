import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { api, dataApi, onSpendingChanged } from './api'
import { useSetup } from './useApi'
import { IconCheck, IconClose, IconChevronRight } from './icons.jsx'

const DISMISS_KEY = 'kinkit_onboarding_dismissed'

function readDismissed() {
  try { return localStorage.getItem(DISMISS_KEY) === '1' } catch { return false }
}

// Open steps are whole clickable rows with a chevron; done steps are plain.
function Step({ step: s }) {
  const body = (
    <>
      <span className="onboarding-check" aria-hidden="true">{s.done ? <IconCheck width={14} height={14} /> : null}</span>
      <span className="onboarding-step-text">
        <span>
          <span className="onboarding-title">{s.title}</span>
          {s.optional && <span className="muted"> (optional)</span>}
          <span className="sr-only">{s.done ? ' - done' : ' - to do'}</span>
        </span>
        <span className="muted">{s.hint}</span>
      </span>
      {!s.done && <IconChevronRight className="chev" width={18} height={18} aria-hidden="true" />}
    </>
  )
  return s.done ? <div className="onboarding-step">{body}</div> : <Link to={s.to} className="onboarding-step">{body}</Link>
}

const hasAmount = list => Array.isArray(list) && list.some(x => Number(x?.amount) > 0)

/**
 * "Getting started" checklist at the top of the Dashboard. Completion is
 * derived from real data (setup, this year's budget, spending count, bot
 * integrations), so it ticks itself off. Dismissible; hides itself once
 * every required step is done.
 */
export default function OnboardingChecklist() {
  const { setup, categories, incomeSources } = useSetup()
  const [dismissed, setDismissed] = useState(readDismissed)
  const [state, setState] = useState(null) // { budget, spending, bot }
  const [nonce, setNonce] = useState(0)

  useEffect(() => onSpendingChanged(() => setNonce(n => n + 1)), [])

  useEffect(() => {
    if (dismissed) return
    let alive = true
    const year = new Date().getUTCFullYear()
    Promise.allSettled([api.budgeting(year), dataApi.spendingCount(), api.integrations()]).then(([b, s, i]) => {
      if (!alive) return
      setState({
        budget: b.status === 'fulfilled' && (hasAmount(b.value?.expense) || hasAmount(b.value?.income)),
        spending: s.status === 'fulfilled' && (s.value?.total || 0) > 0,
        bot: i.status === 'fulfilled' && Array.isArray(i.value) && i.value.length > 0,
      })
    })
    return () => { alive = false }
  }, [dismissed, nonce])

  if (dismissed || !state || !setup) return null

  const steps = [
    { id: 'setup', done: categories.length > 0 && incomeSources.length > 0, to: '/settings/general', title: 'Set up income sources & categories', hint: 'Where your money comes from and what you spend it on.' },
    { id: 'budget', done: state.budget, to: '/budget', title: 'Set a budget', hint: 'Plan this month\'s income and how much each category may use.' },
    { id: 'spending', done: state.spending, to: '/transactions/spending', title: 'Log your first expense', hint: 'Use Add Spending, or import a bank CSV under Settings > Data & devices.' },
    { id: 'bot', done: state.bot, to: '/settings/bots', title: 'Connect a chat bot', hint: 'Log spending from Telegram or Discord.', optional: true },
  ]
  const required = steps.filter(s => !s.optional)
  if (required.every(s => s.done)) return null
  const doneCount = steps.filter(s => s.done).length

  function dismiss() {
    try { localStorage.setItem(DISMISS_KEY, '1') } catch { /* ignore */ }
    setDismissed(true)
  }

  return (
    <section className="card onboarding" aria-labelledby="onboarding-title">
      <div className="onboarding-head">
        <div>
          <h2 id="onboarding-title">Getting started</h2>
          <p className="muted" style={{ margin: 0 }}>{doneCount} of {steps.length} done</p>
        </div>
        <button className="icon-btn" onClick={dismiss} aria-label="Dismiss getting started checklist" title="Dismiss"><IconClose width={14} height={14} /></button>
      </div>
      <div className="progress-bar" aria-hidden="true"><div className="progress-fill" style={{ width: `${(doneCount / steps.length) * 100}%` }} /></div>
      <ol className="onboarding-steps">
        {steps.map(s => (
          <li key={s.id} className={s.done ? 'done' : ''}>
            <Step step={s} />
          </li>
        ))}
      </ol>
    </section>
  )
}
