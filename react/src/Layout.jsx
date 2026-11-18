import { useEffect, useState } from 'react'
import { NavLink, Outlet, useLocation } from 'react-router-dom'
import { useAuth, canLogSpending, canManage } from './AuthContext.jsx'
import { useOffline } from './OfflineContext.jsx'
import { useTheme, THEMES } from './ThemeContext.jsx'
import { formatRupiah, onSpendingChanged } from './api'
import { navRoutes, routeTo, routeEnd } from './routes.jsx'
import QuickAddSpending from './QuickAddSpending.jsx'
import GamificationBadge from './GamificationBadge.jsx'
import Modal from './Modal.jsx'
import OnboardingChecklist from './OnboardingChecklist.jsx'
import { IconLogout, IconCollapse, IconExpand, IconSun, IconMoon, IconPlus, IconMore, IconClose, IconGuide, IconChevronRight, KinkitLogo } from './icons.jsx'

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
  const familyName = (user?.household_name || '').trim()
  const canAdd = !isAdmin && canLogSpending(user)
  const canAddIncome = !isAdmin && canManage(user)
  const links = navRoutes(isAdmin ? 'admin' : 'tenant', user)
  const mainLinks = links.filter(r => r.placement !== 'bottom')
  const bottomLinks = links.filter(r => r.placement === 'bottom')
  const mobileTabs = links.filter(r => r.mobile === 'tab')
  const moreLinks = links.filter(r => r.mobile === 'more')
  const inSection = r => location.pathname === routeTo(r) || location.pathname.startsWith(routeTo(r) + '/')
  const moreActive = location.pathname === '/help' || moreLinks.some(inSection)
  const navItem = r => (
    <li key={r.path}>
      <NavLink to={routeTo(r)} end={routeEnd(r)} title={r.label} aria-label={collapsed ? r.label : undefined}>
        <r.Icon aria-hidden="true" />
        {!collapsed && <span>{r.label}</span>}
      </NavLink>
    </li>
  )

  useEffect(() => {
    document.title = familyName ? `Kinkit · ${familyName}` : 'Kinkit'
    return () => { document.title = 'Kinkit' }
  }, [familyName])
  useEffect(() => onSpendingChanged(() => setGamiRefresh(k => k + 1)), [])
  useEffect(() => { setMoreOpen(false) }, [location.pathname])
  useEffect(() => { try { localStorage.setItem(COLLAPSE_KEY, collapsed ? '1' : '0') } catch { /* ignore */ } }, [collapsed])

  const nextTheme = THEMES[(THEMES.findIndex(t => t.id === theme) + 1) % THEMES.length]

  return (
    <div className="app-shell">
      <a href="#main" className="skip-link">Skip to content</a>

      <aside className={`sidebar ${collapsed ? 'collapsed' : ''}`}>
        <div className="sidebar-top">
          {!collapsed && (
            <div className="brand-row">
              <KinkitLogo className="brand-logo" aria-hidden="true" />
              <div className="brand-block">
                <div className="brand">Kinkit</div>
                {familyName && <div className="brand-family" title={familyName}>{familyName}</div>}
              </div>
            </div>
          )}
          <button className="icon-btn collapse-toggle" onClick={() => setCollapsed(c => !c)} title={collapsed ? 'Expand' : 'Collapse'} aria-label={collapsed ? 'Expand sidebar' : 'Collapse sidebar'} aria-expanded={!collapsed}>
            {collapsed ? <IconExpand /> : <IconCollapse />}
          </button>
        </div>

        <ConnStatus serverOnline={serverOnline} queueSize={queueSize} compact={collapsed} />

        <ul>
          {canAdd && (
            <li>
              <button className="nav-cta" onClick={() => setQuickAddOpen(true)} title={canAddIncome ? 'Add spending or income' : 'Add Spending'} aria-label={collapsed ? 'Add' : undefined}>
                <IconPlus />
                {!collapsed && <span>{canAddIncome ? 'Add' : 'Add Spending'}</span>}
              </button>
            </li>
          )}
          {!isAdmin && (
            <li><GamificationBadge collapsed={collapsed} refreshKey={gamiRefresh} /></li>
          )}
          {mainLinks.map((r, i) => (
            r.group && r.group !== mainLinks[i - 1]?.group && !collapsed
              ? [<li key={`g-${r.group}`} className="nav-group" aria-hidden="true">{r.group}</li>, navItem(r)]
              : navItem(r)
          ))}
        </ul>
        {bottomLinks.length > 0 && <ul className="sidebar-bottom-links">{bottomLinks.map(navItem)}</ul>}
        <div className="sidebar-footer">
          <button className="icon-btn theme-toggle" onClick={() => setTheme(nextTheme.id)} title={`Theme: ${THEMES.find(t => t.id === theme)?.label} (click to switch)`} aria-label={`Switch theme to ${nextTheme.label}`}>
            {theme === 'dark' ? <IconSun /> : <IconMoon />}
          </button>
          {!isAdmin && (
            <NavLink to="/help" className="icon-btn help-btn" title="Help & guide" aria-label="Help and guide">
              <IconGuide />
            </NavLink>
          )}
          {!collapsed && <span className="sidebar-user">{user?.display_name}</span>}
          <button className="icon-btn logout-btn" onClick={logout} title="Logout" aria-label="Logout"><IconLogout /></button>
        </div>
      </aside>

      {/* ---- mobile (<640px): slim top bar + bottom tabs + FAB ---- */}
      <header className="mobile-topbar">
        <div className="brand-row">
          <KinkitLogo className="brand-logo" aria-hidden="true" />
          <div className="brand-block">
            <div className="brand">Kinkit</div>
            {familyName && <div className="brand-family" title={familyName}>{familyName}</div>}
          </div>
        </div>
        <ConnStatus serverOnline={serverOnline} queueSize={queueSize} />
        {!isAdmin && (
          <NavLink to="/help" className="icon-btn help-btn" aria-label="Help and guide"><IconGuide width={20} height={20} /></NavLink>
        )}
      </header>

      <main className="content" id="main" tabIndex={-1}>
        <FailedSyncBanner items={failedItems} onDismiss={dismissFailed} />
        {canAddIncome && location.pathname === '/' && <OnboardingChecklist />}
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
        <button className="fab" onClick={() => setQuickAddOpen(true)} aria-label={canAddIncome ? 'Add spending or income' : 'Add spending'}>
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
                  <r.Icon aria-hidden="true" /><span>{r.label}</span><IconChevronRight className="chev" width={16} height={16} aria-hidden="true" />
                </NavLink>
              </li>
            ))}
            {!isAdmin && (
              <li>
                <NavLink to="/help"><IconGuide aria-hidden="true" /><span>Help</span><IconChevronRight className="chev" width={16} height={16} aria-hidden="true" /></NavLink>
              </li>
            )}
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

      {quickAddOpen && <QuickAddSpending allowIncome={canAddIncome} onClose={() => setQuickAddOpen(false)} />}
    </div>
  )
}
