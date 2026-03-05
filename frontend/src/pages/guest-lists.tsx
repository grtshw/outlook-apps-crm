import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useNavigate, useSearchParams } from 'react-router'
import { toast } from 'sonner'
import { useAuth } from '@/hooks/use-pocketbase'
import { getGuestLists, createGuestList, getEventProjections } from '@/lib/api'
import type { GuestList } from '@/lib/pocketbase'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Badge } from '@/components/ui/badge'
import { Textarea } from '@/components/ui/textarea'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Sheet, SheetContent, SheetHeader, SheetTitle, SheetFooter } from '@/components/ui/sheet'
import { Plus } from 'lucide-react'
import { PageHeader } from '@/components/ui/page-header'
import { ListView } from '@/components/ui/list-view'

const STATUS_VARIANTS: Record<string, 'default' | 'secondary' | 'outline'> = {
  draft: 'outline',
  active: 'default',
  archived: 'secondary',
}

function formatDate(dateString: string) {
  return new Date(dateString).toLocaleDateString('en-AU', {
    day: 'numeric',
    month: 'short',
    year: 'numeric',
  })
}

export function GuestListsPage() {
  const navigate = useNavigate()
  const [searchParams, setSearchParams] = useSearchParams()
  const queryClient = useQueryClient()
  const { isAdmin } = useAuth()

  const search = searchParams.get('search') || ''
  const status = searchParams.get('status') || 'all'
  const [sheetOpen, setSheetOpen] = useState(false)

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

  // Form state
  const [formName, setFormName] = useState('')
  const [formDescription, setFormDescription] = useState('')
  const [formEvent, setFormEvent] = useState('')
  const [formStatus, setFormStatus] = useState('draft')
  const [eventSearch, setEventSearch] = useState('')

  const { data, isLoading } = useQuery({
    queryKey: ['guest-lists', search, status],
    queryFn: () => getGuestLists({ search, status }),
  })

  const { data: eventProjectionsData } = useQuery({
    queryKey: ['event-projections'],
    queryFn: () => getEventProjections(),
    enabled: sheetOpen,
  })

  const filteredEvents = (eventProjectionsData?.items ?? []).filter((ep) =>
    ep.name.toLowerCase().includes(eventSearch.toLowerCase()),
  )

  const createMutation = useMutation({
    mutationFn: createGuestList,
    onSuccess: (result) => {
      queryClient.invalidateQueries({ queryKey: ['guest-lists'] })
      toast.success('Guest list created')
      resetForm()
      setSheetOpen(false)
      navigate(`/guest-lists/${result.id}`)
    },
    onError: () => {
      toast.error('Failed to create guest list')
    },
  })

  const resetForm = () => {
    setFormName('')
    setFormDescription('')
    setFormEvent('')
    setFormStatus('draft')
    setEventSearch('')
  }

  const handleCreate = () => {
    if (!formName.trim()) return
    createMutation.mutate({
      name: formName.trim(),
      description: formDescription.trim() || undefined,
      event_projection: formEvent || undefined,
      status: formStatus,
    })
  }

  const handleOpenSheet = () => {
    resetForm()
    setSheetOpen(true)
  }

  return (
    <div className="space-y-4">
      <PageHeader title="Guest lists">
        {isAdmin && (
          <Button onClick={handleOpenSheet}>
            <Plus className="w-4 h-4 mr-1" /> New guest list
          </Button>
        )}
      </PageHeader>

      <ListView
        items={data?.items ?? []}
        isLoading={isLoading}
        columns={[
          {
            label: 'Name',
            className: 'w-[250px]',
            render: (item: GuestList) => <span>{item.name}</span>,
          },
          {
            label: 'Event',
            render: (item: GuestList) => (
              <span className="text-muted-foreground">{item.event_name || '\u2014'}</span>
            ),
          },
          {
            label: 'Status',
            render: (item: GuestList) => (
              <Badge variant={STATUS_VARIANTS[item.status] ?? 'outline'}>
                {item.status}
              </Badge>
            ),
          },
          {
            label: 'Guests',
            render: (item: GuestList) => (
              <span className="text-muted-foreground">{item.item_count}</span>
            ),
          },
          {
            label: 'Shares',
            render: (item: GuestList) => (
              <span className="text-muted-foreground">{item.share_count}</span>
            ),
          },
          {
            label: 'Created',
            render: (item: GuestList) => (
              <span className="text-muted-foreground">{formatDate(item.created)}</span>
            ),
          },
        ]}
        renderCard={(item) => (
          <div className="p-4">
            <h3 className="truncate">{item.name}</h3>
            <p className="text-sm text-muted-foreground mt-1">{item.event_name || 'No event'}</p>
            <div className="flex items-center gap-2 mt-2">
              <Badge variant={STATUS_VARIANTS[item.status] ?? 'outline'}>
                {item.status}
              </Badge>
              <span className="text-xs text-muted-foreground">{item.item_count} guests</span>
            </div>
          </div>
        )}
        search={search}
        onSearchChange={(v) => updateParams({ search: v || undefined })}
        searchPlaceholder="Search guest lists..."
        extraFilters={
          <Select value={status} onValueChange={(v) => updateParams({ status: v === 'all' ? undefined : v })}>
            <SelectTrigger className="w-[140px]">
              <SelectValue placeholder="Status" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">All</SelectItem>
              <SelectItem value="draft">Draft</SelectItem>
              <SelectItem value="active">Active</SelectItem>
              <SelectItem value="archived">Archived</SelectItem>
            </SelectContent>
          </Select>
        }
        page={1}
        totalPages={1}
        totalItems={data?.items?.length ?? 0}
        perPage={100}
        onPageChange={() => {}}
        noun="guest lists"
        defaultLayout="list"
        storageKey="crm-guest-lists-layout"
        onItemClick={(item) => navigate(`/guest-lists/${item.id}`)}
        emptyTitle="No guest lists found"
      />

      <Sheet open={sheetOpen} onOpenChange={setSheetOpen}>
        <SheetContent>
          <SheetHeader>
            <SheetTitle>New guest list</SheetTitle>
          </SheetHeader>

          <div className="flex-1 space-y-4 p-6">
            <div>
              <label htmlFor="gl-name" className="block text-sm text-muted-foreground mb-1.5">
                Name *
              </label>
              <Input
                id="gl-name"
                value={formName}
                onChange={(e) => setFormName(e.target.value)}
                placeholder="e.g. VIP guests"
              />
            </div>

            <div>
              <label htmlFor="gl-description" className="block text-sm text-muted-foreground mb-1.5">
                Description
              </label>
              <Textarea
                id="gl-description"
                value={formDescription}
                onChange={(e) => setFormDescription(e.target.value)}
                placeholder="Optional description"
                rows={3}
              />
            </div>

            <div>
              <label className="block text-sm text-muted-foreground mb-1.5">
                Event
              </label>
              <Select value={formEvent} onValueChange={setFormEvent}>
                <SelectTrigger>
                  <SelectValue placeholder="Select an event" />
                </SelectTrigger>
                <SelectContent>
                  <div className="p-2">
                    <Input
                      placeholder="Search events..."
                      value={eventSearch}
                      onChange={(e) => setEventSearch(e.target.value)}
                      className="mb-2"
                    />
                  </div>
                  {filteredEvents.map((ep) => (
                    <SelectItem key={ep.id} value={ep.id}>
                      {ep.name}
                    </SelectItem>
                  ))}
                  {filteredEvents.length === 0 && (
                    <p className="py-4 text-center text-sm text-muted-foreground">No events found</p>
                  )}
                </SelectContent>
              </Select>
            </div>

            <div>
              <label className="block text-sm text-muted-foreground mb-1.5">
                Status
              </label>
              <Select value={formStatus} onValueChange={setFormStatus}>
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="draft">Draft</SelectItem>
                  <SelectItem value="active">Active</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>

          <SheetFooter>
            <Button variant="outline" onClick={() => setSheetOpen(false)}>
              Cancel
            </Button>
            <Button
              onClick={handleCreate}
              disabled={!formName.trim() || createMutation.isPending}
            >
              {createMutation.isPending ? 'Creating...' : 'Create'}
            </Button>
          </SheetFooter>
        </SheetContent>
      </Sheet>
    </div>
  )
}
