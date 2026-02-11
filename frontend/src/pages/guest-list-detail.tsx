import { useState } from 'react'
import { useParams, useNavigate } from 'react-router'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { useAuth } from '@/hooks/use-pocketbase'
import {
  getGuestList, updateGuestList, deleteGuestList,
  getGuestListItems, updateGuestListItem, deleteGuestListItem,
  getGuestListShares, revokeGuestListShare,
  getEventProjections,
  getContact,
} from '@/lib/api'
import type { Contact, GuestListItem, GuestListShare } from '@/lib/pocketbase'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Badge } from '@/components/ui/badge'
import { Textarea } from '@/components/ui/textarea'
import {
  Select, SelectContent, SelectItem, SelectTrigger, SelectValue,
} from '@/components/ui/select'
import {
  Sheet, SheetContent, SheetHeader, SheetTitle, SheetFooter,
} from '@/components/ui/sheet'
import {
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
} from '@/components/ui/table'
import { PageHeader } from '@/components/ui/page-header'
import { ContactSearchDrawer } from '@/components/contact-search-dialog'
import { ContactDrawer } from '@/components/contact-drawer'
import { ShareDialog } from '@/components/share-dialog'
import { Avatar, AvatarImage, AvatarFallback } from '@/components/ui/avatar'
import { Pencil, Share2, Trash2, X, ExternalLink, Copy, UserPlus } from 'lucide-react'
import { cn } from '@/lib/utils'

const initials = (name: string) =>
  name.split(' ').map((n) => n[0]).join('').toUpperCase().slice(0, 2)

function formatDate(dateString: string | undefined | null): string {
  if (!dateString) return '—'
  return new Date(dateString).toLocaleDateString('en-AU', {
    day: 'numeric',
    month: 'short',
    year: 'numeric',
  })
}

function hostnameFromUrl(url: string): string {
  try {
    return new URL(url).hostname
  } catch {
    return url
  }
}

