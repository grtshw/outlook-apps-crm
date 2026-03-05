import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useParams, useNavigate, useSearchParams } from 'react-router'
import { getOrganisations, getOrganisation } from '@/lib/api'
import { useAuth } from '@/hooks/use-pocketbase'
import type { Organisation } from '@/lib/pocketbase'
import { CardContent } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Plus } from 'lucide-react'
import { OrgLogo } from '@/components/org-logo'
import { OrganisationDrawer } from '@/components/organisation-drawer'
import { PageHeader } from '@/components/ui/page-header'
import { ListView } from '@/components/ui/list-view'

const PER_PAGE = 24

export function OrganisationsPage() {
  const { id } = useParams()
  const navigate = useNavigate()
  const [searchParams, setSearchParams] = useSearchParams()
  const { isAdmin } = useAuth()

  const page = Number(searchParams.get('page')) || 1
  const search = searchParams.get('search') || ''
  const status = searchParams.get('status') || 'active'

  const [drawerOpen, setDrawerOpen] = useState(!!id)
  const [selectedOrg, setSelectedOrg] = useState<Organisation | null>(null)

  function updateParams(updates: Record<string, string | undefined>) {
    setSearchParams((prev) => {
      const next = new URLSearchParams(prev)
      for (const [key, val] of Object.entries(updates)) {
        if (val === undefined || val === '') next.delete(key)
        else next.set(key, val)
      }
      return next
    }, { replace: true })
  }

  const { data, isLoading } = useQuery({
    queryKey: ['organisations', page, search, status],
    queryFn: () => getOrganisations({ page, perPage: PER_PAGE, search, status }),
  })

  const { data: deepLinkedOrg } = useQuery({
    queryKey: ['organisation', id],
    queryFn: () => getOrganisation(id!),
    enabled: !!id,
  })

  const openOrg = (org: Organisation) => {
    setSelectedOrg(org)
    setDrawerOpen(true)
    navigate(`/organisations/${org.id}`, { replace: true })
  }

  const closeDrawer = () => {
    setDrawerOpen(false)
    setSelectedOrg(null)
    navigate('/organisations', { replace: true })
  }

  const handleAddNew = () => {
    setSelectedOrg(null)
    setDrawerOpen(true)
  }

  return (
    <div className="space-y-4">
      <PageHeader title="Organisations">
        {isAdmin && (
          <Button onClick={handleAddNew}>
            <Plus className="w-4 h-4 mr-1" /> Add organisation
          </Button>
        )}
      </PageHeader>

      <ListView
        items={data?.items ?? []}
        isLoading={isLoading}
        columns={[
          {
            label: 'Organisation',
            render: (org: Organisation) => (
              <div className="flex items-center gap-3">
                <OrgLogo org={org} size={32} iconSize={16} />
                <span>{org.name}</span>
              </div>
            ),
          },
          {
            label: 'Industry',
            render: (org: Organisation) => {
              if (!org.industry) return <span className="text-muted-foreground">—</span>
              const labels: Record<string, string> = {
                technology: 'Technology', media: 'Media', finance: 'Finance',
                healthcare: 'Healthcare', education: 'Education', government: 'Government',
                retail: 'Retail', manufacturing: 'Manufacturing', hospitality: 'Hospitality',
                real_estate: 'Real estate', energy: 'Energy',
                professional_services: 'Professional services', non_profit: 'Non-profit',
                sports: 'Sports', entertainment: 'Entertainment', other: 'Other',
              }
              return <span>{labels[org.industry] ?? org.industry}</span>
            },
          },
          {
            label: 'Status',
            render: (org: Organisation) => (
              org.status === 'archived'
                ? <Badge variant="secondary">Archived</Badge>
                : <Badge variant="default">Active</Badge>
            ),
          },
        ]}
        renderCard={(org) => (
          <CardContent className="flex flex-col items-center text-center">
            <div className="w-full aspect-square flex items-center justify-center mb-3 rounded-lg bg-muted overflow-hidden">
              <OrgLogo org={org} size="full" iconSize={48} className="p-2" />
            </div>
            <p className="text-sm line-clamp-2">{org.name}</p>
            {org.status === 'archived' && (
              <Badge variant="secondary" className="mt-2">
                Archived
              </Badge>
            )}
          </CardContent>
        )}
        search={search}
        onSearchChange={(v) => updateParams({ search: v || undefined, page: undefined })}
        searchPlaceholder="Search organisations..."
        extraFilters={
          <Select value={status} onValueChange={(v) => updateParams({ status: v === 'active' ? undefined : v, page: undefined })}>
            <SelectTrigger className="w-[140px]">
              <SelectValue placeholder="Status" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">All</SelectItem>
              <SelectItem value="active">Active</SelectItem>
              <SelectItem value="archived">Archived</SelectItem>
            </SelectContent>
          </Select>
        }
        page={page}
        totalPages={data?.totalPages ?? 0}
        totalItems={data?.totalItems ?? 0}
        perPage={PER_PAGE}
        onPageChange={(p) => updateParams({ page: p === 1 ? undefined : String(p) })}
        noun="organisations"
        storageKey="crm-orgs-layout"
        onItemClick={openOrg}
        emptyTitle="No organisations found"
      />

      <OrganisationDrawer
        open={drawerOpen}
        onClose={closeDrawer}
        organisation={selectedOrg || deepLinkedOrg || null}
      />
    </div>
  )
}
