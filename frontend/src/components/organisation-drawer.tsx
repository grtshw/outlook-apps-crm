import { useEffect, useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import type { Organisation } from '@/lib/pocketbase'
import { createOrganisation, updateOrganisation, deleteOrganisation } from '@/lib/api'
import { useAuth } from '@/hooks/use-pocketbase'
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from '@/components/ui/collapsible'
import { Trash2, ExternalLink, Building2, ChevronDown } from 'lucide-react'
import { cn } from '@/lib/utils'

interface OrganisationDrawerProps {
  open: boolean
  onClose: () => void
  organisation: Organisation | null
}

interface DrawerSectionProps {
  title: string
  children: React.ReactNode
  defaultOpen?: boolean
}

function DrawerSection({ title, children, defaultOpen = true }: DrawerSectionProps) {
  const [isOpen, setIsOpen] = useState(defaultOpen)

  return (
    <Collapsible open={isOpen} onOpenChange={setIsOpen}>
      <div className="border border-border rounded-lg">
        <CollapsibleTrigger className="flex w-full items-center justify-between p-4 hover:bg-muted/50 transition-colors">
          <span className="text-sm">{title}</span>
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

export function OrganisationDrawer({ open, onClose, organisation }: OrganisationDrawerProps) {
  const { isAdmin } = useAuth()
  const queryClient = useQueryClient()
  const isNew = !organisation?.id

  const [formData, setFormData] = useState({
    name: '',
    website: '',
    linkedin: '',
    description_short: '',
    description_medium: '',
    description_long: '',
    status: 'active' as Organisation['status'],
  })

  useEffect(() => {
    if (organisation) {
      setFormData({
        name: organisation.name || '',
        website: organisation.website || '',
        linkedin: organisation.linkedin || '',
        description_short: organisation.description_short || '',
        description_medium: organisation.description_medium || '',
        description_long: organisation.description_long || '',
        status: organisation.status || 'active',
      })
    } else {
      setFormData({
        name: '',
        website: '',
        linkedin: '',
        description_short: '',
        description_medium: '',
        description_long: '',
        status: 'active',
      })
    }
  }, [organisation])

  const saveMutation = useMutation({
    mutationFn: async (data: Partial<Organisation>) => {
      if (organisation?.id) {
        return updateOrganisation(organisation.id, data)
      }
      return createOrganisation(data)
    },
    onSuccess: () => {
      toast.success(isNew ? 'Organisation created' : 'Organisation updated')
      queryClient.invalidateQueries({ queryKey: ['organisations'] })
      queryClient.invalidateQueries({ queryKey: ['dashboard-stats'] })
      onClose()
    },
    onError: (error: Error) => {
      toast.error(error.message)
    },
  })

  const deleteMutation = useMutation({
    mutationFn: () => deleteOrganisation(organisation!.id),
    onSuccess: () => {
      toast.success('Organisation deleted')
      queryClient.invalidateQueries({ queryKey: ['organisations'] })
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
    if (confirm('Are you sure you want to delete this organisation?')) {
      deleteMutation.mutate()
    }
  }

  const hasDescriptions = !!(formData.description_short || formData.description_medium || formData.description_long)
  const hasSocial = !!(formData.website || formData.linkedin)

  return (
    <Sheet open={open} onOpenChange={(o) => !o && onClose()}>
      <SheetContent className="w-full sm:max-w-xl overflow-y-auto">
        <SheetHeader className="pb-4">
          <SheetTitle>{isNew ? 'Add organisation' : 'Edit organisation'}</SheetTitle>
        </SheetHeader>

        <form onSubmit={handleSubmit} className="space-y-4">
          {/* Header with logo for existing organisations */}
          {!isNew && (
            <div className="flex items-center gap-4 pb-4">
              <div className="w-16 h-16 rounded-lg bg-muted flex items-center justify-center overflow-hidden">
                {organisation?.logo_square_url || organisation?.logo_standard_url ? (
                  <img
                    src={organisation.logo_square_url || organisation.logo_standard_url}
                    alt={organisation.name}
                    className="max-w-full max-h-full object-contain p-1"
                  />
                ) : (
                  <Building2 className="w-8 h-8 text-muted-foreground" />
                )}
              </div>
              <div>
                <p className="text-lg">{organisation?.name}</p>
                {organisation?.website && (
                  <a
                    href={organisation.website}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="text-sm text-muted-foreground hover:underline"
                  >
                    {organisation.website.replace(/^https?:\/\//, '')}
                  </a>
                )}
              </div>
            </div>
          )}

          {/* Details section - always open */}
          <div className="border border-border rounded-lg p-4 space-y-4">
            <h3 className="text-sm">Details</h3>

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
              <FieldLabel htmlFor="status">Status</FieldLabel>
              <Select
                value={formData.status}
                onValueChange={(v) => setFormData({ ...formData, status: v as Organisation['status'] })}
                disabled={!isAdmin}
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="active">Active</SelectItem>
                  <SelectItem value="archived">Archived</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>

          {/* Descriptions section */}
          <DrawerSection title="Descriptions" defaultOpen={hasDescriptions}>
            <div>
              <FieldLabel htmlFor="description_short">Short description</FieldLabel>
              <Textarea
                id="description_short"
                value={formData.description_short}
                onChange={(e) => setFormData({ ...formData, description_short: e.target.value })}
                rows={2}
                disabled={!isAdmin}
                placeholder="One-line summary"
                maxLength={100}
              />
              <p className="text-xs text-muted-foreground text-right mt-1">
                {formData.description_short.length}/100
              </p>
            </div>
            <div>
              <FieldLabel htmlFor="description_medium">Medium description</FieldLabel>
              <Textarea
                id="description_medium"
                value={formData.description_medium}
                onChange={(e) => setFormData({ ...formData, description_medium: e.target.value })}
                rows={3}
                disabled={!isAdmin}
                placeholder="Brief paragraph"
                maxLength={300}
              />
              <p className="text-xs text-muted-foreground text-right mt-1">
                {formData.description_medium.length}/300
              </p>
            </div>
            <div>
              <FieldLabel htmlFor="description_long">Long description</FieldLabel>
              <Textarea
                id="description_long"
                value={formData.description_long}
                onChange={(e) => setFormData({ ...formData, description_long: e.target.value })}
                rows={5}
                disabled={!isAdmin}
                placeholder="Full description"
                maxLength={1000}
              />
              <p className="text-xs text-muted-foreground text-right mt-1">
                {formData.description_long.length}/1000
              </p>
            </div>
          </DrawerSection>

          {/* Social and web section */}
          <DrawerSection title="Social and web" defaultOpen={hasSocial}>
            <div>
              <FieldLabel htmlFor="website">Website</FieldLabel>
              <div className="flex gap-2">
                <Input
                  id="website"
                  value={formData.website}
                  onChange={(e) => setFormData({ ...formData, website: e.target.value })}
                  disabled={!isAdmin}
                  placeholder="https://example.com"
                />
                {formData.website && (
                  <Button variant="outline" size="icon" asChild>
                    <a href={formData.website} target="_blank" rel="noopener noreferrer">
                      <ExternalLink className="h-4 w-4" />
                    </a>
                  </Button>
                )}
              </div>
            </div>
            <div>
              <FieldLabel htmlFor="linkedin">LinkedIn</FieldLabel>
              <Input
                id="linkedin"
                value={formData.linkedin}
                onChange={(e) => setFormData({ ...formData, linkedin: e.target.value })}
                disabled={!isAdmin}
                placeholder="https://linkedin.com/company/..."
              />
            </div>
          </DrawerSection>

          {/* Created/updated section - only for existing organisations */}
          {!isNew && organisation && (
            <DrawerSection title="Created/updated" defaultOpen={false}>
              <div className="space-y-2 text-sm text-muted-foreground">
                {organisation.source && (
                  <p>
                    Source: <span className="text-foreground">{organisation.source}</span>
                  </p>
                )}
                <p>
                  Created: <span className="text-foreground">{new Date(organisation.created).toLocaleDateString()}</span>
                </p>
                <p>
                  Updated: <span className="text-foreground">{new Date(organisation.updated).toLocaleDateString()}</span>
                </p>
              </div>
            </DrawerSection>
          )}

          {/* Action buttons */}
          {isAdmin && (
            <div className="flex gap-2 pt-4">
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
              <Button
                type="button"
                variant="outline"
                className="flex-1"
                onClick={onClose}
              >
                Cancel
              </Button>
              <Button type="submit" className="flex-1" disabled={saveMutation.isPending}>
                {saveMutation.isPending ? 'Saving...' : isNew ? 'Create' : 'Save changes'}
              </Button>
            </div>
          )}
        </form>
      </SheetContent>
    </Sheet>
  )
}
