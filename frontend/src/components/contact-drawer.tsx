import { useEffect, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import type { Contact, Activity } from '@/lib/pocketbase'
import { createContact, updateContact, deleteContact, getContactActivities } from '@/lib/api'
import { useAuth } from '@/hooks/use-pocketbase'
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetFooter,
  SheetTitle,
  SheetSection,
} from '@/components/ui/sheet'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { RichTextEditor } from '@/components/rich-text-editor'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Avatar, AvatarFallback, AvatarImage } from '@/components/ui/avatar'
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from '@/components/ui/collapsible'
import { Trash2, ExternalLink, ChevronDown, Presentation, Trophy, Calendar, Image, Mail, Clock } from 'lucide-react'
import { cn } from '@/lib/utils'

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
    default:
      return Clock
  }
}

function getAvatarSrc(contact: Contact | null) {
  if (!contact) return undefined
  return contact.avatar_small_url || contact.avatar_thumb_url || contact.avatar_url || undefined
}

export function ContactDrawer({ open, onClose, contact }: ContactDrawerProps) {
  const { isAdmin } = useAuth()
  const queryClient = useQueryClient()
  const isNew = !contact?.id

  const [formData, setFormData] = useState({
    name: '',
    email: '',
    phone: '',
    pronouns: '',
    job_title: '',
    bio: '',
    linkedin: '',
    instagram: '',
    website: '',
    location: '',
    status: 'active' as Contact['status'],
  })

  // Load activities for existing contacts
  const { data: activities = [] } = useQuery({
    queryKey: ['contact-activities', contact?.id],
    queryFn: () => getContactActivities(contact!.id),
    enabled: !!contact?.id && open,
  })

  useEffect(() => {
    if (contact) {
      setFormData({
        name: contact.name || '',
        email: contact.email || '',
        phone: contact.phone || '',
        pronouns: contact.pronouns || '',
        job_title: contact.job_title || '',
        bio: contact.bio || '',
        linkedin: contact.linkedin || '',
        instagram: contact.instagram || '',
        website: contact.website || '',
        location: contact.location || '',
        status: contact.status || 'active',
      })
    } else {
      setFormData({
        name: '',
        email: '',
        phone: '',
        pronouns: '',
        job_title: '',
        bio: '',
        linkedin: '',
        instagram: '',
        website: '',
        location: '',
        status: 'active',
      })
    }
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

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    saveMutation.mutate(formData)
  }

  const handleDelete = () => {
    if (confirm('Are you sure you want to delete this contact?')) {
      deleteMutation.mutate()
    }
  }

  const initials = formData.name
    .split(' ')
    .map((n) => n[0])
    .join('')
    .toUpperCase()
    .slice(0, 2)

  const hasBioOrLocation = !!(formData.bio || formData.location)
  const hasSocial = !!(formData.linkedin || formData.instagram || formData.website)

  return (
    <Sheet open={open} onOpenChange={(o) => !o && onClose()}>
      <SheetContent>
        <SheetHeader>
          <SheetTitle>{isNew ? 'Add contact' : 'Edit contact'}</SheetTitle>
        </SheetHeader>

        <form onSubmit={handleSubmit} className="flex-1 overflow-y-auto p-6 space-y-4">
          {/* Header with avatar for existing contacts */}
          {!isNew && (
            <div className="flex items-center gap-4 pb-2">
              <Avatar className="h-16 w-16">
                <AvatarImage src={getAvatarSrc(contact)} />
                <AvatarFallback className="text-lg">{initials}</AvatarFallback>
              </Avatar>
              <div>
                <p className="text-lg">{contact?.name}</p>
                {contact?.organisation_name && (
                  <p className="text-sm text-muted-foreground">
                    {contact.organisation_name}
                  </p>
                )}
              </div>
            </div>
          )}

          {/* Details section */}
          <SheetSection title="Details">
            <div className="grid grid-cols-2 gap-4">
              <div>
                <FieldLabel htmlFor="name">Name *</FieldLabel>
                <Input
                  id="name"
                  value={formData.name}
                  onChange={(e) => setFormData({ ...formData, name: e.target.value })}
                  required
                  disabled={!isAdmin}
                />
              </div>
              <div>
                <FieldLabel htmlFor="email">Email *</FieldLabel>
                <Input
                  id="email"
                  type="email"
                  value={formData.email}
                  onChange={(e) => setFormData({ ...formData, email: e.target.value })}
                  required
                  disabled={!isAdmin}
                />
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
                  <SelectItem value="archived">Archived</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </SheetSection>

          {/* Bio and location section */}
          <CollapsibleSection title="Bio and location" defaultOpen={hasBioOrLocation}>
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
          <SheetFooter>
            {!isNew && (
              <Button
                type="button"
                variant="destructive"
                size="icon"
                onClick={handleDelete}
                disabled={deleteMutation.isPending}
              >
                <Trash2 className="h-4 w-4" />
              </Button>
            )}
            <div className="flex-1" />
            <Button
              type="button"
              variant="outline"
              onClick={onClose}
            >
              Cancel
            </Button>
            <Button onClick={handleSubmit} disabled={saveMutation.isPending}>
              {saveMutation.isPending ? 'Saving...' : isNew ? 'Create' : 'Save changes'}
            </Button>
          </SheetFooter>
        )}
      </SheetContent>
    </Sheet>
  )
}
