import { useEffect, useMemo, useState } from 'react'
import { useNavigate, useLocation } from 'react-router'
import { useAuth } from '@/hooks/use-pocketbase'
import { pb } from '@/lib/pocketbase'
import { getContacts, getOrganisations } from '@/lib/api'
import {
  AppSidebar,
  type EcosystemApp,
  type NavGroup,
  type SearchConfig,
  type SearchResult,
  type DomainAction,
} from '@/components/ui/app-sidebar'
import { LayoutDashboard, Users, Building2, Settings } from 'lucide-react'

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
            app_id: r['app_id'] as string,
            app_name: r['app_name'] as string,
            app_url: r['app_url'] as string,
            app_icon: r['app_icon'] as string,
            sort_order: r['sort_order'] as number,
            is_active: r['is_active'] as boolean,
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

  const appName = apps.find((a) => a.app_id === 'crm')?.app_name ?? 'CRM'

  useEffect(() => {
    for (const group of navigation) {
      const match = group.items.find((item) => isActive(item.href))
      if (match) {
        const parts = [match.name, group.label, appName].filter(Boolean)
        document.title = parts.join(' – ')
        return
      }
    }
    document.title = appName
  }, [location.pathname, appName])

  const searchConfig: SearchConfig = useMemo(
    () => ({
      placeholder: 'Search contacts & organisations...',
      onSearch: async (query: string) => {
        const [contacts, orgs] = await Promise.all([
          getContacts({ search: query, perPage: 5 }).catch(() => ({ items: [] })),
          getOrganisations({ search: query, perPage: 5 }).catch(() => ({ items: [] })),
        ])
        const results: SearchResult[] = []
        for (const c of contacts.items) {
          results.push({
            id: c.id,
            label: c.name,
            subtitle: c.email || c.organisation_name,
            icon: Users,
            href: `/contacts/${c.id}`,
            category: 'Contacts',
          })
        }
        for (const o of orgs.items) {
          results.push({
            id: o.id,
            label: o.name,
            subtitle: o.status,
            icon: Building2,
            href: `/organisations/${o.id}`,
            category: 'Organisations',
          })
        }
        return results
      },
      onSelect: (result: SearchResult) => {
        if (result.href) navigate(result.href)
      },
    }),
    [navigate]
  )

  const domainActions: DomainAction[] = useMemo(
    () => [
      {
        id: 'settings',
        icon: Settings,
        tooltip: 'PocketBase admin',
        href: '/_/',
      },
    ],
    []
  )

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
      search={searchConfig}
      domainActions={domainActions}
    >
      {children}
    </AppSidebar>
  )
}
