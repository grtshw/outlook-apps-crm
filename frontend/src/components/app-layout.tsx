import { useEffect, useState } from 'react'
import { useNavigate, useLocation } from 'react-router'
import { useAuth } from '@/hooks/use-pocketbase'
import { pb } from '@/lib/pocketbase'
import {
  AppSidebar,
  type EcosystemApp,
  type NavGroup,
} from '@/components/ui/app-sidebar'
import { LayoutDashboard, Users, Building2 } from 'lucide-react'

const FALLBACK_APPS: EcosystemApp[] = [
  { app_id: 'events', app_name: 'Events', app_url: 'https://events.theoutlook.io', app_icon: 'calendar4-event', sort_order: 1, is_active: true },
  { app_id: 'presentations', app_name: 'Presentations', app_url: 'https://presentations.theoutlook.io', app_icon: 'easel3', sort_order: 2, is_active: true },
  { app_id: 'crm', app_name: 'CRM', app_url: 'https://crm.theoutlook.io', app_icon: 'person-vcard', sort_order: 3, is_active: true },
  { app_id: 'awards', app_name: 'Awards', app_url: 'https://awards.theoutlook.io/dashboard', app_icon: 'trophy', sort_order: 4, is_active: true },
  { app_id: 'dam', app_name: 'DAM', app_url: 'https://dam.theoutlook.io', app_icon: 'images', sort_order: 5, is_active: true },
]

const navigation: NavGroup[] = [
  {
    items: [
      { name: 'Dashboard', href: '/', icon: LayoutDashboard },
      { name: 'Contacts', href: '/contacts', icon: Users },
      { name: 'Organisations', href: '/organisations', icon: Building2 },
    ],
  },
]

export function AppLayout({ children }: { children: React.ReactNode }) {
  const navigate = useNavigate()
  const location = useLocation()
  const { user, logout } = useAuth()
  const [apps, setApps] = useState<EcosystemApp[]>(FALLBACK_APPS)

  useEffect(() => {
    pb.collection('app_settings')
      .getFullList({ sort: 'sort_order', filter: 'is_active=true' })
      .then((records) => {
        setApps(
          records.map((r) => ({
            app_id: r.get<string>('app_id'),
            app_name: r.get<string>('app_name'),
            app_url: r.get<string>('app_url'),
            app_icon: r.get<string>('app_icon'),
            sort_order: r.get<number>('sort_order'),
            is_active: r.get<boolean>('is_active'),
          }))
        )
      })
      .catch(() => {
        // keep fallback
      })
  }, [])

  const isActive = (href: string) => {
    if (href === '/') return location.pathname === '/'
    return location.pathname.startsWith(href)
  }

  return (
    <AppSidebar
      currentAppId="crm"
      apps={apps}
      navGroups={navigation}
      user={{
        name: user?.name ?? '',
        email: user?.email ?? '',
        avatarUrl: user?.avatar
          ? `/api/files/${user.collectionId}/${user.id}/${user.avatar}`
          : undefined,
        role: user?.role === 'admin' ? 'Administrator' : 'Viewer',
      }}
      isActive={isActive}
      onNavigate={(href) => navigate(href)}
      onSignOut={() => {
        logout()
        navigate('/login')
      }}
    >
      {children}
    </AppSidebar>
  )
}
