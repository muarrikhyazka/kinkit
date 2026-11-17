import Dashboard from './pages/Dashboard.jsx'
import Budgeting from './pages/Budgeting.jsx'
import Spending from './pages/Spending.jsx'
import Assets from './pages/Assets.jsx'
import Setup from './pages/Setup.jsx'
import Guide from './pages/Guide.jsx'
import Integrations from './pages/Integrations.jsx'
import Admin from './pages/Admin.jsx'
import AdminStats from './pages/AdminStats.jsx'
// --- money ---
import Recurring from './pages/Recurring.jsx'
import Income from './pages/Income.jsx'
import Goals from './pages/Goals.jsx'
import { IconRepeat, IconIncome, IconGoal } from './moneyIcons.jsx'
// --- end money ---
// --- family ---
import Family, { IconFamily } from './pages/Family.jsx'
// --- end family ---
// --- data ---
import ImportExport from './pages/ImportExport.jsx'
import { IconDevices } from './Receipts.jsx'
// --- end data ---
import { IconDashboard, IconBudget, IconSpending, IconAssets, IconSetup, IconGuide, IconLink, IconShield } from './icons.jsx'

/**
 * SINGLE SOURCE OF TRUTH for in-app pages: App.jsx builds the <Route>s from
 * this list and Layout.jsx builds the sidebar + mobile bottom bar from it.
 * To add a page, add one entry here - nothing else needs wiring.
 *
 *   path:    route path under "/" ('' = index)
 *   label:   nav label
 *   Icon:    nav icon (icons.jsx)
 *   Component: page component
 *   area:    'tenant' (household pages; super admin is redirected away)
 *            | 'admin' (platform admin pages)
 *   mobile:  'tab'  -> bottom tab bar (keep to ~4 per area)
 *            'more' -> "More" sheet
 *            omitted -> not in mobile nav
 *   nav:     false to register the route without any nav entry
 */
export const APP_ROUTES = [
  { path: '', label: 'Dashboard', Icon: IconDashboard, Component: Dashboard, area: 'tenant', mobile: 'tab' },
  { path: 'spending', label: 'Spending', Icon: IconSpending, Component: Spending, area: 'tenant', mobile: 'tab' },
  { path: 'budgeting', label: 'Budgeting', Icon: IconBudget, Component: Budgeting, area: 'tenant', mobile: 'tab' },
  { path: 'assets', label: 'Assets', Icon: IconAssets, Component: Assets, area: 'tenant', mobile: 'tab' },
  // --- money ---
  { path: 'income', label: 'Income', Icon: IconIncome, Component: Income, area: 'tenant', mobile: 'more' },
  { path: 'recurring', label: 'Recurring', Icon: IconRepeat, Component: Recurring, area: 'tenant', mobile: 'more' },
  { path: 'goals', label: 'Goals', Icon: IconGoal, Component: Goals, area: 'tenant', mobile: 'more' },
  // --- end money ---
  // --- family ---
  { path: 'family', label: 'Family', Icon: IconFamily, Component: Family, area: 'tenant', mobile: 'more' },
  // --- end family ---
  { path: 'setup', label: 'Setup', Icon: IconSetup, Component: Setup, area: 'tenant', mobile: 'more' },
  { path: 'integrations', label: 'Integrations', Icon: IconLink, Component: Integrations, area: 'tenant', mobile: 'more' },
  // --- data ---
  { path: 'import-export', label: 'Data & devices', Icon: IconDevices, Component: ImportExport, area: 'tenant', mobile: 'more' },
  // --- end data ---
  { path: 'guide', label: 'Guide', Icon: IconGuide, Component: Guide, area: 'tenant', mobile: 'more' },

  // Super admin manages the platform, not personal finances.
  { path: 'admin', label: 'Accounts', Icon: IconShield, Component: Admin, area: 'admin', mobile: 'tab' },
  { path: 'admin/stats', label: 'Platform Stats', Icon: IconDashboard, Component: AdminStats, area: 'admin', mobile: 'tab' },
]

export const routeTo = r => '/' + r.path
// NavLink `end` so "/" and "/admin" aren't active on their sub-pages.
export const routeEnd = r => r.path === '' || APP_ROUTES.some(o => o !== r && o.path.startsWith(r.path + '/'))
export const navRoutes = area => APP_ROUTES.filter(r => r.area === area && r.nav !== false)
