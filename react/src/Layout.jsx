import { useEffect, useState } from 'react'
import { NavLink, Outlet, useLocation } from 'react-router-dom'
import { useAuth, canLogSpending } from './AuthContext.jsx'
import { useOffline } from './OfflineContext.jsx'
import { useTheme, THEMES } from './ThemeContext.jsx'
import { formatRupiah, onSpendingChanged } from './api'
import { navRoutes, routeTo, routeEnd } from './routes.jsx'
import QuickAddSpending from './QuickAddSpending.jsx'
import GamificationBadge from './GamificationBadge.jsx'
import Modal from './Modal.jsx'
import OnboardingChecklist from './OnboardingChecklist.jsx'
import { IconLogout, IconCollapse, IconExpand, IconSun, IconMoon, IconPlus, IconMore, IconTv, IconClose } from './icons.jsx'

const COLLAPSE_KEY = 'kinkit_sidebar_collapsed'

function ConnStatus({ serverOnline, queueSize, compact }) {
  const text = serverOnline ? 'Online' : `Offline${queueSize ? ` (${queueSize})` : ''}`
  return (
    <div
      className={`conn-status ${serverOnline ? 'online' : 'offline'}`}
      title={serverOnline ? 'Connected to server' : `Offline${queueSize ? ` - ${queueSize} transaction(s) pending sync` : ''}`}
      role="status"
    >
      <span className="conn-dot" aria-hidden="true" />
      {compact ? <span className="sr-only">{text}</span> : <span>{text}</span>}
    </div>
  )
}

// Offline entries the server rejected (4xx) during sync - shown until dismissed.
function FailedSyncBanner({ items, onDismiss }) {
  if (!items.length) return null
  return (
    <div className="error-box failed-sync" role="alert">
      <strong>{items.length} offline transaction{items.length > 1 ? 's were' : ' was'} rejected by the server and not saved:</strong>
      <ul>
        {items.map(f => (
          <li key={f.client_id}>
            <span>{f.tx_date} · {f.description} · {f.category} · {formatRupiah(f.amount)} - <em>{f.error}</em></span>
            <button className="icon-btn" onClick={() => onDismiss(f.client_id)} aria-label={`Dismiss failed entry ${f.description}`}>
              <IconClose width={12} height={12} />
            </button>
          </li>
        ))}
      </ul>
      {items.length > 1 && <button className="secondary" onClick={() => onDismiss()}>Dismiss all</button>}
    </div>
  )
}

