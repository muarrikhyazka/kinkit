import { NavLink, Navigate, Outlet, useLocation } from 'react-router-dom'
import { useAuth } from './AuthContext.jsx'
import { visibleTabs } from './routes.jsx'
import { InTabsContext } from './PageHeader.jsx'

/**
 * Wrapper for sections with tabs (Transactions, Savings, Settings). Renders
 * the tab strip, then the active tab's page via <Outlet/>. Each tab page keeps
 * its own PageHeader (period stepper etc.).
 */
export default function TabbedSection({ route }) {
  const { user } = useAuth()
  const location = useLocation()
  const tabs = visibleTabs(route, user)
  const base = '/' + route.path

  if (tabs.length === 0) return <Navigate to="/" replace />
  // "/transactions" or a tab this role can't see -> first allowed tab
  const current = location.pathname.slice(base.length + 1).split('/')[0]
  if (!tabs.some(t => t.path === current)) return <Navigate to={`${base}/${tabs[0].path}`} replace />

  return (
    <div>
      {tabs.length > 1 && (
        <nav className="tabs section-tabs" aria-label={route.label}>
          {tabs.map(t => (
            <NavLink key={t.path} to={`${base}/${t.path}`} className={({ isActive }) => `tab ${isActive ? 'active' : ''}`}>
              {t.label}
            </NavLink>
          ))}
        </nav>
      )}
      <InTabsContext.Provider value={tabs.length > 1}>
        <Outlet />
      </InTabsContext.Provider>
    </div>
  )
}
