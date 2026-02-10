import { useState, useMemo } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { getContact, getContactActivities, mergeContacts } from '@/lib/api'
import type { Contact, ContactRole } from '@/lib/pocketbase'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Avatar, AvatarFallback, AvatarImage } from '@/components/ui/avatar'
import { Badge } from '@/components/ui/badge'
import { Checkbox } from '@/components/ui/checkbox'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Separator } from '@/components/ui/separator'
import { Check, Loader2, AlertTriangle } from 'lucide-react'
import { cn } from '@/lib/utils'

interface MergeContactsDialogProps {
  open: boolean
  onClose: () => void
  contactIds: string[]
}

const SCALAR_FIELDS: { key: string; label: string }[] = [
  { key: 'name', label: 'Name' },
  { key: 'email', label: 'Email' },
  { key: 'phone', label: 'Phone' },
  { key: 'pronouns', label: 'Pronouns' },
  { key: 'job_title', label: 'Job title' },
  { key: 'bio', label: 'Bio' },
  { key: 'location', label: 'Location' },
  { key: 'linkedin', label: 'LinkedIn' },
  { key: 'instagram', label: 'Instagram' },
  { key: 'website', label: 'Website' },
  { key: 'organisation', label: 'Organisation' },
  { key: 'status', label: 'Status' },
  { key: 'source', label: 'Source' },
]

const ALL_ROLES: ContactRole[] = ['presenter', 'speaker', 'sponsor', 'judge', 'attendee', 'staff', 'volunteer']

function initials(name: string) {
  return name
    .split(' ')
    .map((n) => n[0])
    .join('')
    .toUpperCase()
    .slice(0, 2)
}

function getDisplayValue(contact: Contact, field: string): string {
  if (field === 'organisation') {
    return contact.organisation_name || contact.organisation || ''
  }
  const val = (contact as Record<string, unknown>)[field]
  if (val === null || val === undefined || val === '') return ''
  return String(val)
}

