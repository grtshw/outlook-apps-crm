import { useEffect, useRef, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import type { Contact, ContactLink, Activity, DietaryRequirement, AccessibilityRequirement } from '@/lib/pocketbase'
import { createContact, updateContact, deleteContact, getContactActivities, getOrganisations, createOrganisation, getContacts, createContactLink, deleteContactLink } from '@/lib/api'
import { useAuth } from '@/hooks/use-pocketbase'
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetFooter,
  SheetTitle,
  SheetSection,
  useSheetClose,
} from '@/components/ui/sheet'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Checkbox } from '@/components/ui/checkbox'
import { RichTextEditor } from '@/components/rich-text-editor'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Badge } from '@/components/ui/badge'
import { OrganisationCombobox } from '@/components/organisation-combobox'
import { Avatar, AvatarFallback, AvatarImage } from '@/components/ui/avatar'
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from '@/components/ui/collapsible'
import { Trash2, ExternalLink, ChevronDown, Presentation, Trophy, Calendar, Image, Mail, Clock, Star, Plus, Link2, X, Search, Loader2, Ticket } from 'lucide-react'
import { cn } from '@/lib/utils'

const DIETARY_OPTIONS: { value: DietaryRequirement; label: string }[] = [
  { value: 'vegetarian', label: 'Vegetarian' },
  { value: 'vegan', label: 'Vegan' },
  { value: 'gluten_free', label: 'Gluten free' },
  { value: 'dairy_free', label: 'Dairy free' },
  { value: 'nut_allergy', label: 'Nut allergy' },
  { value: 'halal', label: 'Halal' },
  { value: 'kosher', label: 'Kosher' },
]

const ACCESSIBILITY_OPTIONS: { value: AccessibilityRequirement; label: string }[] = [
  { value: 'wheelchair_access', label: 'Wheelchair access' },
  { value: 'hearing_assistance', label: 'Hearing assistance' },
  { value: 'vision_assistance', label: 'Vision assistance' },
  { value: 'sign_language_interpreter', label: 'Sign language interpreter' },
  { value: 'mobility_assistance', label: 'Mobility assistance' },
]

interface ContactDrawerProps {
  open: boolean
  onClose: () => void
  contact: Contact | null
}

interface CollapsibleSectionProps {
  title: string
  children: React.ReactNode
  defaultOpen?: boolean
  badge?: number
}

function CollapsibleSection({ title, children, defaultOpen = true, badge }: CollapsibleSectionProps) {
  const [isOpen, setIsOpen] = useState(defaultOpen)

  return (
    <Collapsible open={isOpen} onOpenChange={setIsOpen}>
      <div className="border border-border rounded-lg">
        <CollapsibleTrigger className="flex w-full items-center justify-between p-4 hover:bg-muted/50 transition-colors">
          <div className="flex items-center gap-2">
            <span className="text-sm">{title}</span>
            {badge !== undefined && badge > 0 && (
              <span className="text-xs bg-muted px-2 py-0.5 rounded-full">{badge}</span>
            )}
          </div>
          <ChevronDown className={cn("h-4 w-4 transition-transform", isOpen && "rotate-180")} />
        </CollapsibleTrigger>
        <CollapsibleContent>
          <div className="p-4 pt-0 space-y-4">
            {children}
          </div>
        </CollapsibleContent>
      </div>
    </Collapsible>
  )
}

function FieldLabel({ htmlFor, children }: { htmlFor?: string; children: React.ReactNode }) {
  return (
    <label htmlFor={htmlFor} className="block text-sm text-muted-foreground mb-1.5">
      {children}
    </label>
  )
}

function StarRating({ value, onChange, disabled }: { value: number; onChange: (v: number) => void; disabled?: boolean }) {
  return (
    <div className="flex gap-1">
      {[1, 2, 3, 4, 5].map((star) => (
        <button
          key={star}
          type="button"
          onClick={() => !disabled && onChange(value === star ? 0 : star)}
          disabled={disabled}
          className="p-0.5 disabled:cursor-not-allowed"
        >
          <Star
            className={cn(
              'h-5 w-5 transition-colors',
              star <= value
                ? 'fill-foreground text-foreground'
                : 'text-muted-foreground/30',
            )}
          />
        </button>
      ))}
    </div>
  )
}

