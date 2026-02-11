import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useParams, useNavigate } from 'react-router'
import { getContacts, getContact } from '@/lib/api'
import { useAuth } from '@/hooks/use-pocketbase'
import type { Contact, ContactRole } from '@/lib/pocketbase'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Badge } from '@/components/ui/badge'
import { CardContent } from '@/components/ui/card'
import { Avatar, AvatarFallback, AvatarImage } from '@/components/ui/avatar'
import { Checkbox } from '@/components/ui/checkbox'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Plus, Search, ChevronLeft, ChevronRight, Merge, X, LayoutGrid, List, Star } from 'lucide-react'
import { cn } from '@/lib/utils'
import { ContactDrawer } from '@/components/contact-drawer'
import { MergeContactsDialog } from '@/components/merge-contacts-dialog'
import { EntityList } from '@/components/entity-list'
import { PageHeader } from '@/components/ui/page-header'

const ROLE_VARIANTS: Record<ContactRole, 'default' | 'secondary' | 'destructive' | 'outline'> = {
  presenter: 'default',
  speaker: 'default',
  sponsor: 'secondary',
  judge: 'outline',
  attendee: 'secondary',
  staff: 'secondary',
  volunteer: 'secondary',
}

function getStoredLayout(): 'list' | 'cards' {
  try {
    const v = localStorage.getItem('crm-contacts-layout')
    if (v === 'list' || v === 'cards') return v
  } catch { /* ignore */ }
  return 'list'
}

