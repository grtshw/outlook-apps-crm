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
import { SearchInput } from '@/components/ui/search-input'
import { FilterBar, FilterBarSpacer } from '@/components/ui/filter-bar'
import { ViewToggle, type ViewLayout } from '@/components/ui/view-toggle'
import { EntityList } from '@/components/ui/entity-list'
import { ListPagination } from '@/components/ui/list-pagination'

const PER_PAGE = 24

function getStoredLayout(): ViewLayout {
  try {
    const v = localStorage.getItem('crm-orgs-layout')
    if (v === 'list' || v === 'cards') return v
  } catch { /* ignore */ }
  return 'list'
}

export function OrganisationsPage() {
  const { id } = useParams()
  const navigate = useNavigate()
  const [searchParams, setSearchParams] = useSearchParams()
  const { isAdmin } = useAuth()

  const page = Number(searchParams.get('page')) || 1
  const search = searchParams.get('search') || ''
  const status = searchParams.get('status') || 'active'

  const [layout, setLayoutState] = useState<ViewLayout>(getStoredLayout)
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

  function setSearch(value: string) {
    updateParams({ search: value || undefined, page: undefined })
  }

  function setStatus(value: string) {
    updateParams({ status: value === 'active' ? undefined : value, page: undefined })
  }

  function setPage(p: number) {
    updateParams({ page: p === 1 ? undefined : String(p) })
  }

  const setLayout = (v: ViewLayout) => {
    setLayoutState(v)
    try { localStorage.setItem('crm-orgs-layout', v) } catch { /* ignore */ }
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

      <FilterBar>
        <SearchInput
          value={search}
          onValueChange={setSearch}
          placeholder="Search organisations..."
          className="flex-1 max-w-sm"
        />
        <Select value={status} onValueChange={setStatus}>
          <SelectTrigger className="w-[140px]">
            <SelectValue placeholder="Status" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">All</SelectItem>
            <SelectItem value="active">Active</SelectItem>
            <SelectItem value="archived">Archived</SelectItem>
          </SelectContent>
        </Select>
        <FilterBarSpacer />
        <ViewToggle value={layout} onValueChange={setLayout} />
      </FilterBar>

      <EntityList
        items={data?.items ?? []}
        isLoading={isLoading}
        layout={layout}
        onItemClick={openOrg}
        emptyTitle="No organisations found"
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
      />

      <ListPagination
        page={page}
        totalPages={data?.totalPages ?? 0}
        totalItems={data?.totalItems ?? 0}
        perPage={PER_PAGE}
        onPageChange={setPage}
        noun="organisations"
      />

      <OrganisationDrawer
        open={drawerOpen}
        onClose={closeDrawer}
        organisation={selectedOrg || deepLinkedOrg || null}
      />
    </div>
  )
}
