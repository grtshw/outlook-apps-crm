import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useParams, useNavigate } from 'react-router'
import { getOrganisations, getOrganisation } from '@/lib/api'
import { useAuth } from '@/hooks/use-pocketbase'
import type { Organisation } from '@/lib/pocketbase'
import { CardContent } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Badge } from '@/components/ui/badge'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Plus, Search, ChevronLeft, ChevronRight, Building2, LayoutGrid, List } from 'lucide-react'
import { cn } from '@/lib/utils'
import { OrganisationDrawer } from '@/components/organisation-drawer'
import { EntityList } from '@/components/entity-list'
import { PageHeader } from '@/components/ui/page-header'

function getStoredLayout(): 'list' | 'cards' {
  try {
    const v = localStorage.getItem('crm-orgs-layout')
    if (v === 'list' || v === 'cards') return v
  } catch { /* ignore */ }
  return 'list'
}

export function OrganisationsPage() {
  const { id } = useParams()
  const navigate = useNavigate()
  const { isAdmin } = useAuth()
  const [page, setPage] = useState(1)
  const [search, setSearch] = useState('')
  const [status, setStatus] = useState<string>('active')
  const [layout, setLayoutState] = useState<'list' | 'cards'>(getStoredLayout)
  const [drawerOpen, setDrawerOpen] = useState(!!id)
  const [selectedOrg, setSelectedOrg] = useState<Organisation | null>(null)

  const setLayout = (v: 'list' | 'cards') => {
    setLayoutState(v)
    try { localStorage.setItem('crm-orgs-layout', v) } catch { /* ignore */ }
  }

  const { data, isLoading } = useQuery({
    queryKey: ['organisations', page, search, status],
    queryFn: () => getOrganisations({ page, perPage: 24, search, status }),
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

      <div className="flex items-center gap-4">
        <div className="relative flex-1 max-w-sm">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground" />
          <Input
            placeholder="Search organisations..."
            value={search}
            onChange={(e) => {
              setSearch(e.target.value)
              setPage(1)
            }}
            className="pl-9"
          />
        </div>
        <Select
          value={status}
          onValueChange={(v) => {
            setStatus(v)
            setPage(1)
          }}
        >
          <SelectTrigger className="w-[140px]">
            <SelectValue placeholder="Status" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">All</SelectItem>
            <SelectItem value="active">Active</SelectItem>
            <SelectItem value="archived">Archived</SelectItem>
          </SelectContent>
        </Select>
        <div className="flex items-center gap-1 border rounded-md p-0.5">
          <Button
            variant="ghost"
            size="icon"
            className={cn('h-7 w-7', layout === 'list' && 'bg-accent')}
            onClick={() => setLayout('list')}
            title="List view"
          >
            <List className="h-4 w-4" />
          </Button>
          <Button
            variant="ghost"
            size="icon"
            className={cn('h-7 w-7', layout === 'cards' && 'bg-accent')}
            onClick={() => setLayout('cards')}
            title="Card view"
          >
            <LayoutGrid className="h-4 w-4" />
          </Button>
        </div>
      </div>

      <EntityList
        items={data?.items ?? []}
        isLoading={isLoading}
        layout={layout}
        getName={(org) => org.name}
        onItemClick={openOrg}
        emptyMessage="No organisations found"
        renderListItem={(org) => (
          <>
            <div className="h-8 w-8 rounded bg-muted flex items-center justify-center overflow-hidden shrink-0">
              {org.logo_square_url || org.logo_standard_url ? (
                <img
                  src={org.logo_square_url || org.logo_standard_url}
                  alt={org.name}
                  className="max-w-full max-h-full object-contain"
                />
              ) : (
                <Building2 className="w-4 h-4 text-muted-foreground" />
              )}
            </div>
            <div className="flex-1 min-w-0">
              <div className="truncate">{org.name}</div>
            </div>
            {org.status === 'archived' && (
              <Badge variant="secondary">Archived</Badge>
            )}
          </>
        )}
        renderCard={(org) => (
          <CardContent className="flex flex-col items-center text-center">
            <div className="w-full aspect-square flex items-center justify-center mb-3 rounded-lg bg-muted overflow-hidden">
              {org.logo_square_url || org.logo_standard_url ? (
                <img
                  src={org.logo_square_url || org.logo_standard_url}
                  alt={org.name}
                  className="max-w-full max-h-full object-contain p-2"
                />
              ) : (
                <Building2 className="w-12 h-12 text-muted-foreground" />
              )}
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

      {data && data.totalPages > 1 && (
        <div className="flex items-center justify-between">
          <p className="text-sm text-muted-foreground">
            Showing {(page - 1) * 24 + 1} to {Math.min(page * 24, data.totalItems)} of{' '}
            {data.totalItems} organisations
          </p>
          <div className="flex items-center gap-2">
            <Button
              variant="outline"
              size="sm"
              onClick={() => setPage((p) => Math.max(1, p - 1))}
              disabled={page === 1}
            >
              <ChevronLeft className="w-4 h-4" />
            </Button>
            <span className="text-sm">
              Page {page} of {data.totalPages}
            </span>
            <Button
              variant="outline"
              size="sm"
              onClick={() => setPage((p) => Math.min(data.totalPages, p + 1))}
              disabled={page === data.totalPages}
            >
              <ChevronRight className="w-4 h-4" />
            </Button>
          </div>
        </div>
      )}

      <OrganisationDrawer
        open={drawerOpen}
        onClose={closeDrawer}
        organisation={selectedOrg || deepLinkedOrg || null}
      />
    </div>
  )
}