export function ContactsPage() {
  const { id } = useParams()
  const navigate = useNavigate()
  const { isAdmin } = useAuth()
  const [page, setPage] = useState(1)
  const [search, setSearch] = useState('')
  const [status, setStatus] = useState<string>('active')
  const [layout, setLayoutState] = useState<'list' | 'cards'>(getStoredLayout)
  const [drawerOpen, setDrawerOpen] = useState(!!id)
  const [selectedContact, setSelectedContact] = useState<Contact | null>(null)

  // Merge mode state
  const [mergeMode, setMergeMode] = useState(false)
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set())
  const [mergeDialogOpen, setMergeDialogOpen] = useState(false)

  const setLayout = (v: 'list' | 'cards') => {
    setLayoutState(v)
    try { localStorage.setItem('crm-contacts-layout', v) } catch { /* ignore */ }
  }

  const { data, isLoading } = useQuery({
    queryKey: ['contacts', page, search, status],
    queryFn: () => getContacts({ page, perPage: 25, search, status }),
  })

  const { data: deepLinkedContact } = useQuery({
    queryKey: ['contact', id],
    queryFn: () => getContact(id!),
    enabled: !!id,
  })

  const openContact = (contact: Contact) => {
    if (mergeMode) {
      toggleSelection(contact.id)
      return
    }
    setSelectedContact(contact)
    setDrawerOpen(true)
    navigate(`/contacts/${contact.id}`, { replace: true })
  }

  const closeDrawer = () => {
    setDrawerOpen(false)
    setSelectedContact(null)
    navigate('/contacts', { replace: true })
  }

  const handleAddNew = () => {
    setSelectedContact(null)
    setDrawerOpen(true)
  }

  const toggleMergeMode = () => {
    if (mergeMode) {
      setMergeMode(false)
      setSelectedIds(new Set())
    } else {
      setMergeMode(true)
      setSelectedIds(new Set())
    }
  }

  const toggleSelection = (contactId: string) => {
    setSelectedIds((prev) => {
      const next = new Set(prev)
      if (next.has(contactId)) {
        next.delete(contactId)
      } else {
        next.add(contactId)
      }
      return next
    })
  }

  const handleMergeClose = () => {
    setMergeDialogOpen(false)
    setMergeMode(false)
    setSelectedIds(new Set())
  }

  const initials = (name: string) =>
    name
      .split(' ')
      .map((n) => n[0])
      .join('')
      .toUpperCase()
      .slice(0, 2)

  return (
    <div className="space-y-4">
      <PageHeader title="Contacts">
        {isAdmin && (
          <>
            <Button variant={mergeMode ? 'secondary' : 'outline'} onClick={toggleMergeMode}>
              {mergeMode ? (
                <>
                  <X className="w-4 h-4 mr-1" /> Cancel
                </>
              ) : (
                <>
                  <Merge className="w-4 h-4 mr-1" /> Merge
                </>
              )}
            </Button>
            {!mergeMode && (
              <Button onClick={handleAddNew}>
                <Plus className="w-4 h-4 mr-1" /> Add contact
              </Button>
            )}
          </>
        )}
      </PageHeader>

      {/* Merge mode selection bar */}
      {mergeMode && (
        <div className="flex items-center justify-between p-3 rounded-lg border border-border bg-muted/50">
          <p className="text-sm">
            {selectedIds.size === 0
              ? 'Select contacts to merge'
              : `${selectedIds.size} contact${selectedIds.size > 1 ? 's' : ''} selected`}
          </p>
          <Button
            size="sm"
            disabled={selectedIds.size < 2}
            onClick={() => setMergeDialogOpen(true)}
          >
            <Merge className="w-4 h-4 mr-1" /> Merge selected
          </Button>
        </div>
      )}

      <div className="flex items-center gap-4">
        <div className="relative flex-1 max-w-sm">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground" />
          <Input
            placeholder="Search contacts..."
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
            <SelectItem value="inactive">Inactive</SelectItem>
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
        onItemClick={openContact}
        emptyMessage="No contacts found"
        columns={[
          ...(mergeMode ? [{
            label: '',
            className: 'w-[40px]',
            render: (contact: Contact) => (
              <Checkbox
                checked={selectedIds.has(contact.id)}
                onCheckedChange={() => toggleSelection(contact.id)}
                onClick={(e: React.MouseEvent) => e.stopPropagation()}
              />
            ),
          }] : []),
          {
            label: 'Name',
            className: 'w-[300px]',
            render: (contact: Contact) => (
              <div className="flex items-center gap-3">
                <Avatar className="h-8 w-8">
                  <AvatarImage src={contact.avatar_small_url || contact.avatar_thumb_url || contact.avatar_url} />
                  <AvatarFallback className="text-xs">
                    {initials(contact.name)}
                  </AvatarFallback>
                </Avatar>
                <div>
                  <div>{contact.name}</div>
                  <div className="text-sm text-muted-foreground">{contact.email}</div>
                </div>
              </div>
            ),
          },
          {
            label: 'Organisation',
            render: (contact: Contact) => (
              <span className="text-muted-foreground">{contact.organisation_name || '—'}</span>
            ),
          },
          {
            label: 'Roles',
            render: (contact: Contact) => (
              <div className="flex flex-wrap gap-1">
                {contact.roles?.slice(0, 2).map((role) => (
                  <Badge key={role} variant={ROLE_VARIANTS[role]}>
                    {role}
                  </Badge>
                ))}
                {(contact.roles?.length || 0) > 2 && (
                  <Badge variant="outline">+{contact.roles!.length - 2}</Badge>
                )}
              </div>
            ),
          },
          {
            label: 'Relationship',
            className: 'w-[120px]',
            render: (contact: Contact) => (
              <div className="flex gap-0.5">
                {[1, 2, 3, 4, 5].map((star) => (
                  <Star
                    key={star}
                    className={cn(
                      'h-3.5 w-3.5',
                      star <= (contact.relationship || 0)
                        ? 'fill-yellow-400 text-yellow-400'
                        : 'text-muted-foreground/20',
                    )}
                  />
                ))}
              </div>
            ),
          },
          {
            label: 'Status',
            render: (contact: Contact) => (
              <Badge
                variant={
                  contact.status === 'active'
                    ? 'default'
                    : contact.status === 'inactive'
                    ? 'secondary'
                    : 'outline'
                }
              >
                {contact.status}
              </Badge>
            ),
          },
        ]}
        renderCard={(contact) => (
          <CardContent className="flex flex-col items-center text-center pt-6">
            <Avatar className="h-16 w-16 mb-3">
              <AvatarImage src={contact.avatar_small_url || contact.avatar_thumb_url || contact.avatar_url} />
              <AvatarFallback>{initials(contact.name)}</AvatarFallback>
            </Avatar>
            <p className="text-sm line-clamp-1">{contact.name}</p>
            {contact.organisation_name && (
              <p className="text-xs text-muted-foreground line-clamp-1">{contact.organisation_name}</p>
            )}
            {mergeMode && (
              <Checkbox
                checked={selectedIds.has(contact.id)}
                onCheckedChange={() => toggleSelection(contact.id)}
                onClick={(e) => e.stopPropagation()}
                className="mt-2"
              />
            )}
          </CardContent>
        )}
      />

      {data && data.totalPages > 1 && (
        <div className="flex items-center justify-between">
          <p className="text-sm text-muted-foreground">
            Showing {(page - 1) * 25 + 1} to {Math.min(page * 25, data.totalItems)} of{' '}
            {data.totalItems} contacts
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

      <ContactDrawer
        open={drawerOpen}
        onClose={closeDrawer}
        contact={selectedContact || deepLinkedContact || null}
      />

      {mergeDialogOpen && (
        <MergeContactsDialog
          open={mergeDialogOpen}
          onClose={handleMergeClose}
          contactIds={Array.from(selectedIds)}
        />
      )}
    </div>
  )
}
