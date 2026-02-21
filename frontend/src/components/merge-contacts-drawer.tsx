import { useState, useMemo } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { getContact, getContactActivities, mergeContacts } from '@/lib/api'
import type { Contact, ContactRole, DietaryRequirement, AccessibilityRequirement } from '@/lib/pocketbase'
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetFooter,
  SheetTitle,
} from '@/components/ui/sheet'
import { Button } from '@/components/ui/button'
import { Avatar, AvatarFallback, AvatarImage } from '@/components/ui/avatar'
import { Badge } from '@/components/ui/badge'
import { Checkbox } from '@/components/ui/checkbox'
import { Separator } from '@/components/ui/separator'
import { Check, Loader2, AlertTriangle } from 'lucide-react'
import { cn } from '@/lib/utils'

interface MergeContactsDrawerProps {
  open: boolean
  onClose: () => void
  contactIds: string[]
}

const SCALAR_FIELDS: { key: string; label: string }[] = [
  { key: 'first_name', label: 'First name' },
  { key: 'last_name', label: 'Last name' },
  { key: 'email', label: 'Email' },
  { key: 'personal_email', label: 'Personal email' },
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
  { key: 'dietary_requirements_other', label: 'Dietary (other)' },
  { key: 'accessibility_requirements_other', label: 'Accessibility (other)' },
]

const ALL_DIETARY: DietaryRequirement[] = ['vegetarian', 'vegan', 'gluten_free', 'dairy_free', 'nut_allergy', 'halal', 'kosher']
const ALL_ACCESSIBILITY: AccessibilityRequirement[] = ['wheelchair_access', 'hearing_assistance', 'vision_assistance', 'sign_language_interpreter', 'mobility_assistance']

const DIETARY_LABELS: Record<DietaryRequirement, string> = {
  vegetarian: 'Vegetarian', vegan: 'Vegan', gluten_free: 'Gluten free',
  dairy_free: 'Dairy free', nut_allergy: 'Nut allergy', halal: 'Halal', kosher: 'Kosher',
}

const ACCESSIBILITY_LABELS: Record<AccessibilityRequirement, string> = {
  wheelchair_access: 'Wheelchair access', hearing_assistance: 'Hearing assistance',
  vision_assistance: 'Vision assistance', sign_language_interpreter: 'Sign language interpreter',
  mobility_assistance: 'Mobility assistance',
}

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

export function MergeContactsDrawer({ open, onClose, contactIds }: MergeContactsDrawerProps) {
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
  const [mergedDietary, setMergedDietary] = useState<string[]>([])
  const [mergedAccessibility, setMergedAccessibility] = useState<string[]>([])
  const [rolesInitialised, setRolesInitialised] = useState(false)
  const [tagsInitialised, setTagsInitialised] = useState(false)
  const [dietaryInitialised, setDietaryInitialised] = useState(false)
  const [accessibilityInitialised, setAccessibilityInitialised] = useState(false)

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

  if (contacts.length === contactIds.length && !dietaryInitialised) {
    const items = new Set<string>()
    contacts.forEach((c) => c.dietary_requirements?.forEach((d) => items.add(d)))
    setMergedDietary(Array.from(items))
    setDietaryInitialised(true)
  }

  if (contacts.length === contactIds.length && !accessibilityInitialised) {
    const items = new Set<string>()
    contacts.forEach((c) => c.accessibility_requirements?.forEach((a) => items.add(a)))
    setMergedAccessibility(Array.from(items))
    setAccessibilityInitialised(true)
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

  const toggleDietary = (item: string) => {
    setMergedDietary((prev) =>
      prev.includes(item) ? prev.filter((d) => d !== item) : [...prev, item]
    )
  }

  const toggleAccessibility = (item: string) => {
    setMergedAccessibility((prev) =>
      prev.includes(item) ? prev.filter((a) => a !== item) : [...prev, item]
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
        merged_dietary_requirements: mergedDietary,
        merged_accessibility_requirements: mergedAccessibility,
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

  // Collect all dietary requirements present across contacts
  const presentDietary = useMemo(() => {
    const items = new Set<string>()
    contacts.forEach((c) => c.dietary_requirements?.forEach((d) => items.add(d)))
    return ALL_DIETARY.filter((d) => items.has(d))
  }, [contacts])

  // Collect all accessibility requirements present across contacts
  const presentAccessibility = useMemo(() => {
    const items = new Set<string>()
    contacts.forEach((c) => c.accessibility_requirements?.forEach((a) => items.add(a)))
    return ALL_ACCESSIBILITY.filter((a) => items.has(a))
  }, [contacts])

  if (!open) return null

  return (
    <Sheet open={open} onOpenChange={(o) => !o && onClose()}>
      <SheetContent>
        <SheetHeader>
          <SheetTitle>Merge contacts</SheetTitle>
          <p className="text-sm text-muted-foreground">
            Select which values to keep for the merged contact. The primary contact's ID is preserved.
          </p>
        </SheetHeader>

        {isLoading ? (
          <div className="flex items-center justify-center py-12">
            <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
          </div>
        ) : (
          <div className="flex-1 overflow-y-auto px-6 pb-4">
            <div className="space-y-6">
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

              {/* Dietary requirements union */}
              {presentDietary.length > 0 && (
                <>
                  <Separator />
                  <div>
                    <p className="text-sm text-muted-foreground mb-3">Dietary requirements (combined)</p>
                    <div className="flex flex-wrap gap-2">
                      {presentDietary.map((item) => (
                        <label
                          key={item}
                          className="flex items-center gap-2 cursor-pointer"
                        >
                          <Checkbox
                            checked={mergedDietary.includes(item)}
                            onCheckedChange={() => toggleDietary(item)}
                          />
                          <Badge variant="secondary">{DIETARY_LABELS[item]}</Badge>
                        </label>
                      ))}
                    </div>
                  </div>
                </>
              )}

              {/* Accessibility requirements union */}
              {presentAccessibility.length > 0 && (
                <>
                  <Separator />
                  <div>
                    <p className="text-sm text-muted-foreground mb-3">Accessibility requirements (combined)</p>
                    <div className="flex flex-wrap gap-2">
                      {presentAccessibility.map((item) => (
                        <label
                          key={item}
                          className="flex items-center gap-2 cursor-pointer"
                        >
                          <Checkbox
                            checked={mergedAccessibility.includes(item)}
                            onCheckedChange={() => toggleAccessibility(item)}
                          />
                          <Badge variant="secondary">{ACCESSIBILITY_LABELS[item]}</Badge>
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
          </div>
        )}

        <SheetFooter>
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
        </SheetFooter>
      </SheetContent>
    </Sheet>
  )
}
