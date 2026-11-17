import { createContext, useContext, useEffect, useState } from 'react'
import { api, setCurrency, fetchFxRates } from './api'
import { clearSetupCache } from './useApi'

const AuthCtx = createContext(null)

export function AuthProvider({ children }) {
  const [user, setUser] = useState(undefined) // undefined=loading, null=anon, obj=logged in

  useEffect(() => {
    fetchFxRates() // once per app load; formatRupiah() uses whatever lands here
    api.me().then(d => {
      if (d.authenticated) setCurrency(d.currency)
      setUser(d.authenticated ? d : null)
    }).catch(() => setUser(null))
  }, [])

  // /me is the source of truth for who's logged in: household member, role
  // (owner | parent | kid | viewer), household name, currency.
  async function loadMe(fallback) {
    const me = await api.me().catch(() => null)
    const u = me?.authenticated ? me : { authenticated: true, currency: 'IDR', is_admin: false, ...fallback }
    setCurrency(u.currency)
    setUser(u)
    return u
  }

  async function login(username, password) {
    const d = await api.login(username, password)
    clearSetupCache()
    await loadMe({ username, display_name: d.display_name, role: d.role })
  }

  async function signup(email, password, displayName) {
    const d = await api.signup(email, password, displayName)
    clearSetupCache()
    await loadMe({ username: email, display_name: d.display_name, role: d.role || 'owner' })
  }

  // re-read /me after profile/household changes (name, household name, role)
  async function refreshUser() {
    const me = await api.me().catch(() => null)
    if (me?.authenticated) { setCurrency(me.currency); setUser(me) }
  }

  async function logout() {
    await api.logout().catch(() => {})
    clearSetupCache()
    setUser(null)
  }

  function setUserCurrency(code) {
    setCurrency(code)
    setUser(u => (u ? { ...u, currency: code } : u))
  }

  return <AuthCtx.Provider value={{ user, login, signup, logout, setUserCurrency, refreshUser }}>{children}</AuthCtx.Provider>
}

export const useAuth = () => useContext(AuthCtx)

// Role helpers (mirrors backend family.go): owners/parents run the
// household; kids may log spending; viewers are read-only.
export const canManage = user => !user?.role || user.role === 'owner' || user.role === 'parent'
export const canLogSpending = user => canManage(user) || user?.role === 'kid'
export const ROLE_LABELS = { owner: 'Owner', parent: 'Parent', kid: 'Kid', viewer: 'Viewer' }
