import { Routes, Route, Navigate } from 'react-router'
import { useAuth } from '@/hooks/use-pocketbase'
import { LoginPage } from '@/pages/login'
import { DashboardPage } from '@/pages/dashboard'
import { ContactsPage } from '@/pages/contacts'
import { ContactsSanitisePage } from '@/pages/contacts-sanitise'
import { OrganisationsPage } from '@/pages/organisations'
import CRMProjectionsPage from '@/pages/projections'
import { GuestListsPage } from '@/pages/guest-lists'
import { GuestListDetailPage } from '@/pages/guest-list-detail'
import { SharedGuestListPage } from '@/pages/shared-guest-list'
import { RSVPPage } from '@/pages/rsvp'
import SettingsPage from '@/pages/settings'
import ThemesPage from '@/pages/themes'
import { AppLayout } from '@/components/app-layout'

function ProtectedRoute({ children }: { children: React.ReactNode }) {
  const { isLoggedIn } = useAuth()
  if (!isLoggedIn) {
    return <Navigate to="/login" replace />
  }
  return <>{children}</>
}

const isRSVPDomain = window.location.hostname === 'rsvp.theoutlook.io'

function App() {
  return (
    <Routes>
      {isRSVPDomain && <Route path="/:token" element={<RSVPPage />} />}
      {isRSVPDomain && <Route path="/:token/forward" element={<RSVPPage />} />}
      <Route path="/login" element={<LoginPage />} />
      <Route path="/shared/:token" element={<SharedGuestListPage />} />
      <Route path="/rsvp/:token" element={<RSVPPage />} />
      <Route path="/rsvp/:token/forward" element={<RSVPPage />} />
      <Route
        path="/*"
        element={
          <ProtectedRoute>
            <AppLayout>
              <Routes>
                <Route path="/" element={<DashboardPage />} />
                <Route path="/contacts" element={<ContactsPage />} />
                <Route path="/contacts/sanitise" element={<ContactsSanitisePage />} />
                <Route path="/contacts/:id" element={<ContactsPage />} />
                <Route path="/organisations" element={<OrganisationsPage />} />
                <Route path="/organisations/:id" element={<OrganisationsPage />} />
                <Route path="/projections" element={<CRMProjectionsPage />} />
                <Route path="/guest-lists" element={<GuestListsPage />} />
                <Route path="/guest-lists/themes" element={<ThemesPage />} />
                <Route path="/guest-lists/:id" element={<GuestListDetailPage />} />
                <Route path="/settings/:tab?" element={<SettingsPage />} />
              </Routes>
            </AppLayout>
          </ProtectedRoute>
        }
      />
    </Routes>
  )
}

export default App