export default function Layout() {
  const { logout, user } = useAuth()
  const { serverOnline, queueSize, failedItems, dismissFailed } = useOffline()
  const { theme, setTheme } = useTheme()
  const location = useLocation()
  const [collapsed, setCollapsed] = useState(() => { try { return localStorage.getItem(COLLAPSE_KEY) === '1' } catch { return false } })
  const [quickAddOpen, setQuickAddOpen] = useState(false)
  const [moreOpen, setMoreOpen] = useState(false)
  const [gamiRefresh, setGamiRefresh] = useState(0)

  const isAdmin = !!user?.is_admin
  const canAdd = !isAdmin && canLogSpending(user)
  const links = navRoutes(isAdmin ? 'admin' : 'tenant')
  const mobileTabs = links.filter(r => r.mobile === 'tab')
  const moreLinks = links.filter(r => r.mobile === 'more')
  const moreActive = moreLinks.some(r => location.pathname === routeTo(r))

  useEffect(() => onSpendingChanged(() => setGamiRefresh(k => k + 1)), [])
  useEffect(() => { setMoreOpen(false) }, [location.pathname])
  useEffect(() => { try { localStorage.setItem(COLLAPSE_KEY, collapsed ? '1' : '0') } catch { /* ignore */ } }, [collapsed])

  const nextTheme = THEMES[(THEMES.findIndex(t => t.id === theme) + 1) % THEMES.length]

  return (
    <div className="app-shell">
      <a href="#main" className="skip-link">Skip to content</a>

      <aside className={`sidebar ${collapsed ? 'collapsed' : ''}`}>
        <div className="sidebar-top">
          {!collapsed && <div className="brand">Kinkit</div>}
          <button className="icon-btn collapse-toggle" onClick={() => setCollapsed(c => !c)} title={collapsed ? 'Expand' : 'Collapse'} aria-label={collapsed ? 'Expand sidebar' : 'Collapse sidebar'} aria-expanded={!collapsed}>
            {collapsed ? <IconExpand /> : <IconCollapse />}
          </button>
        </div>

        <ConnStatus serverOnline={serverOnline} queueSize={queueSize} compact={collapsed} />

        <ul>
          {canAdd && (
            <li>
              <button className="nav-cta" onClick={() => setQuickAddOpen(true)} title="Add Spending" aria-label={collapsed ? 'Add Spending' : undefined}>
                <IconPlus />
                {!collapsed && <span>Add Spending</span>}
              </button>
            </li>
          )}
          {!isAdmin && (
            <li><GamificationBadge collapsed={collapsed} refreshKey={gamiRefresh} /></li>
          )}
          {links.map(r => (
            <li key={r.path}>
              <NavLink to={routeTo(r)} end={routeEnd(r)} title={r.label} aria-label={collapsed ? r.label : undefined}>
                <r.Icon aria-hidden="true" />
                {!collapsed && <span>{r.label}</span>}
              </NavLink>
            </li>
          ))}
        </ul>
        <div className="sidebar-footer">
          {!collapsed ? (
            <select className="theme-select" value={theme} onChange={e => setTheme(e.target.value)} title="Theme" aria-label="Theme">
              {THEMES.map(t => <option key={t.id} value={t.id}>{t.label}</option>)}
            </select>
          ) : (
            <button className="icon-btn theme-toggle" onClick={() => setTheme(nextTheme.id)} title="Cycle theme" aria-label={`Switch theme to ${nextTheme.label}`}>
              {theme === 'dark' ? <IconSun /> : <IconMoon />}
            </button>
          )}
          {!collapsed && <span>{user?.display_name}</span>}
          {!collapsed && <a href="/tv" target="_blank" rel="noopener" className="link-btn" title="Open TV / car display view">TV view</a>}
          <button className="icon-btn logout-btn" onClick={logout} title="Logout" aria-label="Logout"><IconLogout /></button>
        </div>
      </aside>

      {/* ---- mobile (<640px): slim top bar + bottom tabs + FAB ---- */}
      <header className="mobile-topbar">
        <div className="brand">Kinkit</div>
        <ConnStatus serverOnline={serverOnline} queueSize={queueSize} />
      </header>

      <main className="content" id="main" tabIndex={-1}>
        <FailedSyncBanner items={failedItems} onDismiss={dismissFailed} />
        {!isAdmin && location.pathname === '/' && <OnboardingChecklist />}
        <Outlet />
      </main>

      <nav className="mobile-tabbar" aria-label="Main">
        {mobileTabs.map(r => (
          <NavLink key={r.path} to={routeTo(r)} end={routeEnd(r)} className="mobile-tab">
            <r.Icon width={22} height={22} aria-hidden="true" />
            <span>{r.label}</span>
          </NavLink>
        ))}
        <button className={`mobile-tab ${moreActive || moreOpen ? 'active' : ''}`} onClick={() => setMoreOpen(true)} aria-haspopup="dialog" aria-expanded={moreOpen}>
          <IconMore width={22} height={22} aria-hidden="true" />
          <span>More</span>
        </button>
      </nav>

      {canAdd && (
        <button className="fab" onClick={() => setQuickAddOpen(true)} aria-label="Add spending">
          <IconPlus width={26} height={26} />
        </button>
      )}

      {moreOpen && (
        <Modal title="More" onClose={() => setMoreOpen(false)} className="sheet">
          {!isAdmin && <GamificationBadge refreshKey={gamiRefresh} />}
          <ul className="sheet-links">
            {moreLinks.map(r => (
              <li key={r.path}>
                <NavLink to={routeTo(r)} end={routeEnd(r)}>
                  <r.Icon aria-hidden="true" /><span>{r.label}</span>
                </NavLink>
              </li>
            ))}
            <li>
              <a href="/tv" target="_blank" rel="noopener"><IconTv aria-hidden="true" /><span>TV view</span></a>
            </li>
          </ul>
          <label className="sheet-row">
            <span>Theme</span>
            <select value={theme} onChange={e => setTheme(e.target.value)}>
              {THEMES.map(t => <option key={t.id} value={t.id}>{t.label}</option>)}
            </select>
          </label>
          <div className="sheet-row">
            <span className="muted">Signed in as {user?.display_name || user?.username}</span>
            <button className="secondary" onClick={logout}><IconLogout width={16} height={16} aria-hidden="true" /> Logout</button>
          </div>
        </Modal>
      )}

      {quickAddOpen && <QuickAddSpending onClose={() => setQuickAddOpen(false)} />}
    </div>
  )
}