function getActivityIcon(sourceApp: string) {
  switch (sourceApp) {
    case 'presentations':
      return Presentation
    case 'awards':
      return Trophy
    case 'events':
      return Calendar
    case 'dam':
      return Image
    case 'hubspot':
      return Mail
    case 'humanitix':
      return Ticket
    default:
      return Clock
  }
}

function getAvatarSrc(contact: Contact | null) {
  if (!contact) return undefined
  return contact.avatar_thumb_url || contact.avatar_small_url || contact.avatar_url || undefined
}

function LinkedContactsSection({ contact, isAdmin }: { contact: Contact; isAdmin: boolean }) {
  const queryClient = useQueryClient()
  const [showSearch, setShowSearch] = useState(false)
  const [linkSearch, setLinkSearch] = useState('')

  const linkedContacts = contact.linked_contacts ?? []

  const { data: searchResults, isFetching: isSearching } = useQuery({
    queryKey: ['contacts', 'link-search', linkSearch],
    queryFn: () => getContacts({ search: linkSearch, perPage: 5 }),
    enabled: showSearch && linkSearch.length >= 2,
  })

  const linkMutation = useMutation({
    mutationFn: (targetId: string) => createContactLink(contact.id, targetId),
    onSuccess: () => {
      toast.success('Contact linked')
      queryClient.invalidateQueries({ queryKey: ['contact', contact.id] })
      setShowSearch(false)
      setLinkSearch('')
    },
    onError: (error: Error) => toast.error(error.message),
  })

  const unlinkMutation = useMutation({
    mutationFn: (linkId: string) => deleteContactLink(linkId),
    onSuccess: () => {
      toast.success('Link removed')
      queryClient.invalidateQueries({ queryKey: ['contact', contact.id] })
    },
    onError: (error: Error) => toast.error(error.message),
  })

  // Filter out already-linked contacts and self from search results
  const linkedIds = new Set(linkedContacts.map((l: ContactLink) => l.contact_id))
  linkedIds.add(contact.id)
  const filteredResults = (searchResults?.items ?? []).filter((c) => !linkedIds.has(c.id))

  return (
    <CollapsibleSection title="Linked contacts" defaultOpen={linkedContacts.length > 0} badge={linkedContacts.length}>
      {linkedContacts.length === 0 && !showSearch && (
        <p className="text-sm text-muted-foreground">No linked contacts.</p>
      )}

      {linkedContacts.map((link: ContactLink) => (
        <div key={link.link_id} className="flex items-center gap-3">
          <Avatar className="h-8 w-8">
            <AvatarImage src={link.avatar_thumb_url || undefined} />
            <AvatarFallback className="text-xs">
              {link.name.split(' ').map((n: string) => n[0]).join('').slice(0, 2).toUpperCase()}
            </AvatarFallback>
          </Avatar>
          <div className="flex-1 min-w-0">
            <p className="text-sm truncate">{link.name}</p>
            <p className="text-xs text-muted-foreground truncate">
              {[link.email, link.organisation].filter(Boolean).join(' · ')}
            </p>
          </div>
          <Badge variant="outline" className="text-xs shrink-0">{link.source}</Badge>
          {isAdmin && (
            <button
              type="button"
              onClick={() => {
                if (confirm('Remove this link?')) unlinkMutation.mutate(link.link_id)
              }}
              className="p-1 text-muted-foreground hover:text-destructive"
              disabled={unlinkMutation.isPending}
            >
              <X className="w-3.5 h-3.5" />
            </button>
          )}
        </div>
      ))}

      {isAdmin && !showSearch && (
        <button
          type="button"
          onClick={() => setShowSearch(true)}
          className="flex items-center gap-1.5 text-xs text-muted-foreground hover:text-foreground"
        >
          <Link2 className="w-3.5 h-3.5" /> Link contact
        </button>
      )}

      {showSearch && (
        <div className="space-y-2">
          <div className="relative">
            <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground" />
            <Input
              placeholder="Search contacts to link..."
              className="pl-9"
              value={linkSearch}
              onChange={(e) => setLinkSearch(e.target.value)}
              autoFocus
              onKeyDown={(e) => {
                if (e.key === 'Escape') { setShowSearch(false); setLinkSearch('') }
              }}
            />
          </div>
          {isSearching && (
            <div className="flex justify-center py-2">
              <Loader2 className="w-4 h-4 animate-spin text-muted-foreground" />
            </div>
          )}
          {linkSearch.length >= 2 && !isSearching && filteredResults.length === 0 && (
            <p className="text-xs text-muted-foreground py-1">No matching contacts</p>
          )}
          {filteredResults.map((c) => (
            <button
              key={c.id}
              type="button"
              onClick={() => linkMutation.mutate(c.id)}
              disabled={linkMutation.isPending}
              className="w-full flex items-center gap-3 p-2 rounded-lg hover:bg-muted/50 text-left"
            >
              <Avatar className="h-7 w-7">
                <AvatarImage src={c.avatar_thumb_url || c.avatar_small_url || undefined} />
                <AvatarFallback className="text-xs">
                  {(c.first_name?.[0] || '') + (c.last_name?.[0] || '')}
                </AvatarFallback>
              </Avatar>
              <div className="flex-1 min-w-0">
                <p className="text-sm truncate">{c.name}</p>
                <p className="text-xs text-muted-foreground truncate">{c.email}</p>
              </div>
            </button>
          ))}
          <button
            type="button"
            onClick={() => { setShowSearch(false); setLinkSearch('') }}
            className="text-xs text-muted-foreground hover:text-foreground"
          >
            Cancel
          </button>
        </div>
      )}
    </CollapsibleSection>
  )
}

