import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { getContacts, bulkAddGuestListItems } from '@/lib/api'
import type { Contact } from '@/lib/pocketbase'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Badge } from '@/components/ui/badge'
import { Checkbox } from '@/components/ui/checkbox'
import { ScrollArea } from '@/components/ui/scroll-area'
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
  SheetFooter,
} from '@/components/ui/sheet'
import { Search, Loader2, ChevronLeft, ChevronRight } from 'lucide-react'

const PER_PAGE = 25

interface ContactSearchDrawerProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  listId: string
  existingContactIds: string[]
}

export function ContactSearchDrawer({
  open,
  onOpenChange,
  listId,
  existingContactIds,
}: ContactSearchDrawerProps) {
  const queryClient = useQueryClient()
  const [search, setSearch] = useState('')
  const [page, setPage] = useState(1)
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set())

  const { data, isLoading } = useQuery({
    queryKey: ['contacts', 'search-drawer', search, page],
    queryFn: () => getContacts({ search, page, perPage: PER_PAGE }),
    enabled: open,
  })

  const contacts = data?.items ?? []
  const totalItems = data?.totalItems ?? 0
  const totalPages = data?.totalPages ?? 1

  // Group contacts by first letter of name
  const grouped = contacts.reduce<{ letter: string; contacts: Contact[] }[]>(
    (acc, contact) => {
      const letter = (contact.name?.[0] || '#').toUpperCase()
      const last = acc[acc.length - 1]
      if (last && last.letter === letter) {
        last.contacts.push(contact)
      } else {
        acc.push({ letter, contacts: [contact] })
      }
      return acc
    },
    [],
  )

  const addMutation = useMutation({
    mutationFn: () =>
      bulkAddGuestListItems(listId, {
        contact_ids: Array.from(selectedIds),
      }),
    onSuccess: (result) => {
      toast.success(`${result.added} contact${result.added === 1 ? '' : 's'} added to guest list`)
      queryClient.invalidateQueries({ queryKey: ['guest-list-items', listId] })
      queryClient.invalidateQueries({ queryKey: ['guest-list', listId] })
      setSelectedIds(new Set())
      setSearch('')
      setPage(1)
      onOpenChange(false)
    },
    onError: (error: Error) => {
      toast.error(error.message)
    },
  })

  const toggleContact = (contactId: string) => {
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

  const handleAdd = () => {
    if (selectedIds.size > 0) {
      addMutation.mutate()
    }
  }

  const existingSet = new Set(existingContactIds)

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent>
        <SheetHeader>
          <SheetTitle>Add contacts</SheetTitle>
        </SheetHeader>

        <div className="flex-1 min-h-0 flex flex-col gap-4 p-6">
          {/* Search input */}
          <div className="relative">
            <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground" />
            <Input
              placeholder="Search contacts..."
              className="pl-9"
              value={search}
              onChange={(e) => {
                setSearch(e.target.value)
                setPage(1)
              }}
            />
          </div>

          {/* Results */}
          <ScrollArea className="flex-1 min-h-0">
            {isLoading ? (
              <div className="flex items-center justify-center py-12">
                <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
              </div>
            ) : contacts.length === 0 ? (
              <div className="flex items-center justify-center py-12">
                <p className="text-sm text-muted-foreground">
                  {search ? 'No contacts found' : 'No contacts'}
                </p>
              </div>
            ) : (
              <div className="pr-4">
                {grouped.map((group) => (
                  <div key={group.letter}>
                    <div className="sticky top-0 bg-background py-1.5 px-1 z-10">
                      <span className="text-xs text-muted-foreground">{group.letter}</span>
                    </div>
                    <div className="space-y-1 mb-2">
                      {group.contacts.map((contact) => {
                        const isExisting = existingSet.has(contact.id)
                        const isSelected = selectedIds.has(contact.id)

                        return (
                          <label
                            key={contact.id}
                            className={`flex items-center gap-3 p-3 rounded-lg border transition-colors ${
                              isExisting
                                ? 'opacity-50 cursor-not-allowed border-border'
                                : isSelected
                                ? 'border-primary bg-primary/5 cursor-pointer'
                                : 'border-border hover:bg-muted/50 cursor-pointer'
                            }`}
                          >
                            <Checkbox
                              checked={isSelected}
                              onCheckedChange={() => toggleContact(contact.id)}
                              disabled={isExisting}
                            />
                            <div className="flex-1 min-w-0">
                              <div className="flex items-center gap-2">
                                <span className="text-sm truncate">{contact.name}</span>
                                {isExisting && (
                                  <Badge variant="outline">Added</Badge>
                                )}
                              </div>
                              <div className="text-xs text-muted-foreground truncate">
                                {[contact.job_title, contact.organisation_name]
                                  .filter(Boolean)
                                  .join(' · ') || '\u00A0'}
                              </div>
                            </div>
                          </label>
                        )
                      })}
                    </div>
                  </div>
                ))}
              </div>
            )}
          </ScrollArea>

          {/* Pagination */}
          {totalPages > 1 && (
            <div className="flex items-center justify-between pt-2 border-t">
              <p className="text-xs text-muted-foreground">
                {(page - 1) * PER_PAGE + 1}–{Math.min(page * PER_PAGE, totalItems)} of {totalItems}
              </p>
              <div className="flex items-center gap-1">
                <Button
                  variant="outline"
                  size="icon"
                  className="h-7 w-7"
                  onClick={() => setPage((p) => Math.max(1, p - 1))}
                  disabled={page === 1}
                >
                  <ChevronLeft className="w-4 h-4" />
                </Button>
                <span className="text-xs text-muted-foreground px-2">
                  {page} / {totalPages}
                </span>
                <Button
                  variant="outline"
                  size="icon"
                  className="h-7 w-7"
                  onClick={() => setPage((p) => Math.min(totalPages, p + 1))}
                  disabled={page === totalPages}
                >
                  <ChevronRight className="w-4 h-4" />
                </Button>
              </div>
            </div>
          )}
        </div>

        <SheetFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <Button
            onClick={handleAdd}
            disabled={selectedIds.size === 0 || addMutation.isPending}
          >
            {addMutation.isPending
              ? 'Adding...'
              : `Add selected (${selectedIds.size})`}
          </Button>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  )
}
