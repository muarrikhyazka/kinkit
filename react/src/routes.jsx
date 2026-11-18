import Dashboard from './pages/Dashboard.jsx'
import Budgeting from './pages/Budgeting.jsx'
import Spending from './pages/Spending.jsx'
import Assets from './pages/Assets.jsx'
import Setup from './pages/Setup.jsx'
import Guide from './pages/Guide.jsx'
import Integrations from './pages/Integrations.jsx'
import Admin from './pages/Admin.jsx'
import AdminStats from './pages/AdminStats.jsx'
import Recurring from './pages/Recurring.jsx'
import Income from './pages/Income.jsx'
import Goals from './pages/Goals.jsx'
import Family, { IconFamily } from './pages/Family.jsx'
import ImportExport from './pages/ImportExport.jsx'
import DisplaySettings from './pages/DisplaySettings.jsx'
import Today from './pages/Today.jsx'
import Reminders from './pages/Reminders.jsx'
import Chores from './pages/Chores.jsx'
import Lists from './pages/Lists.jsx'
import { IconGoal } from './moneyIcons.jsx'
import { IconDashboard, IconBudget, IconSpending, IconSetup, IconGuide, IconShield, IconToday, IconHome, IconBell } from './icons.jsx'

/**
 * SINGLE SOURCE OF TRUTH for in-app pages. App.jsx builds the <Route>s from
 * this list and Layout.jsx builds the sidebar, mobile bottom bar and "More"
 * sheet from it.
 *
 *   path:      route path under "/" ('' = index)
 *   label:     nav label
 *   Icon:      nav icon
 *   Component: page component (single-page section)
 *   tabs:      [{ path, label, Component, roles? }] - section with tabs at
 *              /<path>/<tab.path>; "/<path>" redirects to the first tab the
 *              user may see. Use instead of Component.
 *   area:      'tenant' (household pages; super admin is redirected away)
 *              | 'admin' (platform admin pages)
 *   roles:     household roles allowed to see it (default: all). The backend
 *              enforces permissions anyway - this only hides useless menus.
 *   mobile:    'tab' -> bottom tab bar (keep to ~4) | 'more' -> "More" sheet
 *   placement: 'bottom' -> pinned to the sidebar bottom (settings-like)
 *   group:     sidebar group heading shown above the first item of a run
 *              of items with the same group (e.g. 'Money')
 *   nav:       false to register the route without any nav entry
 */
const ALL = ['owner', 'parent', 'kid', 'viewer']
const MANAGERS = ['owner', 'parent']
const READERS = ['owner', 'parent', 'viewer']

export const APP_ROUTES = [
  { path: '', label: 'Today', Icon: IconToday, Component: Today, area: 'tenant', mobile: 'tab' },
  { path: 'reminders', label: 'Reminders', Icon: IconBell, Component: Reminders, area: 'tenant', mobile: 'tab' },
  {
    path: 'home', label: 'Home', Icon: IconHome, area: 'tenant', mobile: 'tab',
    tabs: [
      { path: 'chores', label: 'Chores', Component: Chores },
      { path: 'lists', label: 'Lists', Component: Lists },
    ],
  },
  { path: 'dashboard', label: 'Dashboard', Icon: IconDashboard, Component: Dashboard, area: 'tenant', mobile: 'more', group: 'Money' },
  {
    path: 'transactions', label: 'Transactions', Icon: IconSpending, area: 'tenant', mobile: 'tab', group: 'Money',
    tabs: [
      { path: 'spending', label: 'Spending', Component: Spending },
      { path: 'income', label: 'Income', Component: Income, roles: READERS },
      { path: 'recurring', label: 'Recurring', Component: Recurring, roles: READERS },
    ],
  },
  { path: 'budget', label: 'Budget', Icon: IconBudget, Component: Budgeting, area: 'tenant', mobile: 'more', roles: READERS, group: 'Money' },
  {
    path: 'savings', label: 'Savings', Icon: IconGoal, area: 'tenant', mobile: 'more', roles: READERS, group: 'Money',
    tabs: [
      { path: 'goals', label: 'Goals', Component: Goals },
      { path: 'assets', label: 'Assets', Component: Assets },
    ],
  },
  { path: 'family', label: 'Family', Icon: IconFamily, Component: Family, area: 'tenant', mobile: 'more', group: 'Family' },
  {
    path: 'settings', label: 'Settings', Icon: IconSetup, area: 'tenant', mobile: 'more', placement: 'bottom',
    tabs: [
      { path: 'general', label: 'Categories & accounts', Component: Setup, roles: MANAGERS },
      { path: 'bots', label: 'Bots', Component: Integrations, roles: MANAGERS },
      { path: 'data', label: 'Data & devices', Component: ImportExport, roles: MANAGERS },
      { path: 'display', label: 'Display', Component: DisplaySettings },
    ],
  },
  { path: 'help', label: 'Help', Icon: IconGuide, Component: Guide, area: 'tenant', nav: false },

  // Super admin manages the platform, not personal finances.
  { path: 'admin', label: 'Accounts', Icon: IconShield, Component: Admin, area: 'admin', mobile: 'tab' },
  { path: 'admin/stats', label: 'Platform Stats', Icon: IconDashboard, Component: AdminStats, area: 'admin', mobile: 'tab' },
]

// Old URLs (bookmarks, links in bot messages) keep working.
export const LEGACY_REDIRECTS = {
  spending: '/transactions/spending',
  income: '/transactions/income',
  recurring: '/transactions/recurring',
  budgeting: '/budget',
  goals: '/savings/goals',
  assets: '/savings/assets',
  setup: '/settings/general',
  integrations: '/settings/bots',
  'import-export': '/settings/data',
  guide: '/help',
  'home/reminders': '/reminders', // Reminders moved from a Home tab to its own page
}

const roleOf = user => user?.role || 'owner' // legacy sessions without a role are the owner
export const canSee = (item, user) => (item.roles || ALL).includes(roleOf(user))
export const visibleTabs = (route, user) => (route.tabs || []).filter(t => canSee(t, user))

export const routeTo = r => '/' + r.path
// NavLink `end` so "/" and "/admin" aren't active on their sub-pages.
export const routeEnd = r => r.path === '' || APP_ROUTES.some(o => o !== r && o.path.startsWith(r.path + '/'))
export const navRoutes = (area, user) => APP_ROUTES.filter(r =>
  r.area === area && r.nav !== false && canSee(r, user) && (!r.tabs || visibleTabs(r, user).length > 0))