function ContactDrawerFooter({
  isNew,
  onDelete,
  onSubmit,
  isDeleting,
  isSaving,
}: {
  isNew: boolean
  onDelete: () => void
  onSubmit: (e: React.FormEvent) => void
  isDeleting: boolean
  isSaving: boolean
}) {
  const requestClose = useSheetClose()
  return (
    <SheetFooter>
      {!isNew && (
        <Button
          type="button"
          variant="destructive"
          size="icon"
          onClick={onDelete}
          disabled={isDeleting}
        >
          <Trash2 className="h-4 w-4" />
        </Button>
      )}
      <div className="flex-1" />
      <Button type="button" variant="outline" onClick={requestClose}>
        Cancel
      </Button>
      <Button onClick={onSubmit} disabled={isSaving}>
        {isSaving ? 'Saving...' : isNew ? 'Create' : 'Save changes'}
      </Button>
    </SheetFooter>
  )
}

export function ContactDrawer({ open, onClose, contact }: ContactDrawerProps) {
  const { isAdmin } = useAuth()
  const queryClient = useQueryClient()
  const isNew = !contact?.id

  const [formData, setFormData] = useState({
    first_name: '',
    last_name: '',
    email: '',
    personal_email: '',
    phone: '',
    pronouns: '',
    job_title: '',
    bio: '',
    linkedin: '',
    instagram: '',
    website: '',
    location: '',
    status: 'active' as Contact['status'],
    organisation: '',
    degrees: '' as Contact['degrees'] | '',
    relationship: 0,
    notes: '',
    dietary_requirements: [] as DietaryRequirement[],
    dietary_requirements_other: '',
    accessibility_requirements: [] as AccessibilityRequirement[],
    accessibility_requirements_other: '',
  })
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({})
  const initialFormData = useRef(formData)

  const isDirty = JSON.stringify(formData) !== JSON.stringify(initialFormData.current)

  // Load organisations for picker
  const { data: orgsData } = useQuery({
    queryKey: ['organisations-picker'],
    queryFn: () => getOrganisations({ perPage: 500, sort: 'name' }),
    enabled: open,
  })

  // Load activities for existing contacts
  const { data: activities = [] } = useQuery({
    queryKey: ['contact-activities', contact?.id],
    queryFn: () => getContactActivities(contact!.id),
    enabled: !!contact?.id && open,
  })

  useEffect(() => {
    let data: typeof formData
    if (contact) {
      data = {
        first_name: contact.first_name || '',
        last_name: contact.last_name || '',
        email: contact.email || '',
        personal_email: contact.personal_email || '',
        phone: contact.phone || '',
        pronouns: contact.pronouns || '',
        job_title: contact.job_title || '',
        bio: contact.bio || '',
        linkedin: contact.linkedin || '',
        instagram: contact.instagram || '',
        website: contact.website || '',
        location: contact.location || '',
        status: contact.status || 'active',
        organisation: contact.organisation || '',
        degrees: contact.degrees || '',
        relationship: contact.relationship || 0,
        notes: contact.notes || '',
        dietary_requirements: contact.dietary_requirements || [],
        dietary_requirements_other: contact.dietary_requirements_other || '',
        accessibility_requirements: contact.accessibility_requirements || [],
        accessibility_requirements_other: contact.accessibility_requirements_other || '',
      }
    } else {
      data = {
        first_name: '',
        last_name: '',
        email: '',
        personal_email: '',
        phone: '',
        pronouns: '',
        job_title: '',
        bio: '',
        linkedin: '',
        instagram: '',
        website: '',
        location: '',
        status: 'active',
        organisation: '',
        degrees: '',
        relationship: 0,
        notes: '',
        dietary_requirements: [],
        dietary_requirements_other: '',
        accessibility_requirements: [],
        accessibility_requirements_other: '',
      }
    }
    setFormData(data)
    initialFormData.current = data
    setFieldErrors({})
  }, [contact])

  const saveMutation = useMutation({
    mutationFn: async (data: Partial<Contact>) => {
      if (contact?.id) {
        return updateContact(contact.id, data)
      }
      return createContact(data)
    },
    onSuccess: () => {
      toast.success(isNew ? 'Contact created' : 'Contact updated')
      setFieldErrors({})
      queryClient.invalidateQueries({ queryKey: ['contacts'] })
      queryClient.invalidateQueries({ queryKey: ['dashboard-stats'] })
      onClose()
    },
    onError: (error: Error) => {
      toast.error(error.message)
    },
  })

  const deleteMutation = useMutation({
    mutationFn: () => deleteContact(contact!.id),
    onSuccess: () => {
      toast.success('Contact deleted')
      queryClient.invalidateQueries({ queryKey: ['contacts'] })
      queryClient.invalidateQueries({ queryKey: ['dashboard-stats'] })
      onClose()
    },
    onError: (error: Error) => {
      toast.error(error.message)
    },
  })

  const [quickOrgName, setQuickOrgName] = useState('')
  const [showQuickOrg, setShowQuickOrg] = useState(false)

  const createOrgMutation = useMutation({
    mutationFn: (name: string) => createOrganisation({ name, status: 'active' }),
    onSuccess: (org) => {
      toast.success('Organisation created')
      queryClient.invalidateQueries({ queryKey: ['organisations-picker'] })
      setFormData((prev) => ({ ...prev, organisation: org.id }))
      setQuickOrgName('')
      setShowQuickOrg(false)
    },
    onError: (error: Error) => toast.error(error.message),
  })

  const handleQuickCreateOrg = () => {
    const name = quickOrgName.trim()
    if (!name) return
    createOrgMutation.mutate(name)
  }

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    const errors: Record<string, string> = {}
    if (!formData.first_name.trim()) errors.first_name = 'First name is required'
    if (!formData.email.trim()) {
      errors.email = 'Email is required'
    } else if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(formData.email)) {
      errors.email = 'Must be a valid email address'
    }
    if (formData.personal_email.trim() && !/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(formData.personal_email)) {
      errors.personal_email = 'Must be a valid email address'
    }
    if (Object.keys(errors).length > 0) {
      setFieldErrors(errors)
      return
    }
    setFieldErrors({})
    const { degrees, ...rest } = formData
    saveMutation.mutate({
      ...rest,
      ...(degrees ? { degrees: degrees as Contact['degrees'] } : {}),
    })
  }

  const handleDelete = () => {
    if (confirm('Are you sure you want to delete this contact?')) {
      deleteMutation.mutate()
    }
  }

  const initials = [formData.first_name, formData.last_name]
    .filter(Boolean)
    .map((n) => n[0])
    .join('')
    .toUpperCase()
    .slice(0, 2)

  const fullName = [formData.first_name, formData.last_name].filter(Boolean).join(' ')

  const hasBio = !!formData.bio
  const hasSocial = !!(formData.linkedin || formData.instagram || formData.website)
  const hasRequirements = !!(
    formData.dietary_requirements.length > 0 ||
    formData.dietary_requirements_other ||
    formData.accessibility_requirements.length > 0 ||
    formData.accessibility_requirements_other
  )

  return (
    <Sheet open={open} onOpenChange={(o) => !o && onClose()} isDirty={isDirty}>
      <SheetContent>
        <SheetHeader>
          <SheetTitle>{isNew ? 'Add contact' : 'Edit contact'}</SheetTitle>
        </SheetHeader>

        <form onSubmit={handleSubmit} className="flex-1 overflow-y-auto p-6 space-y-4">
          {/* Header with avatar and status for existing contacts */}
          {!isNew && (
            <div className="flex items-center gap-4 pb-2">
              <Avatar className="h-16 w-16">
                <AvatarImage src={getAvatarSrc(contact)} />
                <AvatarFallback className="text-lg">{initials}</AvatarFallback>
              </Avatar>
              <div className="flex-1">
                <p className="text-lg">{fullName || contact?.name}</p>
                {contact?.organisation_name && (
                  <p className="text-sm text-muted-foreground">
                    {contact.organisation_name}
                  </p>
                )}
              </div>
              <Select
                value={formData.status}
                onValueChange={(v) => setFormData({ ...formData, status: v as Contact['status'] })}
                disabled={!isAdmin}
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="active">Active</SelectItem>
                  <SelectItem value="inactive">Inactive</SelectItem>
                  <SelectItem value="pending">Pending</SelectItem>
                  <SelectItem value="archived">Archived</SelectItem>
                </SelectContent>
              </Select>
            </div>
          )}

          {/* Details section */}
          <SheetSection title="Details">
            <div className="grid grid-cols-2 gap-4">
              <div>
                <FieldLabel htmlFor="first_name">First name *</FieldLabel>
                <Input
                  id="first_name"
                  value={formData.first_name}
                  onChange={(e) => {
                    setFormData({ ...formData, first_name: e.target.value })
                    if (fieldErrors.first_name) setFieldErrors((prev) => { const { first_name: _, ...rest } = prev; return rest })
                  }}
                  disabled={!isAdmin}
                  className={fieldErrors.first_name ? 'border-destructive' : ''}
                />
                {fieldErrors.first_name && <p className="text-sm text-destructive mt-1">{fieldErrors.first_name}</p>}
              </div>
              <div>
                <FieldLabel htmlFor="last_name">Last name</FieldLabel>
                <Input
                  id="last_name"
                  value={formData.last_name}
                  onChange={(e) => setFormData({ ...formData, last_name: e.target.value })}
                  disabled={!isAdmin}
                />
              </div>
            </div>

            <div className="grid grid-cols-2 gap-4">
              <div>
                <FieldLabel htmlFor="email">Email *</FieldLabel>
                <Input
                  id="email"
                  type="email"
                  value={formData.email}
                  onChange={(e) => {
                    setFormData({ ...formData, email: e.target.value })
                    if (fieldErrors.email) setFieldErrors((prev) => { const { email: _, ...rest } = prev; return rest })
                  }}
                  disabled={!isAdmin}
                  className={fieldErrors.email ? 'border-destructive' : ''}
                />
                {fieldErrors.email && <p className="text-sm text-destructive mt-1">{fieldErrors.email}</p>}
              </div>
              <div>
                <FieldLabel htmlFor="personal_email">Personal email</FieldLabel>
                <Input
                  id="personal_email"
                  type="email"
                  value={formData.personal_email}
                  onChange={(e) => {
                    setFormData({ ...formData, personal_email: e.target.value })
                    if (fieldErrors.personal_email) setFieldErrors((prev) => { const { personal_email: _, ...rest } = prev; return rest })
                  }}
                  disabled={!isAdmin}
                  className={fieldErrors.personal_email ? 'border-destructive' : ''}
                />
                {fieldErrors.personal_email && <p className="text-sm text-destructive mt-1">{fieldErrors.personal_email}</p>}
              </div>
            </div>

            <div className="grid grid-cols-2 gap-4">
              <div>
                <FieldLabel htmlFor="phone">Phone</FieldLabel>
                <Input
                  id="phone"
                  value={formData.phone}
                  onChange={(e) => setFormData({ ...formData, phone: e.target.value })}
                  disabled={!isAdmin}
                  placeholder="+61 400 000 000"
                />
              </div>
              <div>
                <FieldLabel htmlFor="pronouns">Pronouns</FieldLabel>
                <Input
                  id="pronouns"
                  value={formData.pronouns}
                  onChange={(e) => setFormData({ ...formData, pronouns: e.target.value })}
                  disabled={!isAdmin}
                  placeholder="e.g., she/her"
                />
              </div>
            </div>

            <div className="grid grid-cols-2 gap-4">
              <div>
                <FieldLabel htmlFor="job_title">Job title</FieldLabel>
                <Input
                  id="job_title"
                  value={formData.job_title}
                  onChange={(e) => setFormData({ ...formData, job_title: e.target.value })}
                  disabled={!isAdmin}
                  placeholder="e.g., Senior developer"
                />
              </div>
              <div>
                <div className="flex items-center justify-between mb-1.5">
                  <FieldLabel htmlFor="organisation">Organisation</FieldLabel>
                  {isAdmin && !showQuickOrg && (
                    <button
                      type="button"
                      onClick={() => setShowQuickOrg(true)}
                      className="text-xs text-muted-foreground hover:text-foreground inline-flex items-center gap-0.5"
                    >
                      <Plus className="w-3 h-3" /> New
                    </button>
                  )}
                </div>
                {showQuickOrg ? (
                  <div className="flex gap-2">
                    <Input
                      value={quickOrgName}
                      onChange={(e) => setQuickOrgName(e.target.value)}
                      placeholder="Organisation name"
                      autoFocus
                      onKeyDown={(e) => {
                        if (e.key === 'Enter') { e.preventDefault(); handleQuickCreateOrg() }
                        if (e.key === 'Escape') { setShowQuickOrg(false); setQuickOrgName('') }
                      }}
                    />
                    <Button
                      type="button"
                      size="sm"
                      onClick={handleQuickCreateOrg}
                      disabled={!quickOrgName.trim() || createOrgMutation.isPending}
                    >
                      {createOrgMutation.isPending ? '...' : 'Add'}
                    </Button>
                    <Button
                      type="button"
                      size="sm"
                      variant="ghost"
                      onClick={() => { setShowQuickOrg(false); setQuickOrgName('') }}
                    >
                      Cancel
                    </Button>
                  </div>
                ) : (
                  <OrganisationCombobox
                    value={formData.organisation}
                    organisations={orgsData?.items ?? []}
                    onChange={(orgId) => setFormData({ ...formData, organisation: orgId })}
                    disabled={!isAdmin}
                  />
                )}
              </div>
            </div>

            <div>
              <FieldLabel htmlFor="location">Location</FieldLabel>
              <Input
                id="location"
                value={formData.location}
                onChange={(e) => setFormData({ ...formData, location: e.target.value })}
                disabled={!isAdmin}
                placeholder="e.g., Melbourne, Australia"
              />
            </div>

            {/* Status for new contacts only (existing contacts show it in header) */}
            {isNew && (
              <div>
                <FieldLabel htmlFor="status">Status</FieldLabel>
                <Select
                  value={formData.status}
                  onValueChange={(v) => setFormData({ ...formData, status: v as Contact['status'] })}
                  disabled={!isAdmin}
                >
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="active">Active</SelectItem>
                    <SelectItem value="inactive">Inactive</SelectItem>
                    <SelectItem value="pending">Pending</SelectItem>
                    <SelectItem value="archived">Archived</SelectItem>
                  </SelectContent>
                </Select>
              </div>
            )}
          </SheetSection>

          {/* Relationship section */}
          <CollapsibleSection title="Relationship" defaultOpen={true}>
            <div className="grid grid-cols-2 gap-4">
              <div>
                <FieldLabel htmlFor="degrees">Connection</FieldLabel>
                <Select
                  value={formData.degrees || 'none'}
                  onValueChange={(v) => setFormData({ ...formData, degrees: v === 'none' ? '' : v as Contact['degrees'] })}
                  disabled={!isAdmin}
                >
                  <SelectTrigger>
                    <SelectValue placeholder="Select" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="none">None</SelectItem>
                    <SelectItem value="1st">1st</SelectItem>
                    <SelectItem value="2nd">2nd</SelectItem>
                    <SelectItem value="3rd">3rd</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <div>
                <FieldLabel>Relationship</FieldLabel>
                <StarRating
                  value={formData.relationship}
                  onChange={(v) => setFormData({ ...formData, relationship: v })}
                  disabled={!isAdmin}
                />
              </div>
            </div>
            <div>
              <FieldLabel>Notes</FieldLabel>
              <RichTextEditor
                content={formData.notes}
                onChange={(html) => setFormData({ ...formData, notes: html })}
                placeholder="Internal notes about this contact..."
                minHeight={80}
                disabled={!isAdmin}
              />
            </div>
          </CollapsibleSection>

          {/* Requirements section */}
          <CollapsibleSection title="Requirements" defaultOpen={hasRequirements}>
            <div>
              <FieldLabel>Dietary requirements</FieldLabel>
              <div className="flex flex-wrap gap-x-4 gap-y-2 mb-2">
                {DIETARY_OPTIONS.map(({ value, label }) => (
                  <label key={value} className="flex items-center gap-1.5 cursor-pointer">
                    <Checkbox
                      checked={formData.dietary_requirements.includes(value)}
                      onCheckedChange={(checked) => {
                        setFormData((prev) => ({
                          ...prev,
                          dietary_requirements: checked
                            ? [...prev.dietary_requirements, value]
                            : prev.dietary_requirements.filter((v) => v !== value),
                        }))
                      }}
                      disabled={!isAdmin}
                    />
                    <span className="text-sm">{label}</span>
                  </label>
                ))}
              </div>
              <Input
                placeholder="Other dietary requirements"
                value={formData.dietary_requirements_other}
                onChange={(e) => setFormData({ ...formData, dietary_requirements_other: e.target.value })}
                disabled={!isAdmin}
              />
            </div>

            <div>
              <FieldLabel>Accessibility requirements</FieldLabel>
              <div className="flex flex-wrap gap-x-4 gap-y-2 mb-2">
                {ACCESSIBILITY_OPTIONS.map(({ value, label }) => (
                  <label key={value} className="flex items-center gap-1.5 cursor-pointer">
                    <Checkbox
                      checked={formData.accessibility_requirements.includes(value)}
                      onCheckedChange={(checked) => {
                        setFormData((prev) => ({
                          ...prev,
                          accessibility_requirements: checked
                            ? [...prev.accessibility_requirements, value]
                            : prev.accessibility_requirements.filter((v) => v !== value),
                        }))
                      }}
                      disabled={!isAdmin}
                    />
                    <span className="text-sm">{label}</span>
                  </label>
                ))}
              </div>
              <Input
                placeholder="Other accessibility requirements"
                value={formData.accessibility_requirements_other}
                onChange={(e) => setFormData({ ...formData, accessibility_requirements_other: e.target.value })}
                disabled={!isAdmin}
              />
            </div>
          </CollapsibleSection>

          {/* Bio section */}
          <CollapsibleSection title="Bio" defaultOpen={hasBio}>
            <div>
              <FieldLabel>Bio</FieldLabel>
              <RichTextEditor
                content={formData.bio}
                onChange={(html) => setFormData({ ...formData, bio: html })}
                placeholder="Brief biography"
                minHeight={100}
                disabled={!isAdmin}
              />
            </div>
          </CollapsibleSection>

          {/* Social and web section */}
          <CollapsibleSection title="Social and web" defaultOpen={hasSocial}>
            <div>
              <FieldLabel htmlFor="linkedin">LinkedIn</FieldLabel>
              <div className="flex gap-2">
                <Input
                  id="linkedin"
                  value={formData.linkedin}
                  onChange={(e) => setFormData({ ...formData, linkedin: e.target.value })}
                  disabled={!isAdmin}
                  placeholder="https://linkedin.com/in/..."
                />
                {formData.linkedin && (
                  <Button variant="outline" size="icon" asChild>
                    <a href={formData.linkedin} target="_blank" rel="noopener noreferrer">
                      <ExternalLink className="h-4 w-4" />
                    </a>
                  </Button>
                )}
              </div>
            </div>
            <div>
              <FieldLabel htmlFor="instagram">Instagram</FieldLabel>
              <Input
                id="instagram"
                value={formData.instagram}
                onChange={(e) => setFormData({ ...formData, instagram: e.target.value })}
                disabled={!isAdmin}
                placeholder="https://instagram.com/..."
              />
            </div>
            <div>
              <FieldLabel htmlFor="website">Website</FieldLabel>
              <Input
                id="website"
                value={formData.website}
                onChange={(e) => setFormData({ ...formData, website: e.target.value })}
                disabled={!isAdmin}
                placeholder="https://example.com"
              />
            </div>
          </CollapsibleSection>

          {/* Linked contacts section - only for existing contacts */}
          {!isNew && contact && (
            <LinkedContactsSection
              contact={contact}
              isAdmin={isAdmin}
            />
          )}

          {/* Activity section - only for existing contacts */}
          {!isNew && (
            <CollapsibleSection title="Activity" defaultOpen={false} badge={activities.length}>
              {activities.length === 0 ? (
                <p className="text-sm text-muted-foreground">No activities recorded yet.</p>
              ) : (
                <div className="space-y-3">
                  {activities.map((activity: Activity) => {
                    const Icon = getActivityIcon(activity.source_app)
                    return (
                      <div key={activity.id} className="flex gap-3 text-sm">
                        <div className="flex-shrink-0 w-8 h-8 rounded-full bg-muted flex items-center justify-center">
                          <Icon className="w-4 h-4 text-muted-foreground" />
                        </div>
                        <div className="flex-1 min-w-0">
                          <p>{activity.title || activity.type}</p>
                          <div className="flex items-center gap-2 mt-0.5 text-muted-foreground">
                            <span>{activity.source_app}</span>
                            {activity.occurred_at && (
                              <span>{new Date(activity.occurred_at).toLocaleDateString()}</span>
                            )}
                            {activity.source_url && (
                              <a
                                href={activity.source_url}
                                target="_blank"
                                rel="noopener noreferrer"
                                className="text-primary hover:underline"
                              >
                                View
                              </a>
                            )}
                          </div>
                        </div>
                      </div>
                    )
                  })}
                </div>
              )}
            </CollapsibleSection>
          )}

          {/* Created/updated section - only for existing contacts */}
          {!isNew && contact && (
            <CollapsibleSection title="Created/updated" defaultOpen={false}>
              <div className="space-y-2 text-sm text-muted-foreground">
                {contact.source && (
                  <p>
                    Source: <span className="text-foreground">{contact.source}</span>
                  </p>
                )}
                <p>
                  Created: <span className="text-foreground">{new Date(contact.created).toLocaleDateString()}</span>
                </p>
                <p>
                  Updated: <span className="text-foreground">{new Date(contact.updated).toLocaleDateString()}</span>
                </p>
              </div>
            </CollapsibleSection>
          )}
        </form>

        {/* Action buttons in sticky footer */}
        {isAdmin && (
          <ContactDrawerFooter
            isNew={isNew}
            onDelete={handleDelete}
            onSubmit={handleSubmit}
            isDeleting={deleteMutation.isPending}
            isSaving={saveMutation.isPending}
          />
        )}
      </SheetContent>
    </Sheet>
  )
}
