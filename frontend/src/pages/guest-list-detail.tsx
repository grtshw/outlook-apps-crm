import { useState, useMemo } from 'react'
import { useParams, useNavigate } from 'react-router'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { useAuth } from '@/hooks/use-pocketbase'
import {
  getGuestList, updateGuestList, deleteGuestList, cloneGuestList,
  getGuestListItems, updateGuestListItem, deleteGuestListItem,
  getGuestListShares, revokeGuestListShare,
  getEventProjections,
  getContact,
  getContacts,
  getOrganisations,
  toggleGuestListRSVP,
  sendRSVPInvites,
} from '@/lib/api'
import type { Contact, GuestListItem, GuestListShare, ProgramItem } from '@/lib/pocketbase'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Badge } from '@/components/ui/badge'
import { Textarea } from '@/components/ui/textarea'
import {
  Select, SelectContent, SelectItem, SelectTrigger, SelectValue,
} from '@/components/ui/select'
import {
  Sheet, SheetContent, SheetHeader, SheetTitle, SheetFooter, SheetSection,
} from '@/components/ui/sheet'
import {
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
} from '@/components/ui/table'
import { PageHeader } from '@/components/ui/page-header'
import { ContactSearchDrawer } from '@/components/contact-search-dialog'
import { ContactDrawer } from '@/components/contact-drawer'
import { ShareDialog } from '@/components/share-dialog'
import { Avatar, AvatarImage, AvatarFallback } from '@/components/ui/avatar'
import { Switch } from '@/components/ui/switch'
import { RichTextEditor } from '@/components/rich-text-editor'
import { ProgramEditor } from '@/components/program-editor'
import { OrganisationCombobox } from '@/components/organisation-combobox'
import { ContactCombobox } from '@/components/contact-combobox'
import {
  DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger, DropdownMenuSeparator,
} from '@/components/ui/dropdown-menu'
import { Pencil, Share2, Trash2, X, ExternalLink, Copy, UserPlus, ArrowUp, ArrowDown, ArrowUpDown, Columns3, CircleCheck, XCircle, Send, EllipsisVertical } from 'lucide-react'
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
  const [sortKey, setSortKey] = useState<string | null>(null)
  const [sortDir, setSortDir] = useState<'asc' | 'desc'>('asc')
  const [showContactCols, setShowContactCols] = useState(false)
  const [rsvpDetailItem, setRsvpDetailItem] = useState<GuestListItem | null>(null)
  const [rsvpDrawerOpen, setRsvpDrawerOpen] = useState(false)
  const [cloneOpen, setCloneOpen] = useState(false)

  // Edit form state
  const [editForm, setEditForm] = useState({
    name: '',
    description: '',
    event_projection: '',
    status: '',
    event_date: '',
    event_time: '',
    event_location: '',
    event_location_address: '',
    landing_program: [] as ProgramItem[],
    landing_content: '',
    program_description: '',
    organisation: '',
    rsvp_bcc_contacts: [] as string[],
  })

  // Clone form state
  const [cloneForm, setCloneForm] = useState({
    name: '',
    description: '',
    event_projection: '',
    status: 'draft',
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
    enabled: editOpen || cloneOpen,
    staleTime: 5 * 60 * 1000,
  })

  const { data: orgsData } = useQuery({
    queryKey: ['organisations-all'],
    queryFn: () => getOrganisations({ perPage: 200, status: 'active' }),
    enabled: editOpen,
    staleTime: 5 * 60 * 1000,
  })

  const { data: bccContactsData } = useQuery({
    queryKey: ['contacts-bcc'],
    queryFn: () => getContacts({ perPage: 200, status: 'active', sort: 'name' }),
    enabled: editOpen,
    staleTime: 30 * 1000,
  })

  const items = itemsData?.items ?? []
  const shares = sharesData?.items ?? []
  const events = eventsData?.items ?? []
  const organisations = orgsData?.items ?? []

  const handleSort = (key: string) => {
    if (sortKey === key) {
      setSortDir((d) => (d === 'asc' ? 'desc' : 'asc'))
    } else {
      setSortKey(key)
      setSortDir('asc')
    }
  }

  const sortedItems = useMemo(() => {
    if (!sortKey) return items
    return [...items].sort((a, b) => {
      let aVal: string | number = ''
      let bVal: string | number = ''
      switch (sortKey) {
        case 'name': aVal = a.contact_name; bVal = b.contact_name; break
        case 'role': aVal = a.contact_job_title || ''; bVal = b.contact_job_title || ''; break
        case 'company': aVal = a.contact_organisation_name || ''; bVal = b.contact_organisation_name || ''; break
        case 'invite_round': aVal = a.invite_round || ''; bVal = b.invite_round || ''; break
        case 'invite_status': aVal = a.invite_status || ''; bVal = b.invite_status || ''; break
        case 'city': aVal = a.contact_location || ''; bVal = b.contact_location || ''; break
        case 'connection': aVal = a.contact_degrees || ''; bVal = b.contact_degrees || ''; break
        case 'relationship': aVal = a.contact_relationship || 0; bVal = b.contact_relationship || 0; break
      }
      if (typeof aVal === 'number' && typeof bVal === 'number') {
        return sortDir === 'asc' ? aVal - bVal : bVal - aVal
      }
      const cmp = String(aVal).localeCompare(String(bVal))
      return sortDir === 'asc' ? cmp : -cmp
    })
  }, [items, sortKey, sortDir])

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

  const cloneListMutation = useMutation({
    mutationFn: (data: { name: string; description: string; event_projection: string; status: string }) =>
      cloneGuestList(id!, data),
    onSuccess: (data) => {
      setCloneOpen(false)
      toast.success(`Cloned with ${data.items_cloned} guests`)
      queryClient.invalidateQueries({ queryKey: ['guest-lists'] })
      navigate(`/guest-lists/${data.id}`)
    },
    onError: (error: Error) => toast.error(error.message),
  })

  const updateItemMutation = useMutation({
    mutationFn: ({ itemId, data }: { itemId: string; data: Partial<{ invite_round: string; invite_status: string; notes: string }> }) =>
      updateGuestListItem(itemId, data),
    onSuccess: () => {
      toast.success('Updated')
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

  const toggleRSVPMutation = useMutation({
    mutationFn: (enabled: boolean) => toggleGuestListRSVP(id!, enabled),
    onSuccess: (data) => {
      toast.success(data.rsvp_enabled ? 'RSVP enabled' : 'RSVP disabled')
      queryClient.invalidateQueries({ queryKey: ['guest-list', id] })
    },
    onError: (error: Error) => toast.error(error.message),
  })

  const sendInvitesMutation = useMutation({
    mutationFn: (itemIds?: string[]) => sendRSVPInvites(id!, itemIds),
    onSuccess: (data) => {
      toast.success(`Sent ${data.sent} invite${data.sent !== 1 ? 's' : ''}${data.skipped ? `, ${data.skipped} skipped` : ''}`)
      queryClient.invalidateQueries({ queryKey: ['guest-list-items', id] })
    },
    onError: (error: Error) => toast.error(error.message),
  })

  // RSVP summary counts
  const rsvpCounts = useMemo(() => {
    const accepted = items.filter((i) => i.rsvp_status === 'accepted').length
    const declined = items.filter((i) => i.rsvp_status === 'declined').length
    const plusOnes = items.filter((i) => i.rsvp_plus_one && i.rsvp_status === 'accepted').length
    return { accepted, declined, plusOnes }
  }, [items])

  // ── Handlers ──

  const handleOpenEdit = () => {
    if (!guestList) return
    setEditForm({
      name: guestList.name || '',
      description: guestList.description || '',
      event_projection: guestList.event_projection || '',
      status: guestList.status || 'draft',
      event_date: guestList.event_date || '',
      event_time: guestList.event_time || '',
      event_location: guestList.event_location || '',
      event_location_address: guestList.event_location_address || '',
      landing_program: guestList.landing_program || [],
      landing_content: guestList.landing_content || '',
      program_description: guestList.program_description || '',
      organisation: guestList.organisation || '',
      rsvp_bcc_contacts: (guestList.rsvp_bcc_contacts || []).map((c: { id: string }) => c.id),
    })
    setEditOpen(true)
  }

  const handleSaveEdit = (e: React.FormEvent) => {
    e.preventDefault()
    updateListMutation.mutate(editForm)
  }

  const handleOpenClone = () => {
    if (!guestList) return
    setCloneForm({
      name: guestList.name + ' (copy)',
      description: guestList.description || '',
      event_projection: guestList.event_projection || '',
      status: 'draft',
    })
    setCloneOpen(true)
  }

  const handleClone = (e: React.FormEvent) => {
    e.preventDefault()
    cloneListMutation.mutate(cloneForm)
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
    if (!value) return
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
            <Button variant="outline" onClick={() => setShareDialogOpen(true)}>
              <Share2 className="w-4 h-4 mr-1" /> Share
            </Button>
            <Button variant="outline" onClick={() => setRsvpDrawerOpen(true)}>
              <Send className="w-4 h-4 mr-1" /> RSVP
              {rsvpCounts.accepted > 0 && (
                <Badge variant="secondary" className="ml-1">{rsvpCounts.accepted}</Badge>
              )}
            </Button>
            <Button onClick={() => setContactSearchOpen(true)}>
              <UserPlus className="w-4 h-4 mr-1" /> Select guests
            </Button>
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button variant="outline" size="icon">
                  <EllipsisVertical className="w-4 h-4" />
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end">
                <DropdownMenuItem onClick={handleOpenEdit}>
                  <Pencil className="w-4 h-4 mr-2" /> Edit
                </DropdownMenuItem>
                <DropdownMenuItem onClick={handleOpenClone}>
                  <Copy className="w-4 h-4 mr-2" /> Clone
                </DropdownMenuItem>
                <DropdownMenuSeparator />
                <DropdownMenuItem onClick={handleDeleteList} variant="destructive">
                  <Trash2 className="w-4 h-4 mr-2" /> Delete
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          </>
        )}
      </PageHeader>

      {/* Guests section */}
      <div className="space-y-3">
        <div className="flex items-center gap-2">
          <h2 className="text-lg">Guests</h2>
          <Badge variant="secondary">{items.length}</Badge>
          {items.length > 0 && (
            <Button
              variant="ghost"
              size="sm"
              onClick={() => setShowContactCols((v) => !v)}
              className="ml-auto text-muted-foreground"
            >
              <Columns3 className="w-4 h-4 mr-1" />
              {showContactCols ? 'Hide' : 'Show'} contact details
            </Button>
          )}
        </div>

        {items.length === 0 ? (
          <p className="text-muted-foreground py-8 text-center">
            No guests added yet. Click 'Select guests' to get started.
          </p>
        ) : (
          <div className="overflow-x-auto overflow-y-visible border rounded-md">
          <Table>
            <TableHeader className="sticky top-0 z-10 bg-background">
              <TableRow>
                {[
                  { key: 'name', label: 'Name', className: 'max-w-[180px]' },
                  { key: 'role', label: 'Role', className: 'max-w-[140px]' },
                  { key: 'company', label: 'Company', className: 'max-w-[120px]' },
                  { key: null, label: '', className: 'w-[40px]' },
                  { key: 'invite_round', label: 'Invite round' },
                  { key: 'invite_status', label: 'Invite status' },
                  { key: null, label: 'RSVP' },
                  { key: 'city', label: 'City', collapsible: true },
                  { key: 'connection', label: 'Connection', collapsible: true },
                  { key: 'relationship', label: 'Relationship', collapsible: true },
                  { key: null, label: 'Notes', collapsible: true },
                  { key: null, label: 'Client notes', collapsible: true },
                ].filter((col) => !col.collapsible || showContactCols).map((col, idx) => (
                  <TableHead
                    key={col.label || idx}
                    className={cn(col.key ? 'cursor-pointer select-none hover:text-foreground' : '', col.className)}
                    onClick={col.key ? () => handleSort(col.key!) : undefined}
                  >
                    <span className="inline-flex items-center gap-1">
                      {col.label}
                      {col.key && (
                        sortKey === col.key
                          ? sortDir === 'asc' ? <ArrowUp className="w-3 h-3" /> : <ArrowDown className="w-3 h-3" />
                          : <ArrowUpDown className="w-3 h-3 opacity-30" />
                      )}
                    </span>
                  </TableHead>
                ))}
                <TableHead className="w-[50px]" />
              </TableRow>
            </TableHeader>
            <TableBody>
              {sortedItems.map((item) => {
                const isArchived = item.contact_status === 'archived'
                return (
                  <TableRow key={item.id} className={cn(isArchived && 'text-muted-foreground')}>
                    <TableCell className="max-w-[180px]">
                      <button
                        type="button"
                        className="flex items-center gap-2 hover:underline cursor-pointer text-left min-w-0"
                        onClick={() => handleContactClick(item)}
                      >
                        <Avatar className="h-8 w-8 shrink-0">
                          <AvatarImage src={item.contact_avatar_small_url || item.contact_avatar_thumb_url || item.contact_avatar_url} />
                          <AvatarFallback className="text-xs">{initials(item.contact_name)}</AvatarFallback>
                        </Avatar>
                        <span className="truncate">{item.contact_name}</span>
                        {isArchived && <Badge variant="outline">Archived</Badge>}
                      </button>
                    </TableCell>
                    <TableCell className="text-muted-foreground max-w-[140px] truncate">
                      {item.contact_job_title || '—'}
                    </TableCell>
                    <TableCell className="text-muted-foreground max-w-[120px] truncate">
                      {item.contact_organisation_name || '—'}
                    </TableCell>
                    <TableCell>
                      {item.contact_linkedin ? (
                        <a
                          href={item.contact_linkedin}
                          target="_blank"
                          rel="noopener noreferrer"
                          className="text-muted-foreground hover:text-foreground"
                          title={item.contact_linkedin}
                        >
                          <ExternalLink className="w-4 h-4" />
                        </a>
                      ) : null}
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
                          <SelectItem value="to_invite">To invite</SelectItem>
                          <SelectItem value="invited">Invited</SelectItem>
                          <SelectItem value="accepted">Accepted</SelectItem>
                          <SelectItem value="declined">Declined</SelectItem>
                          <SelectItem value="no_show">No show</SelectItem>
                        </SelectContent>
                      </Select>
                    </TableCell>
                    <TableCell>
                      <button
                        type="button"
                        className="cursor-pointer inline-flex items-center gap-1"
                        onClick={() => setRsvpDetailItem(item)}
                      >
                        {item.rsvp_status === 'accepted' ? (
                          <span className="inline-flex items-center gap-1 text-green-600">
                            <CircleCheck className="h-4 w-4" />
                            <span className="text-sm">Accepted</span>
                          </span>
                        ) : item.rsvp_status === 'declined' ? (
                          <span className="inline-flex items-center gap-1 text-muted-foreground">
                            <XCircle className="h-4 w-4" />
                            <span className="text-sm">Declined</span>
                          </span>
                        ) : (
                          <span className="text-muted-foreground">—</span>
                        )}
                      </button>
                    </TableCell>
                    {showContactCols && (
                      <>
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
                      </>
                    )}
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
          </div>
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

      {/* RSVP detail sheet */}
      <Sheet open={!!rsvpDetailItem} onOpenChange={(o) => !o && setRsvpDetailItem(null)}>
        <SheetContent>
          <SheetHeader>
            <SheetTitle>RSVP details</SheetTitle>
          </SheetHeader>
          {rsvpDetailItem && (
            <div className="p-6 space-y-6">
              <div className="space-y-1">
                <p className="text-sm text-muted-foreground">Guest</p>
                <p>{rsvpDetailItem.contact_name}</p>
              </div>

              <div className="space-y-1">
                <p className="text-sm text-muted-foreground">Status</p>
                {rsvpDetailItem.rsvp_status === 'accepted' ? (
                  <span className="inline-flex items-center gap-1 text-green-600">
                    <CircleCheck className="h-4 w-4" />
                    Accepted
                  </span>
                ) : rsvpDetailItem.rsvp_status === 'declined' ? (
                  <span className="inline-flex items-center gap-1 text-muted-foreground">
                    <XCircle className="h-4 w-4" />
                    Declined
                  </span>
                ) : (
                  <span className="text-muted-foreground">No response yet</span>
                )}
              </div>

              {rsvpDetailItem.rsvp_responded_at && (
                <div className="space-y-1">
                  <p className="text-sm text-muted-foreground">Responded</p>
                  <p>{formatDate(rsvpDetailItem.rsvp_responded_at)}</p>
                </div>
              )}

              {rsvpDetailItem.rsvp_comments && (
                <div className="space-y-1">
                  <p className="text-sm text-muted-foreground">Comments</p>
                  <p>{rsvpDetailItem.rsvp_comments}</p>
                </div>
              )}

              {rsvpDetailItem.rsvp_plus_one && (
                <div className="space-y-3">
                  <div className="space-y-1">
                    <p className="text-sm text-muted-foreground">Plus-one</p>
                    <p>{rsvpDetailItem.rsvp_plus_one_name || 'Yes (no name provided)'}</p>
                  </div>
                  {rsvpDetailItem.rsvp_plus_one_dietary && (
                    <div className="space-y-1">
                      <p className="text-sm text-muted-foreground">Plus-one dietary requirements</p>
                      <p>{rsvpDetailItem.rsvp_plus_one_dietary}</p>
                    </div>
                  )}
                </div>
              )}

              {rsvpDetailItem.rsvp_invited_by && (
                <div className="space-y-1">
                  <p className="text-sm text-muted-foreground">Invited by</p>
                  <p>{rsvpDetailItem.rsvp_invited_by}</p>
                </div>
              )}

              {rsvpDetailItem.rsvp_token && (
                <div className="space-y-1">
                  <p className="text-sm text-muted-foreground">Personal RSVP link</p>
                  <div className="flex items-center gap-2">
                    <span className="text-sm text-muted-foreground truncate">
                      {window.location.origin}/rsvp/{rsvpDetailItem.rsvp_token}
                    </span>
                    <Button
                      variant="ghost"
                      size="icon"
                      onClick={() => {
                        navigator.clipboard.writeText(`${window.location.origin}/rsvp/${rsvpDetailItem.rsvp_token}`)
                        toast.success('RSVP link copied')
                      }}
                    >
                      <Copy className="w-4 h-4" />
                    </Button>
                  </div>
                </div>
              )}
            </div>
          )}
        </SheetContent>
      </Sheet>

      {/* RSVP drawer */}
      <Sheet open={rsvpDrawerOpen} onOpenChange={(o) => !o && setRsvpDrawerOpen(false)}>
        <SheetContent>
          <SheetHeader>
            <SheetTitle>RSVP</SheetTitle>
          </SheetHeader>
          <div className="flex-1 overflow-y-auto p-6 space-y-6">
            <SheetSection title="RSVP links">
              <div className="flex items-center justify-between">
                <div className="space-y-0.5">
                  <span className="text-sm">{guestList.rsvp_enabled ? 'Links are active' : 'Links are paused'}</span>
                </div>
                <Switch
                  checked={guestList.rsvp_enabled}
                  onCheckedChange={(checked: boolean) => toggleRSVPMutation.mutate(checked)}
                />
              </div>

              {(rsvpCounts.accepted > 0 || rsvpCounts.declined > 0) && (
                <p className="text-sm text-muted-foreground">
                  {rsvpCounts.accepted} accepted{rsvpCounts.plusOnes > 0 && ` (+${rsvpCounts.plusOnes} plus-ones)`}
                  {rsvpCounts.declined > 0 && `, ${rsvpCounts.declined} declined`}
                </p>
              )}

              {guestList.rsvp_generic_url && (
                <div className="flex items-center gap-2 bg-muted/50 rounded-md px-3 py-2">
                  <span className="text-sm text-muted-foreground truncate flex-1">{guestList.rsvp_generic_url}</span>
                  <Button
                    variant="ghost"
                    size="icon"
                    onClick={() => {
                      navigator.clipboard.writeText(guestList.rsvp_generic_url!)
                      toast.success('Generic RSVP link copied')
                    }}
                  >
                    <Copy className="w-4 h-4" />
                  </Button>
                </div>
              )}
            </SheetSection>

            {guestList.rsvp_enabled && (
              <SheetSection title="Personal invites">
                {items.length === 0 ? (
                  <p className="text-sm text-muted-foreground py-4 text-center">
                    No guests on this list yet.
                  </p>
                ) : (
                  <div className="divide-y divide-border">
                    {items.map((item) => {
                      const hasEmail = !!item.contact_email
                      const wasSent = item.invite_status === 'invited' || item.rsvp_status !== ''
                      return (
                        <div key={item.id} className="flex items-center gap-3 py-2.5 px-2">
                          <span className="text-sm truncate w-[140px] shrink-0">{item.contact_name}</span>
                          <span className="text-sm text-muted-foreground truncate w-[100px] shrink-0">{item.contact_organisation_name || '—'}</span>
                          <span className="text-sm text-muted-foreground truncate min-w-0 flex-1">{item.contact_email || 'No email'}</span>
                          {item.rsvp_status === 'accepted' ? (
                            <span className="shrink-0 text-xs text-green-600 bg-green-50 px-2.5 py-1 rounded-full">Accepted</span>
                          ) : item.rsvp_status === 'declined' ? (
                            <span className="shrink-0 text-xs text-red-600 bg-red-50 px-2.5 py-1 rounded-full">Declined</span>
                          ) : (
                            <Button
                              variant="outline"
                              size="sm"
                              className="shrink-0 cursor-pointer"
                              disabled={!hasEmail || sendInvitesMutation.isPending}
                              onClick={() => sendInvitesMutation.mutate([item.id])}
                            >
                              <Send className="w-3.5 h-3.5 mr-1" />
                              {wasSent ? 'Resend' : 'Send'}
                            </Button>
                          )}
                        </div>
                      )
                    })}
                  </div>
                )}
              </SheetSection>
            )}
          </div>

        </SheetContent>
      </Sheet>

      {/* Edit sheet */}
      <Sheet open={editOpen} onOpenChange={(o) => !o && setEditOpen(false)}>
        <SheetContent>
          <SheetHeader>
            <SheetTitle>Edit guest list</SheetTitle>
          </SheetHeader>

          <div className="flex-1 overflow-y-auto p-6 space-y-6">
            <SheetSection title="Details">
              <div className="space-y-4">
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
                <div>
                  <label className="block text-sm text-muted-foreground mb-1.5">Organisation</label>
                  <OrganisationCombobox
                    value={editForm.organisation}
                    organisations={organisations}
                    onChange={(orgId) => setEditForm({ ...editForm, organisation: orgId })}
                  />
                </div>
              </div>
            </SheetSection>

            <SheetSection title="Event details">
              <div className="space-y-4">
                <div>
                  <label className="block text-sm text-muted-foreground mb-1.5">Date</label>
                  <Input
                    value={editForm.event_date}
                    onChange={(e) => setEditForm({ ...editForm, event_date: e.target.value })}
                    placeholder="e.g. Thursday 20 March 2026"
                  />
                </div>
                <div>
                  <label className="block text-sm text-muted-foreground mb-1.5">Time</label>
                  <Input
                    value={editForm.event_time}
                    onChange={(e) => setEditForm({ ...editForm, event_time: e.target.value })}
                    placeholder="e.g. 5:30PM – 9:00PM"
                  />
                </div>
                <div>
                  <label className="block text-sm text-muted-foreground mb-1.5">Location name</label>
                  <Input
                    value={editForm.event_location}
                    onChange={(e) => setEditForm({ ...editForm, event_location: e.target.value })}
                    placeholder="e.g. The Establishment"
                  />
                </div>
                <div>
                  <label className="block text-sm text-muted-foreground mb-1.5">Location address</label>
                  <Input
                    value={editForm.event_location_address}
                    onChange={(e) => setEditForm({ ...editForm, event_location_address: e.target.value })}
                    placeholder="e.g. 252 George St, Sydney NSW 2000"
                  />
                </div>
              </div>
            </SheetSection>

            <SheetSection title="RSVP content">
              <div className="space-y-4">
                <div>
                  <label className="block text-sm text-muted-foreground mb-1.5">Program description</label>
                  <Textarea
                    value={editForm.program_description}
                    onChange={(e) => setEditForm({ ...editForm, program_description: e.target.value })}
                    placeholder="Intro text shown above the program..."
                    rows={3}
                  />
                </div>
                <div>
                  <label className="block text-sm text-muted-foreground mb-1.5">Program</label>
                  <ProgramEditor
                    items={editForm.landing_program}
                    onChange={(items) => setEditForm({ ...editForm, landing_program: items })}
                  />
                </div>
                <div>
                  <label className="block text-sm text-muted-foreground mb-1.5">Additional content</label>
                  <RichTextEditor
                    content={editForm.landing_content}
                    onChange={(html) => setEditForm({ ...editForm, landing_content: html })}
                    placeholder="Additional content below the program..."
                  />
                </div>
              </div>
            </SheetSection>

            <SheetSection title="Confirmation emails">
              <div className="space-y-4">
                <div>
                  <label className="block text-sm text-muted-foreground mb-1.5">BCC contacts</label>
                  <ContactCombobox
                    value={editForm.rsvp_bcc_contacts}
                    contacts={bccContactsData?.items ?? []}
                    onChange={(ids) => setEditForm({ ...editForm, rsvp_bcc_contacts: ids })}
                  />
                  <p className="text-xs text-muted-foreground mt-1.5">These contacts will be BCC'd when someone accepts an RSVP.</p>
                </div>
              </div>
            </SheetSection>
          </div>

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

      {/* Clone sheet */}
      <Sheet open={cloneOpen} onOpenChange={(o) => !o && setCloneOpen(false)}>
        <SheetContent>
          <SheetHeader>
            <SheetTitle>Clone guest list</SheetTitle>
          </SheetHeader>

          <form onSubmit={handleClone} className="flex-1 overflow-y-auto p-6 space-y-4">
            <div>
              <label className="block text-sm text-muted-foreground mb-1.5">Name</label>
              <Input
                value={cloneForm.name}
                onChange={(e) => setCloneForm({ ...cloneForm, name: e.target.value })}
                required
              />
            </div>
            <div>
              <label className="block text-sm text-muted-foreground mb-1.5">Description</label>
              <Textarea
                value={cloneForm.description}
                onChange={(e) => setCloneForm({ ...cloneForm, description: e.target.value })}
                rows={3}
              />
            </div>
            <div>
              <label className="block text-sm text-muted-foreground mb-1.5">Event</label>
              <Select
                value={cloneForm.event_projection || 'none'}
                onValueChange={(v) => setCloneForm({ ...cloneForm, event_projection: v === 'none' ? '' : v })}
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
                value={cloneForm.status}
                onValueChange={(v) => setCloneForm({ ...cloneForm, status: v })}
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
            <Button variant="outline" onClick={() => setCloneOpen(false)}>
              Cancel
            </Button>
            <Button onClick={handleClone} disabled={cloneListMutation.isPending}>
              {cloneListMutation.isPending ? 'Cloning...' : 'Clone'}
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
