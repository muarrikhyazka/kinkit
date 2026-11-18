import { useEffect, useState } from 'react'
import { useTheme, THEMES } from '../ThemeContext.jsx'
import PageHeader from '../PageHeader.jsx'
import { remindersApi } from '../api'
import { useApi } from '../useApi'
import { useToast } from '../Toast.jsx'

const ZONES = (() => {
  try { return Intl.supportedValuesOf('timeZone') } catch { return ['UTC', 'Asia/Jakarta', 'Asia/Singapore', 'Europe/London', 'America/New_York'] }
})()

/** Household time zone + morning digest (owners/parents can edit). */
function HouseholdTimeCard() {
  const toast = useToast()
  const tz = useApi(() => remindersApi.timezone(), [])
  const digest = useApi(() => remindersApi.digest(), [])
  const [zone, setZone] = useState('')
  const [dg, setDg] = useState(null)
  useEffect(() => { if (tz.data) setZone(tz.data.timezone) }, [tz.data])
  useEffect(() => { if (digest.data) setDg(digest.data) }, [digest.data])
  if (!tz.data || !dg) return null
  const canEdit = tz.data.can_edit
  const detected = Intl.DateTimeFormat().resolvedOptions().timeZone

  async function saveZone(value) {
    setZone(value)
    try { await remindersApi.setTimezone(value); toast.success(`Time zone set to ${value}`); tz.reload() } catch (err) { toast.error(err.message); setZone(tz.data.timezone) }
  }
  async function saveDigest(patch) {
    const next = { ...dg, ...patch }
    setDg(next)
    try { await remindersApi.setDigest(next); toast.success('Morning digest saved') } catch (err) { toast.error(err.message); digest.reload() }
  }

  return (
    <div className="card">
      <h2>Household time &amp; morning digest</h2>
      <p className="muted">Everything is stored in UTC. The household time zone decides when "every day at 08:00" actually fires, for the whole family.</p>
      <div className="form-row">
        <label>Time zone
          <select value={zone} onChange={e => saveZone(e.target.value)} disabled={!canEdit}>
            {!ZONES.includes(zone) && <option value={zone}>{zone}</option>}
            {ZONES.map(z => <option key={z} value={z}>{z}</option>)}
          </select>
        </label>
        {canEdit && detected && detected !== zone && (
          <button type="button" className="secondary" style={{ alignSelf: 'flex-end' }} onClick={() => saveZone(detected)}>Use this device ({detected})</button>
        )}
      </div>
      <div className="form-row" style={{ marginTop: '1rem' }}>
        <label className="checkbox-row">
          <input type="checkbox" checked={dg.enabled} disabled={!canEdit} onChange={e => saveDigest({ enabled: e.target.checked })} />
          Send a morning digest to the family chat
        </label>
        <label>At
          <input type="time" value={dg.time_of_day} disabled={!canEdit || !dg.enabled} onChange={e => saveDigest({ time_of_day: e.target.value })} />
        </label>
        <label>Via
          <select value={dg.channel} disabled={!canEdit || !dg.enabled} onChange={e => saveDigest({ channel: e.target.value })}>
            <option value="all">All connected chats</option>
            <option value="telegram">Telegram</option>
            <option value="discord">Discord</option>
          </select>
        </label>
      </div>
      {!canEdit && <p className="muted">Only the family admin and parents can change these.</p>}
    </div>
  )
}

const THEME_HINTS = {
  light: 'Clean and bright - the default.',
  dark: 'Easier on the eyes at night.',
  playful: 'Bigger, rounder and more colourful - great for kids.',
  senior: 'Large text and high contrast for easier reading.',
}

/** Settings > Display: per-device preferences (stored in this browser). */
export default function DisplaySettings() {
  const { theme, setTheme } = useTheme()
  return (
    <div>
      <PageHeader title="Display" subtitle="Theme for this device, plus household time zone and morning digest. The household currency is under Categories & accounts." />
      <div className="card">
        <h2>Theme</h2>
        <div className="theme-options" role="radiogroup" aria-label="Theme">
          {THEMES.map(t => (
            <label key={t.id} className={`theme-option ${theme === t.id ? 'active' : ''}`}>
              <input type="radio" name="theme" value={t.id} checked={theme === t.id} onChange={() => setTheme(t.id)} />
              <span className="theme-option-label">{t.label}</span>
              <span className="muted">{THEME_HINTS[t.id]}</span>
            </label>
          ))}
        </div>
      </div>
      <HouseholdTimeCard />
      <div className="card">
        <h2>TV / car display</h2>
        <p className="muted">A big-text, auto-refreshing overview for a screen that is always on.</p>
        <a href="/tv" target="_blank" rel="noopener" className="button-link">Open TV view</a>
      </div>
    </div>
  )
}
