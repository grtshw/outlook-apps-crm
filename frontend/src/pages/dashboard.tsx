import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router'
import { getDashboardStats } from '@/lib/api'
import { useAuth } from '@/hooks/use-pocketbase'
import { Card, CardContent } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { Users, Building2, Activity, Plus } from 'lucide-react'

export function DashboardPage() {
  const { user } = useAuth()
  const { data: stats, isLoading } = useQuery({
    queryKey: ['dashboard-stats'],
    queryFn: getDashboardStats,
  })

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl">Dashboard</h1>
        {user && (
          <p className="text-muted-foreground">
            Welcome back, {user.name || user.email}
          </p>
        )}
      </div>

      {isLoading ? (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
          {[1, 2, 3].map((i) => (
            <Skeleton key={i} className="h-36" />
          ))}
        </div>
      ) : stats ? (
        <>
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
            <Link to="/contacts" className="block">
              <Card className="hover:shadow-md transition-shadow">
                <CardContent className="p-6">
                  <div className="flex items-center gap-4 mb-4">
                    <div className="w-12 h-12 rounded-full bg-brand-green/10 flex items-center justify-center">
                      <Users className="w-6 h-6 text-brand-green" />
                    </div>
                    <div>
                      <h3 className="text-lg">Contacts</h3>
                      <p className="text-3xl">{stats.contacts.total}</p>
                    </div>
                  </div>
                  <div className="flex gap-4 text-sm text-muted-foreground">
                    <span>{stats.contacts.active} active</span>
                    <span>{stats.contacts.inactive} inactive</span>
                    <span>{stats.contacts.archived} archived</span>
                  </div>
                </CardContent>
              </Card>
            </Link>

            <Link to="/organisations" className="block">
              <Card className="hover:shadow-md transition-shadow">
                <CardContent className="p-6">
                  <div className="flex items-center gap-4 mb-4">
                    <div className="w-12 h-12 rounded-full bg-brand-purple/10 flex items-center justify-center">
                      <Building2 className="w-6 h-6 text-brand-purple" />
                    </div>
                    <div>
                      <h3 className="text-lg">Organisations</h3>
                      <p className="text-3xl">{stats.organisations.total}</p>
                    </div>
                  </div>
                  <div className="flex gap-4 text-sm text-muted-foreground">
                    <span>{stats.organisations.active} active</span>
                    <span>{stats.organisations.archived} archived</span>
                  </div>
                </CardContent>
              </Card>
            </Link>

            <Card>
              <CardContent className="p-6">
                <div className="flex items-center gap-4 mb-4">
                  <div className="w-12 h-12 rounded-full bg-amber-100 flex items-center justify-center">
                    <Activity className="w-6 h-6 text-amber-600" />
                  </div>
                  <div>
                    <h3 className="text-lg">Recent activity</h3>
                    <p className="text-3xl">{stats.recent_activities}</p>
                  </div>
                </div>
                <p className="text-sm text-muted-foreground">
                  Activities in the last 30 days
                </p>
              </CardContent>
            </Card>
          </div>

          <Card>
            <CardContent className="p-6">
              <h2 className="text-lg mb-4">Quick actions</h2>
              <div className="flex flex-wrap gap-3">
                <Button variant="secondary" asChild>
                  <Link to="/contacts">
                    <Plus className="w-4 h-4 mr-1" /> Add contact
                  </Link>
                </Button>
                <Button variant="secondary" asChild>
                  <Link to="/organisations">
                    <Plus className="w-4 h-4 mr-1" /> Add organisation
                  </Link>
                </Button>
              </div>
            </CardContent>
          </Card>
        </>
      ) : (
        <p className="text-muted-foreground">Failed to load dashboard data.</p>
      )}
    </div>
  )
}
