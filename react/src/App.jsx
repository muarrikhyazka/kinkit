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
import { APP_ROUTES } from './routes.jsx'

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
        {APP_ROUTES.map(({ path, Component, area }) => {
          const el = area === 'tenant' ? <AdminGuard><Component /></AdminGuard> : <Component />
          return path === ''
            ? <Route key="index" index element={el} />
            : <Route key={path} path={path} element={el} />
        })}
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