export function GuestListDetailPage() {
  const { id } = useParams()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const { isAdmin } = useAuth()

  const [editOpen, setEditOpen] = useState(false)
  const [contactSearchOpen, setContactSearchOpen] = useState(false)
  const [shareDialogOpen, setShareDialogOpen] = useState(false)
  const [editingNotesId, setEditingNotesId] = useState<string | null>(null)
  const [editingNotesValue, setEditingNotesValue] = useState('')
  const [contactDrawerOpen, setContactDrawerOpen] = useState(false)
  const [selectedContact, setSelectedContact] = useState<Contact | null>(null)

  // Edit form state
  const [editForm, setEditForm] = useState({
    name: '',
    description: '',
    event_projection: '',
    status: '',
  })

  // ── Queries ──

  const { data: guestList, isLoading: listLoading } = useQuery({
    queryKey: ['guest-list', id],
    queryFn: () => getGuestList(id!),
    enabled: !!id,
  })

  const { data: itemsData } = useQuery({
    queryKey: ['guest-list-items', id],
    queryFn: () => getGuestListItems(id!),
    enabled: !!id,
  })

  const { data: sharesData } = useQuery({
    queryKey: ['guest-list-shares', id],
    queryFn: () => getGuestListShares(id!),
    enabled: !!id,
  })

  const { data: eventsData } = useQuery({
    queryKey: ['event-projections'],
    queryFn: () => getEventProjections(),
    enabled: editOpen,
  })

  const items = itemsData?.items ?? []
  const shares = sharesData?.items ?? []
  const events = eventsData?.items ?? []

  // ── Mutations ──

  const updateListMutation = useMutation({
    mutationFn: (data: Partial<{ name: string; description: string; event_projection: string; status: string }>) =>
      updateGuestList(id!, data),
    onSuccess: () => {
      toast.success('Guest list updated')
      queryClient.invalidateQueries({ queryKey: ['guest-list', id] })
      queryClient.invalidateQueries({ queryKey: ['guest-lists'] })
      setEditOpen(false)
    },
    onError: (error: Error) => toast.error(error.message),
  })

  const deleteListMutation = useMutation({
    mutationFn: () => deleteGuestList(id!),
    onSuccess: () => {
      toast.success('Guest list deleted')
      queryClient.invalidateQueries({ queryKey: ['guest-lists'] })
      navigate('/guest-lists')
    },
    onError: (error: Error) => toast.error(error.message),
  })

  const updateItemMutation = useMutation({
    mutationFn: ({ itemId, data }: { itemId: string; data: Partial<{ invite_round: string; invite_status: string; notes: string }> }) =>
      updateGuestListItem(itemId, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['guest-list-items', id] })
    },
    onError: (error: Error) => toast.error(error.message),
  })

  const deleteItemMutation = useMutation({
    mutationFn: (itemId: string) => deleteGuestListItem(itemId),
    onSuccess: () => {
      toast.success('Guest removed')
      queryClient.invalidateQueries({ queryKey: ['guest-list-items', id] })
      queryClient.invalidateQueries({ queryKey: ['guest-list', id] })
    },
    onError: (error: Error) => toast.error(error.message),
  })

  const revokeShareMutation = useMutation({
    mutationFn: (shareId: string) => revokeGuestListShare(shareId),
    onSuccess: () => {
      toast.success('Share revoked')
      queryClient.invalidateQueries({ queryKey: ['guest-list-shares', id] })
      queryClient.invalidateQueries({ queryKey: ['guest-list', id] })
    },
    onError: (error: Error) => toast.error(error.message),
  })

  // ── Handlers ──

  const handleOpenEdit = () => {
    if (!guestList) return
    setEditForm({
      name: guestList.name || '',
      description: guestList.description || '',
      event_projection: guestList.event_projection || '',
      status: guestList.status || 'draft',
    })
    setEditOpen(true)
  }

  const handleSaveEdit = (e: React.FormEvent) => {
    e.preventDefault()
    updateListMutation.mutate(editForm)
  }

  const handleDeleteList = () => {
    if (confirm('Are you sure you want to delete this guest list? This cannot be undone.')) {
      deleteListMutation.mutate()
    }
  }

  const handleInviteRoundChange = (item: GuestListItem, value: string) => {
    updateItemMutation.mutate({
      itemId: item.id,
      data: { invite_round: value },
    })
  }

  const handleInviteStatusChange = (item: GuestListItem, value: string) => {
    updateItemMutation.mutate({
      itemId: item.id,
      data: { invite_status: value },
    })
  }

  const handleNotesClick = (item: GuestListItem) => {
    setEditingNotesId(item.id)
    setEditingNotesValue(item.notes || '')
  }

  const handleNotesBlur = (item: GuestListItem) => {
    if (editingNotesValue !== (item.notes || '')) {
      updateItemMutation.mutate({
        itemId: item.id,
        data: { notes: editingNotesValue },
      })
    }
    setEditingNotesId(null)
  }

  const handleContactClick = async (item: GuestListItem) => {
    try {
      const contact = await getContact(item.contact_id)
      setSelectedContact(contact)
      setContactDrawerOpen(true)
    } catch {
      toast.error('Failed to load contact')
    }
  }

  const handleCloseContactDrawer = () => {
    setContactDrawerOpen(false)
    setSelectedContact(null)
    queryClient.invalidateQueries({ queryKey: ['guest-list-items', id] })
  }

  const handleRemoveItem = (item: GuestListItem) => {
    deleteItemMutation.mutate(item.id)
  }

  const handleCopyShareUrl = (share: GuestListShare) => {
    const url = `${window.location.origin}/shared/${share.token}`
    navigator.clipboard.writeText(url)
    toast.success('Share URL copied to clipboard')
  }

  const handleRevokeShare = (share: GuestListShare) => {
    if (confirm('Are you sure you want to revoke this share?')) {
      revokeShareMutation.mutate(share.id)
    }
  }

  const getShareStatus = (share: GuestListShare): { label: string; variant: 'default' | 'destructive' | 'secondary' } => {
    if (share.revoked) return { label: 'Revoked', variant: 'destructive' }
    if (share.expires_at && new Date(share.expires_at) < new Date()) return { label: 'Expired', variant: 'secondary' }
    return { label: 'Active', variant: 'default' }
  }

  if (listLoading) {
    return (
      <div className="space-y-4">
        <PageHeader title="Loading..." />
      </div>
    )
  }

  if (!guestList) {
    return (
      <div className="space-y-4">
        <PageHeader title="Guest list not found" />
        <p className="text-muted-foreground">This guest list could not be found.</p>
      </div>
    )
  }

  const existingContactIds = items.map((item) => item.contact_id)

  return (
    <div className="space-y-8">
      {/* Header */}
      <PageHeader title={guestList.name}>
        {isAdmin && (
          <>
            <Button variant="outline" onClick={handleOpenEdit}>
              <Pencil className="w-4 h-4 mr-1" /> Edit
            </Button>
            <Button variant="outline" onClick={() => setShareDialogOpen(true)}>
              <Share2 className="w-4 h-4 mr-1" /> Share
            </Button>
            <Button onClick={() => setContactSearchOpen(true)}>
              <UserPlus className="w-4 h-4 mr-1" /> Add contacts
            </Button>
            <Button variant="outline" onClick={handleDeleteList}>
              <Trash2 className="w-4 h-4" />
            </Button>
          </>
        )}
      </PageHeader>

      {/* Guests section */}
      <div className="space-y-3">
        <div className="flex items-center gap-2">
          <h2 className="text-lg">Guests</h2>
          <Badge variant="secondary">{items.length}</Badge>
        </div>

        {items.length === 0 ? (
          <p className="text-muted-foreground py-8 text-center">
            No guests added yet. Click 'Add contacts' to get started.
          </p>
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Name</TableHead>
                <TableHead>Role</TableHead>
                <TableHead>Company</TableHead>
                <TableHead>Invite round</TableHead>
                <TableHead>Invite status</TableHead>
                <TableHead>LinkedIn</TableHead>
                <TableHead>City</TableHead>
                <TableHead>Connection</TableHead>
                <TableHead>Relationship</TableHead>
                <TableHead>Notes</TableHead>
                <TableHead>Client notes</TableHead>
                <TableHead className="w-[50px]" />
              </TableRow>
            </TableHeader>
            <TableBody>
              {items.map((item) => {
                const isArchived = item.contact_status === 'archived'
                return (
                  <TableRow key={item.id} className={cn(isArchived && 'text-muted-foreground')}>
                    <TableCell>
                      <button
                        type="button"
                        className="flex items-center gap-2 hover:underline cursor-pointer text-left"
                        onClick={() => handleContactClick(item)}
                      >
                        <Avatar className="h-8 w-8">
                          <AvatarImage src={item.contact_avatar_small_url || item.contact_avatar_thumb_url || item.contact_avatar_url} />
                          <AvatarFallback className="text-xs">{initials(item.contact_name)}</AvatarFallback>
                        </Avatar>
                        {item.contact_name}
                        {isArchived && <Badge variant="outline">Archived</Badge>}
                      </button>
                    </TableCell>
                    <TableCell className="text-muted-foreground">
                      {item.contact_job_title || '—'}
                    </TableCell>
                    <TableCell className="text-muted-foreground">
                      {item.contact_organisation_name || '—'}
                    </TableCell>
                    <TableCell>
                      <Select
                        value={item.invite_round || 'none'}
                        onValueChange={(v) => handleInviteRoundChange(item, v === 'none' ? '' : v)}
                        disabled={!isAdmin}
                      >
                        <SelectTrigger className="w-[100px]">
                          <SelectValue placeholder="—" />
                        </SelectTrigger>
                        <SelectContent>
                          <SelectItem value="none">—</SelectItem>
                          <SelectItem value="1st">1st</SelectItem>
                          <SelectItem value="2nd">2nd</SelectItem>
                          <SelectItem value="3rd">3rd</SelectItem>
                          <SelectItem value="maybe">Maybe</SelectItem>
                        </SelectContent>
                      </Select>
                    </TableCell>
                    <TableCell>
                      <Select
                        value={item.invite_status || 'none'}
                        onValueChange={(v) => handleInviteStatusChange(item, v === 'none' ? '' : v)}
                        disabled={!isAdmin}
                      >
                        <SelectTrigger className="w-[120px]">
                          <SelectValue placeholder="—" />
                        </SelectTrigger>
                        <SelectContent>
                          <SelectItem value="none">—</SelectItem>
                          <SelectItem value="invited">Invited</SelectItem>
                          <SelectItem value="accepted">Accepted</SelectItem>
                          <SelectItem value="declined">Declined</SelectItem>
                          <SelectItem value="no_show">No show</SelectItem>
                        </SelectContent>
                      </Select>
                    </TableCell>
                    <TableCell>
                      {item.contact_linkedin ? (
                        <a
                          href={item.contact_linkedin}
                          target="_blank"
                          rel="noopener noreferrer"
                          className="text-primary hover:underline inline-flex items-center gap-1"
                        >
                          {hostnameFromUrl(item.contact_linkedin)}
                          <ExternalLink className="w-3 h-3" />
                        </a>
                      ) : (
                        <span className="text-muted-foreground">—</span>
                      )}
                    </TableCell>
                    <TableCell className="text-muted-foreground">
                      {item.contact_location || '—'}
                    </TableCell>
                    <TableCell>
                      {item.contact_degrees ? (
                        <Badge variant="outline">{item.contact_degrees}</Badge>
                      ) : (
                        <span className="text-muted-foreground">—</span>
                      )}
                    </TableCell>
                    <TableCell>
                      {item.contact_relationship ? (
                        <span className="text-muted-foreground">{item.contact_relationship}/5</span>
                      ) : (
                        <span className="text-muted-foreground">—</span>
                      )}
                    </TableCell>
                    <TableCell>
                      {editingNotesId === item.id ? (
                        <Textarea
                          value={editingNotesValue}
                          onChange={(e) => setEditingNotesValue(e.target.value)}
                          onBlur={() => handleNotesBlur(item)}
                          autoFocus
                          rows={2}
                          className="min-w-[200px]"
                        />
                      ) : (
                        <button
                          type="button"
                          onClick={() => handleNotesClick(item)}
                          className="text-left text-sm cursor-pointer min-w-[100px] min-h-[24px] rounded px-1 -mx-1 hover:bg-muted/50"
                          disabled={!isAdmin}
                        >
                          {item.notes ? (
                            <span className="line-clamp-2">{item.notes}</span>
                          ) : (
                            <span className="text-muted-foreground">—</span>
                          )}
                        </button>
                      )}
                    </TableCell>
                    <TableCell className="text-muted-foreground max-w-xs">
                      {item.client_notes ? (
                        <span className="line-clamp-2">{item.client_notes}</span>
                      ) : (
                        <span>—</span>
                      )}
                    </TableCell>
                    <TableCell>
                      {isAdmin && (
                        <Button
                          variant="ghost"
                          size="icon"
                          onClick={() => handleRemoveItem(item)}
                          disabled={deleteItemMutation.isPending}
                        >
                          <X className="w-4 h-4" />
                        </Button>
                      )}
                    </TableCell>
                  </TableRow>
                )
              })}
            </TableBody>
          </Table>
        )}
      </div>

      {/* Shares section */}
      <div className="space-y-3">
        <div className="flex items-center gap-2">
          <h2 className="text-lg">Shares</h2>
          <Badge variant="secondary">{shares.length}</Badge>
        </div>

        {shares.length === 0 ? (
          <p className="text-muted-foreground py-8 text-center">
            No shares yet. Click 'Share' to share this list.
          </p>
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Recipient</TableHead>
                <TableHead>Created</TableHead>
                <TableHead>Last accessed</TableHead>
                <TableHead>Views</TableHead>
                <TableHead>Expires</TableHead>
                <TableHead>Status</TableHead>
                <TableHead>Actions</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {shares.map((share) => {
                const status = getShareStatus(share)
                const isActive = status.label === 'Active'
                return (
                  <TableRow key={share.id}>
                    <TableCell>
                      <div>
                        <div>{share.recipient_email}</div>
                        {share.recipient_name && (
                          <div className="text-sm text-muted-foreground">{share.recipient_name}</div>
                        )}
                      </div>
                    </TableCell>
                    <TableCell className="text-muted-foreground">
                      {formatDate(share.created)}
                    </TableCell>
                    <TableCell className="text-muted-foreground">
                      {share.last_accessed_at ? formatDate(share.last_accessed_at) : 'Never'}
                    </TableCell>
                    <TableCell className="text-muted-foreground">
                      {share.access_count}
                    </TableCell>
                    <TableCell className="text-muted-foreground">
                      {formatDate(share.expires_at)}
                    </TableCell>
                    <TableCell>
                      <Badge variant={status.variant}>{status.label}</Badge>
                    </TableCell>
                    <TableCell>
                      <div className="flex items-center gap-1">
                        <Button
                          variant="ghost"
                          size="icon"
                          onClick={() => handleCopyShareUrl(share)}
                          title="Copy share URL"
                        >
                          <Copy className="w-4 h-4" />
                        </Button>
                        {isActive && isAdmin && (
                          <Button
                            variant="ghost"
                            size="icon"
                            onClick={() => handleRevokeShare(share)}
                            disabled={revokeShareMutation.isPending}
                            title="Revoke share"
                          >
                            <X className="w-4 h-4" />
                          </Button>
                        )}
                      </div>
                    </TableCell>
                  </TableRow>
                )
              })}
            </TableBody>
          </Table>
        )}
      </div>

      {/* Edit sheet */}
      <Sheet open={editOpen} onOpenChange={(o) => !o && setEditOpen(false)}>
        <SheetContent>
          <SheetHeader>
            <SheetTitle>Edit guest list</SheetTitle>
          </SheetHeader>

          <form onSubmit={handleSaveEdit} className="flex-1 overflow-y-auto p-6 space-y-4">
            <div>
              <label className="block text-sm text-muted-foreground mb-1.5">Name</label>
              <Input
                value={editForm.name}
                onChange={(e) => setEditForm({ ...editForm, name: e.target.value })}
                required
              />
            </div>
            <div>
              <label className="block text-sm text-muted-foreground mb-1.5">Description</label>
              <Textarea
                value={editForm.description}
                onChange={(e) => setEditForm({ ...editForm, description: e.target.value })}
                rows={3}
              />
            </div>
            <div>
              <label className="block text-sm text-muted-foreground mb-1.5">Event</label>
              <Select
                value={editForm.event_projection || 'none'}
                onValueChange={(v) => setEditForm({ ...editForm, event_projection: v === 'none' ? '' : v })}
              >
                <SelectTrigger>
                  <SelectValue placeholder="Select event" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="none">None</SelectItem>
                  {events.map((event) => (
                    <SelectItem key={event.id} value={event.id}>
                      {event.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div>
              <label className="block text-sm text-muted-foreground mb-1.5">Status</label>
              <Select
                value={editForm.status}
                onValueChange={(v) => setEditForm({ ...editForm, status: v })}
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="draft">Draft</SelectItem>
                  <SelectItem value="active">Active</SelectItem>
                  <SelectItem value="archived">Archived</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </form>

          <SheetFooter>
            <Button variant="outline" onClick={() => setEditOpen(false)}>
              Cancel
            </Button>
            <Button onClick={handleSaveEdit} disabled={updateListMutation.isPending}>
              {updateListMutation.isPending ? 'Saving...' : 'Save changes'}
            </Button>
          </SheetFooter>
        </SheetContent>
      </Sheet>

      {/* Contact search drawer */}
      <ContactSearchDrawer
        open={contactSearchOpen}
        onOpenChange={setContactSearchOpen}
        listId={id!}
        existingContactIds={existingContactIds}
      />

      {/* Contact drawer */}
      <ContactDrawer
        open={contactDrawerOpen}
        onClose={handleCloseContactDrawer}
        contact={selectedContact}
      />

      {/* Share dialog */}
      <ShareDialog
        open={shareDialogOpen}
        onOpenChange={setShareDialogOpen}
        listId={id!}
        listName={guestList.name}
        eventName={guestList.event_name}
      />
    </div>
  )
}
