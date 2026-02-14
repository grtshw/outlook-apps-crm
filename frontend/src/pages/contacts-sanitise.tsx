import { useState, useEffect } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { getContacts, getOrganisations, updateContact } from '@/lib/api'
import type { Contact, Organisation } from '@/lib/pocketbase'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { PageHeader } from '@/components/ui/page-header'
import { OrganisationCombobox } from '@/components/organisation-combobox'
import { ChevronLeft, ChevronRight, Search, Check, Loader2 } from 'lucide-react'
import { Skeleton } from '@/components/ui/skeleton'

interface RowEdits {
  email?: string
  phone?: string
  job_title?: string
  organisation?: string
}

export function ContactsSanitisePage() {
  const queryClient = useQueryClient()
  const [page, setPage] = useState(1)
  const [nameFilter, setNameFilter] = useState('')
  const [emailFilter, setEmailFilter] = useState('')
  const [edits, setEdits] = useState<Record<string, RowEdits>>({})
  const [savingIds, setSavingIds] = useState<Set<string>>(new Set())

  const { data, isLoading } = useQuery({
    queryKey: ['contacts-sanitise', page, nameFilter],
    queryFn: () => getContacts({ page, perPage: 25, search: nameFilter, status: 'all', sort: 'name' }),
  })

  const { data: orgsData } = useQuery({
    queryKey: ['organisations-all'],
    queryFn: () => getOrganisations({ perPage: 100, status: 'all', sort: 'name' }),
  })

  // Reset edits when page/filter changes
  useEffect(() => {
    setEdits({})
  }, [page, nameFilter])

  // Client-side email filter
  const filteredItems = emailFilter
    ? (data?.items ?? []).filter((c) =>
        c.email?.toLowerCase().includes(emailFilter.toLowerCase())
      )
    : (data?.items ?? [])

  const getEdit = (id: string): RowEdits => edits[id] ?? {}

  const setFieldEdit = (id: string, field: keyof RowEdits, value: string) => {
    setEdits((prev) => ({
      ...prev,
      [id]: { ...prev[id], [field]: value },
    }))
  }

  const isDirty = (contact: Contact) => {
    const edit = getEdit(contact.id)
    if (Object.keys(edit).length === 0) return false
    return (
      (edit.email !== undefined && edit.email !== (contact.email ?? '')) ||
      (edit.phone !== undefined && edit.phone !== (contact.phone ?? '')) ||
      (edit.job_title !== undefined && edit.job_title !== (contact.job_title ?? '')) ||
      (edit.organisation !== undefined && edit.organisation !== (contact.organisation ?? ''))
    )
  }

  const handleSave = async (contact: Contact) => {
    const edit = getEdit(contact.id)
    const payload: Record<string, string> = {}
    if (edit.email !== undefined && edit.email !== (contact.email ?? '')) payload.email = edit.email
    if (edit.phone !== undefined && edit.phone !== (contact.phone ?? '')) payload.phone = edit.phone
    if (edit.job_title !== undefined && edit.job_title !== (contact.job_title ?? '')) payload.job_title = edit.job_title
    if (edit.organisation !== undefined && edit.organisation !== (contact.organisation ?? '')) payload.organisation = edit.organisation

    if (Object.keys(payload).length === 0) return

    setSavingIds((prev) => new Set(prev).add(contact.id))
    try {
      await updateContact(contact.id, payload)
      toast.success(`Updated ${contact.name}`)
      setEdits((prev) => {
        const next = { ...prev }
        delete next[contact.id]
        return next
      })
      queryClient.invalidateQueries({ queryKey: ['contacts-sanitise'] })
    } catch (err: any) {
      toast.error(err.message || 'Failed to update')
    } finally {
      setSavingIds((prev) => {
        const next = new Set(prev)
        next.delete(contact.id)
        return next
      })
    }
  }

  const currentValue = (contact: Contact, field: keyof RowEdits) => {
    const edit = getEdit(contact.id)
    if (edit[field] !== undefined) return edit[field]!
    return (contact[field] as string) ?? ''
  }

  return (
    <div className="space-y-4">
      <PageHeader title="Sanitise contacts" />

      <div className="flex items-center gap-4">
        <div className="relative flex-1 max-w-xs">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground" />
          <Input
            placeholder="Name includes..."
            value={nameFilter}
            onChange={(e) => {
              setNameFilter(e.target.value)
              setPage(1)
            }}
            className="pl-9"
          />
        </div>
        <div className="relative flex-1 max-w-xs">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground" />
          <Input
            placeholder="Email includes..."
            value={emailFilter}
            onChange={(e) => setEmailFilter(e.target.value)}
            className="pl-9"
          />
        </div>
      </div>

      <div className="rounded-md border">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="w-[200px]">Name</TableHead>
              <TableHead className="w-[240px]">Email</TableHead>
              <TableHead className="w-[160px]">Phone</TableHead>
              <TableHead className="w-[180px]">Job title</TableHead>
              <TableHead className="w-[220px]">Organisation</TableHead>
              <TableHead className="w-[70px]" />
            </TableRow>
          </TableHeader>
          <TableBody>
            {isLoading
              ? Array.from({ length: 10 }).map((_, i) => (
                  <TableRow key={i}>
                    {Array.from({ length: 6 }).map((_, j) => (
                      <TableCell key={j}>
                        <Skeleton className="h-8 w-full" />
                      </TableCell>
                    ))}
                  </TableRow>
                ))
              : filteredItems.length === 0
              ? (
                <TableRow>
                  <TableCell colSpan={6} className="text-center text-muted-foreground py-8">
                    No contacts found
                  </TableCell>
                </TableRow>
              )
              : filteredItems.map((contact) => (
                  <TableRow key={contact.id}>
                    <TableCell>
                      <a
                        href={`/contacts/${contact.id}`}
                        className="hover:underline"
                        onClick={(e) => {
                          e.preventDefault()
                          window.open(`/contacts/${contact.id}`, '_blank')
                        }}
                      >
                        {contact.name}
                      </a>
                    </TableCell>
                    <TableCell>
                      <Input
                        value={currentValue(contact, 'email')}
                        onChange={(e) => setFieldEdit(contact.id, 'email', e.target.value)}
                        className="h-8"
                      />
                    </TableCell>
                    <TableCell>
                      <Input
                        value={currentValue(contact, 'phone')}
                        onChange={(e) => setFieldEdit(contact.id, 'phone', e.target.value)}
                        className="h-8"
                      />
                    </TableCell>
                    <TableCell>
                      <Input
                        value={currentValue(contact, 'job_title')}
                        onChange={(e) => setFieldEdit(contact.id, 'job_title', e.target.value)}
                        className="h-8"
                      />
                    </TableCell>
                    <TableCell>
                      <OrganisationCombobox
                        value={currentValue(contact, 'organisation')}
                        organisations={orgsData?.items ?? []}
                        onChange={(orgId) => setFieldEdit(contact.id, 'organisation', orgId)}
                      />
                    </TableCell>
                    <TableCell>
                      {isDirty(contact) && (
                        <Button
                          size="sm"
                          variant="ghost"
                          className="h-8 w-8 p-0"
                          disabled={savingIds.has(contact.id)}
                          onClick={() => handleSave(contact)}
                        >
                          {savingIds.has(contact.id) ? (
                            <Loader2 className="h-4 w-4 animate-spin" />
                          ) : (
                            <Check className="h-4 w-4" />
                          )}
                        </Button>
                      )}
                    </TableCell>
                  </TableRow>
                ))}
          </TableBody>
        </Table>
      </div>

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
    </div>
  )
}
