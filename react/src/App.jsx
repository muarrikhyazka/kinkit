import { Routes, Route, Navigate, useSearchParams } from 'react-router-dom'
import { AuthProvider, useAuth } from './AuthContext.jsx'
import { YearProvider } from './YearContext.jsx'
import { OfflineProvider } from './OfflineContext.jsx'
import { ThemeProvider } from './ThemeContext.jsx'
import { ToastProvider } from './Toast.jsx'
import { SkeletonPage } from './Skeleton.jsx'
import Layout from './Layout.jsx'
import Login from './pages/Login.jsx'
import TVView from './pages/TVView.jsx'
import { APP_ROUTES, LEGACY_REDIRECTS, canSee } from './routes.jsx'
import TabbedSection from './TabbedSection.jsx'

function Private({ children }) {
  const { user } = useAuth()
  if (user === undefined) return <div className="content"><SkeletonPage /></div>
  if (user === null) return <Navigate to="/login" replace />
  return children
}

// Super admin has no personal-finance data of their own - send them
// straight to the platform pages instead of an empty Dashboard/Spending/etc.
function AdminGuard({ children }) {
  const { user } = useAuth()
  if (user?.is_admin) return <Navigate to="/admin" replace />
  return children
}

// Pages hidden for this household role (e.g. Budget for kids) go home.
function RoleGuard({ route, children }) {
  const { user } = useAuth()
  if (!canSee(route, user)) return <Navigate to="/" replace />
  return children
}

// --- data --- /tv?k=<share token> works without logging in (TV / car screen)
function TVRoute() {
  const [params] = useSearchParams()
  const k = params.get('k')
  return k ? <TVView token={k} /> : <Private><TVView /></Private>
}
// --- end data ---

function AppRoutes() {
  return (
    <Routes>
      <Route path="/login" element={<Login />} />
      <Route path="/tv" element={<TVRoute />} />
      <Route path="/" element={<Private><Layout /></Private>}>
        {/* pages come from routes.jsx - add new pages there */}
        {APP_ROUTES.map(route => {
          const { path, Component, area, tabs } = route
          const guard = el => area === 'tenant'
            ? <AdminGuard><RoleGuard route={route}>{el}</RoleGuard></AdminGuard>
            : el
          if (tabs) {
            return (
              <Route key={path} path={path} element={guard(<TabbedSection route={route} />)}>
                <Route index element={null} />
                {tabs.map(t => <Route key={t.path} path={t.path} element={<t.Component />} />)}
              </Route>
            )
          }
          return path === ''
            ? <Route key="index" index element={guard(<Component />)} />
            : <Route key={path} path={path} element={guard(<Component />)} />
        })}
        {Object.entries(LEGACY_REDIRECTS).map(([from, to]) => (
          <Route key={from} path={from} element={<Navigate to={to} replace />} />
        ))}
      </Route>
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  )
}

export default function App() {
  return (
    <ThemeProvider>
      <ToastProvider>
        <AuthProvider>
          <YearProvider>
            <OfflineProvider>
              <AppRoutes />
            </OfflineProvider>
          </YearProvider>
        </AuthProvider>
      </ToastProvider>
    </ThemeProvider>
  )
}