export function MergeContactsDialog({ open, onClose, contactIds }: MergeContactsDialogProps) {
  const queryClient = useQueryClient()

  // Fetch full contact data for all selected IDs
  const contactQueries = contactIds.map((id) =>
    // eslint-disable-next-line react-hooks/rules-of-hooks
    useQuery({
      queryKey: ['contact', id],
      queryFn: () => getContact(id),
      enabled: open && !!id,
    })
  )

  // Fetch activity counts for all contacts
  const activityQueries = contactIds.map((id) =>
    // eslint-disable-next-line react-hooks/rules-of-hooks
    useQuery({
      queryKey: ['contact-activities', id],
      queryFn: () => getContactActivities(id),
      enabled: open && !!id,
    })
  )

  const contacts = contactQueries
    .map((q) => q.data)
    .filter((c): c is Contact => !!c)

  const isLoading = contactQueries.some((q) => q.isLoading)

  const totalActivities = activityQueries.reduce((sum, q) => sum + (q.data?.length || 0), 0)

  // State
  const [primaryId, setPrimaryId] = useState<string>(contactIds[0])
  const [fieldSelections, setFieldSelections] = useState<Record<string, string>>({})
  const [mergedRoles, setMergedRoles] = useState<string[]>([])
  const [mergedTags, setMergedTags] = useState<string[]>([])
  const [rolesInitialised, setRolesInitialised] = useState(false)
  const [tagsInitialised, setTagsInitialised] = useState(false)

  // Initialise defaults once contacts are loaded
  if (contacts.length === contactIds.length && !rolesInitialised) {
    // Default field selections to primary contact
    const defaults: Record<string, string> = {}
    for (const { key } of SCALAR_FIELDS) {
      defaults[key] = primaryId
    }
    // Avatar group follows primary too
    defaults['avatar_url'] = primaryId
    setFieldSelections(defaults)

    // Union all roles
    const allRoles = new Set<string>()
    contacts.forEach((c) => c.roles?.forEach((r) => allRoles.add(r)))
    setMergedRoles(Array.from(allRoles))
    setRolesInitialised(true)
  }

  if (contacts.length === contactIds.length && !tagsInitialised) {
    const allTags = new Set<string>()
    contacts.forEach((c) => c.tags?.forEach((t) => allTags.add(t)))
    setMergedTags(Array.from(allTags))
    setTagsInitialised(true)
  }

  // Merged source_ids preview
  const mergedSourceIds = useMemo(() => {
    const merged: Record<string, string> = {}
    contacts.forEach((c) => {
      if (c.source_ids) {
        Object.entries(c.source_ids).forEach(([k, v]) => {
          merged[k] = v
        })
      }
    })
    return merged
  }, [contacts])

  const selectField = (field: string, contactId: string) => {
    setFieldSelections((prev) => ({ ...prev, [field]: contactId }))
  }

  const toggleRole = (role: string) => {
    setMergedRoles((prev) =>
      prev.includes(role) ? prev.filter((r) => r !== role) : [...prev, role]
    )
  }

  const toggleTag = (tag: string) => {
    setMergedTags((prev) =>
      prev.includes(tag) ? prev.filter((t) => t !== tag) : [...prev, tag]
    )
  }

  // Build the actual avatar field selections based on which contact's avatar set is chosen
  const avatarSourceId = fieldSelections['avatar_url'] || primaryId

  const mergeMutation = useMutation({
    mutationFn: () => {
      // Build the full field selections including avatar fields
      const fullSelections = { ...fieldSelections }
      // Map avatar group to individual fields
      fullSelections['avatar_url'] = avatarSourceId
      fullSelections['avatar_thumb_url'] = avatarSourceId
      fullSelections['avatar_small_url'] = avatarSourceId
      fullSelections['avatar_original_url'] = avatarSourceId

      return mergeContacts({
        primary_id: primaryId,
        merged_ids: contactIds.filter((id) => id !== primaryId),
        field_selections: fullSelections,
        merged_roles: mergedRoles,
        merged_tags: mergedTags,
      })
    },
    onSuccess: (data) => {
      toast.success(
        `Contacts merged. ${data.activities_reassigned} activities reassigned, ${data.contacts_deleted} contacts removed.`
      )
      queryClient.invalidateQueries({ queryKey: ['contacts'] })
      queryClient.invalidateQueries({ queryKey: ['dashboard-stats'] })
      onClose()
    },
    onError: (error: Error) => {
      toast.error(error.message)
    },
  })

  const primaryContact = contacts.find((c) => c.id === primaryId)
  const mergedCount = contactIds.length - 1

  // Collect all unique tags across contacts
  const allTags = useMemo(() => {
    const tags = new Set<string>()
    contacts.forEach((c) => c.tags?.forEach((t) => tags.add(t)))
    return Array.from(tags)
  }, [contacts])

  // Collect all roles present across contacts
  const presentRoles = useMemo(() => {
    const roles = new Set<string>()
    contacts.forEach((c) => c.roles?.forEach((r) => roles.add(r)))
    return ALL_ROLES.filter((r) => roles.has(r))
  }, [contacts])

  if (!open) return null

  return (
    <Dialog open={open} onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="sm:max-w-3xl max-h-[90vh] flex flex-col">
        <DialogHeader>
          <DialogTitle>Merge contacts</DialogTitle>
          <DialogDescription>
            Select which values to keep for the merged contact. The primary contact's ID is preserved.
          </DialogDescription>
        </DialogHeader>

        {isLoading ? (
          <div className="flex items-center justify-center py-12">
            <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
          </div>
        ) : (
          <ScrollArea className="flex-1 -mx-6 px-6">
            <div className="space-y-6 pb-4">
              {/* Primary contact selection */}
              <div>
                <p className="text-sm text-muted-foreground mb-3">Primary contact (survives)</p>
                <div className="grid gap-2">
                  {contacts.map((contact) => (
                    <button
                      key={contact.id}
                      type="button"
                      onClick={() => setPrimaryId(contact.id)}
                      className={cn(
                        'flex items-center gap-3 p-3 rounded-lg border text-left transition-colors',
                        primaryId === contact.id
                          ? 'border-primary bg-primary/5'
                          : 'border-border hover:bg-muted/50'
                      )}
                    >
                      <Avatar className="h-8 w-8">
                        <AvatarImage src={contact.avatar_thumb_url || contact.avatar_url} />
                        <AvatarFallback className="text-xs">
                          {initials(contact.name)}
                        </AvatarFallback>
                      </Avatar>
                      <div className="flex-1 min-w-0">
                        <div className="text-sm">{contact.name}</div>
                        <div className="text-xs text-muted-foreground truncate">{contact.email}</div>
                      </div>
                      {primaryId === contact.id && (
                        <Check className="h-4 w-4 text-primary shrink-0" />
                      )}
                    </button>
                  ))}
                </div>
              </div>

              <Separator />

              {/* Field-by-field resolution */}
              <div>
                <p className="text-sm text-muted-foreground mb-3">Choose values to keep</p>
                <div className="space-y-1">
                  {SCALAR_FIELDS.map(({ key, label }) => {
                    // Get values from all contacts
                    const values = contacts.map((c) => ({
                      id: c.id,
                      value: getDisplayValue(c, key),
                      name: c.name,
                    }))

                    // Skip if all values are the same or all empty
                    const nonEmpty = values.filter((v) => v.value)
                    const uniqueValues = new Set(nonEmpty.map((v) => v.value))
                    if (uniqueValues.size <= 1 && nonEmpty.length > 0) {
                      // All same - auto-select and don't show
                      return null
                    }

                    return (
                      <div key={key} className="grid grid-cols-[120px_1fr] gap-2 items-start py-2">
                        <span className="text-sm text-muted-foreground pt-1.5">{label}</span>
                        <div className="flex flex-wrap gap-1.5">
                          {values.map((v) => (
                            <button
                              key={v.id}
                              type="button"
                              onClick={() => selectField(key, v.id)}
                              className={cn(
                                'text-sm px-3 py-1.5 rounded-md border transition-colors text-left max-w-[200px] truncate',
                                fieldSelections[key] === v.id
                                  ? 'border-primary bg-primary/5'
                                  : 'border-border hover:bg-muted/50',
                                !v.value && 'text-muted-foreground italic'
                              )}
                              title={v.value || `Empty (${v.name})`}
                            >
                              {v.value || '—'}
                            </button>
                          ))}
                        </div>
                      </div>
                    )
                  })}
                </div>
              </div>

              {/* Avatar selection */}
              {contacts.some((c) => c.avatar_url || c.avatar_thumb_url) && (
                <>
                  <Separator />
                  <div>
                    <p className="text-sm text-muted-foreground mb-3">Avatar</p>
                    <div className="flex gap-3">
                      {contacts.map((contact) => {
                        const hasAvatar = !!(contact.avatar_thumb_url || contact.avatar_url)
                        return (
                          <button
                            key={contact.id}
                            type="button"
                            onClick={() => selectField('avatar_url', contact.id)}
                            className={cn(
                              'flex flex-col items-center gap-2 p-3 rounded-lg border transition-colors',
                              avatarSourceId === contact.id
                                ? 'border-primary bg-primary/5'
                                : 'border-border hover:bg-muted/50'
                            )}
                          >
                            <Avatar className="h-12 w-12">
                              <AvatarImage src={contact.avatar_thumb_url || contact.avatar_url} />
                              <AvatarFallback>{initials(contact.name)}</AvatarFallback>
                            </Avatar>
                            <span className={cn('text-xs', !hasAvatar && 'text-muted-foreground')}>
                              {hasAvatar ? contact.name.split(' ')[0] : 'None'}
                            </span>
                          </button>
                        )
                      })}
                    </div>
                  </div>
                </>
              )}

              {/* Roles union */}
              {presentRoles.length > 0 && (
                <>
                  <Separator />
                  <div>
                    <p className="text-sm text-muted-foreground mb-3">Roles (combined)</p>
                    <div className="flex flex-wrap gap-2">
                      {presentRoles.map((role) => (
                        <label
                          key={role}
                          className="flex items-center gap-2 cursor-pointer"
                        >
                          <Checkbox
                            checked={mergedRoles.includes(role)}
                            onCheckedChange={() => toggleRole(role)}
                          />
                          <Badge variant="secondary">{role}</Badge>
                        </label>
                      ))}
                    </div>
                  </div>
                </>
              )}

              {/* Tags union */}
              {allTags.length > 0 && (
                <>
                  <Separator />
                  <div>
                    <p className="text-sm text-muted-foreground mb-3">Tags (combined)</p>
                    <div className="flex flex-wrap gap-2">
                      {allTags.map((tag) => (
                        <label
                          key={tag}
                          className="flex items-center gap-2 cursor-pointer"
                        >
                          <Checkbox
                            checked={mergedTags.includes(tag)}
                            onCheckedChange={() => toggleTag(tag)}
                          />
                          <Badge variant="outline">{tag}</Badge>
                        </label>
                      ))}
                    </div>
                  </div>
                </>
              )}

              {/* Source IDs preview */}
              {Object.keys(mergedSourceIds).length > 0 && (
                <>
                  <Separator />
                  <div>
                    <p className="text-sm text-muted-foreground mb-2">Source IDs (auto-merged)</p>
                    <div className="flex flex-wrap gap-1.5">
                      {Object.entries(mergedSourceIds).map(([key, val]) => (
                        <Badge key={key} variant="outline" className="text-xs">
                          {key}: {val}
                        </Badge>
                      ))}
                    </div>
                  </div>
                </>
              )}

              <Separator />

              {/* Confirmation summary */}
              <div className="flex items-start gap-3 p-3 rounded-lg bg-muted/50">
                <AlertTriangle className="h-4 w-4 text-muted-foreground shrink-0 mt-0.5" />
                <div className="text-sm">
                  <p>
                    Merging {contactIds.length} contacts into{' '}
                    <span className="text-foreground">{primaryContact?.name}</span>.{' '}
                    {totalActivities > 0 && (
                      <>{totalActivities} activities will be reassigned. </>
                    )}
                    {mergedCount} contact{mergedCount > 1 ? 's' : ''} will be permanently deleted.
                  </p>
                </div>
              </div>
            </div>
          </ScrollArea>
        )}

        <DialogFooter>
          <Button variant="outline" onClick={onClose}>
            Cancel
          </Button>
          <Button
            variant="destructive"
            onClick={() => mergeMutation.mutate()}
            disabled={isLoading || mergeMutation.isPending}
          >
            {mergeMutation.isPending ? 'Merging...' : 'Merge contacts'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
