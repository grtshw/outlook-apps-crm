import { Routes, Route, Navigate } from 'react-router'
import { useAuth } from '@/hooks/use-pocketbase'
import { LoginPage } from '@/pages/login'
import { DashboardPage } from '@/pages/dashboard'
import { ContactsPage } from '@/pages/contacts'
import { OrganisationsPage } from '@/pages/organisations'
import CRMProjectionsPage from '@/pages/projections'
import { GuestListsPage } from '@/pages/guest-lists'
import { GuestListDetailPage } from '@/pages/guest-list-detail'
import { SharedGuestListPage } from '@/pages/shared-guest-list'
import { AppLayout } from '@/components/app-layout'

function ProtectedRoute({ children }: { children: React.ReactNode }) {
  const { isLoggedIn } = useAuth()
  if (!isLoggedIn) {
    return <Navigate to="/login" replace />
  }
  return <>{children}</>
}

function App() {
  return (
    <Routes>
      <Route path="/login" element={<LoginPage />} />
      <Route path="/shared/:token" element={<SharedGuestListPage />} />
      <Route
        path="/*"
        element={
          <ProtectedRoute>
            <AppLayout>
              <Routes>
                <Route path="/" element={<DashboardPage />} />
                <Route path="/contacts" element={<ContactsPage />} />
                <Route path="/contacts/:id" element={<ContactsPage />} />
                <Route path="/organisations" element={<OrganisationsPage />} />
                <Route path="/organisations/:id" element={<OrganisationsPage />} />
                <Route path="/projections" element={<CRMProjectionsPage />} />
                <Route path="/guest-lists" element={<GuestListsPage />} />
                <Route path="/guest-lists/:id" element={<GuestListDetailPage />} />
              </Routes>
            </AppLayout>
          </ProtectedRoute>
        }
      />
    </Routes>
  )
}

export default App
