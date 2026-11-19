import { useEffect, useRef, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useAuth } from '../AuthContext.jsx'
import { KinkitLogo, IconBudget, IconChores, IconCalendar, IconWidgets, IconTrophy } from '../icons.jsx'
import Mascot from '../Mascot.jsx'
import './login.css'

const FEATURES = [
  { Icon: IconBudget, text: 'Money & budgets' },
  { Icon: IconChores, text: 'Chores, lists & meals' },
  { Icon: IconCalendar, text: 'Shared calendar & reminders' },
  { Icon: IconWidgets, text: 'Widget screens for phone, TV & car' },
  { Icon: IconTrophy, text: 'Points, levels & badges' },
]

/**
 * Animated isometric household scene (three.js). The gradient hero and the text
 * render immediately; the three.js chunk is loaded lazily and the canvas fades
 * in once ready. Without WebGL the canvas is simply never shown.
 */
function HeroScene() {
  const canvasRef = useRef(null)
  const [ready, setReady] = useState(false)
  useEffect(() => {
    let cancelled = false
    let handle = null
    const reduced = window.matchMedia?.('(prefers-reduced-motion: reduce)').matches
    import('../LoginScene.js')
      .then(m => {
        if (cancelled || !canvasRef.current) return
        handle = m.createLoginScene(canvasRef.current, { reducedMotion: !!reduced })
        if (handle) setReady(true)
      })
      .catch(() => { /* keep the gradient hero */ })
    return () => { cancelled = true; handle?.dispose() }
  }, [])
  return <canvas ref={canvasRef} className={`login-canvas ${ready ? 'ready' : ''}`} aria-hidden="true" />
}

export default function Login() {
  const { login, signup } = useAuth()
  const navigate = useNavigate()
  const [mode, setMode] = useState('login') // 'login' | 'signup'
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [displayName, setDisplayName] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  async function handleSubmit(e) {
    e.preventDefault()
    setError('')
    setBusy(true)
    try {
      if (mode === 'signup') {
        await signup(username, password, displayName)
      } else {
        await login(username, password)
      }
      navigate('/')
    } catch (err) {
      setError(err.message || (mode === 'signup' ? 'Sign up failed' : 'Login failed'))
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="login-split">
      <section className="login-hero" aria-label="About Kinkit">
        <HeroScene />
        <div className="login-hero-top">
          <div className="login-hero-brand"><KinkitLogo className="login-hero-logo" aria-hidden="true" /><span>Kinkit</span></div>
          <p className="login-hero-title">Run your whole household, <em>not just its budget.</em></p>
        </div>
        <ul className="login-chips">
          {FEATURES.map(({ Icon, text }) => (
            <li key={text}><Icon width={16} height={16} aria-hidden="true" />{text}</li>
          ))}
        </ul>
      </section>

      <div className="login-side">
      <form className="card login-card" onSubmit={handleSubmit}>
        <div className="login-mascot"><Mascot pose={error ? 'think' : 'idle'} size={112} wave /></div>
        <div className="login-brand"><KinkitLogo className="login-logo" aria-hidden="true" /><h1>Kinkit</h1></div>
        <p className="muted">{mode === 'login' ? 'Welcome back. Log in to your household.' : 'Create a household and invite your family.'}</p>
        {error && <div className="error-box">{error}</div>}

        <label>
          Email
          <input type="email" value={username} onChange={e => setUsername(e.target.value)} required autoFocus />
        </label>
        {mode === 'signup' && (
          <label>
            Name
            <input value={displayName} onChange={e => setDisplayName(e.target.value)} placeholder="optional" />
          </label>
        )}
        <label>
          Password
          <input type="password" value={password} onChange={e => setPassword(e.target.value)} required minLength={mode === 'signup' ? 8 : undefined} />
        </label>
        <button type="submit" disabled={busy}>
          {busy ? 'Please wait...' : mode === 'signup' ? 'Create account' : 'Login'}
        </button>
        <button type="button" className="link-btn" onClick={() => { setMode(m => (m === 'login' ? 'signup' : 'login')); setError('') }}>
          {mode === 'login' ? "Don't have an account? Sign up" : 'Already have an account? Log in'}
        </button>
        <p className="muted login-hint">
          {mode === 'login'
            ? 'Family members log in here too, with the email and temporary password the family admin or a parent set up for them.'
            : 'Signing up creates a new household with you as its family admin. Joining an existing family? Ask the family admin to add you on the Family page instead.'}
        </p>
      </form>
      </div>
    </div>
  )
}
