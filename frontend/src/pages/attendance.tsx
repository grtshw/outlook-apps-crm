import { useState, useMemo } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useSearchParams } from 'react-router'
import { getAttendanceCompanies, type AttendanceCompany } from '@/lib/api'
import { getClearbitLogoUrl, guessWebsiteUrls } from '@/lib/logo'
import { PageHeader } from '@/components/ui/page-header'
import { SearchInput } from '@/components/ui/search-input'
import { Badge } from '@/components/ui/badge'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { Skeleton } from '@/components/ui/skeleton'
import { Building2, Users, CalendarDays } from 'lucide-react'

type SortKey = 'company' | 'attendee_count' | 'event_count'
type SortDir = 'asc' | 'desc'

function CompanyLogo({ company }: { company: AttendanceCompany }) {
  const [failed, setFailed] = useState(false)

  // Try the API-provided logo_url first, then guess from company name
  let logoUrl = company.logo_url
  if (!logoUrl || failed) {
    const guesses = guessWebsiteUrls(company.company)
    if (guesses.length > 0) {
      logoUrl = getClearbitLogoUrl(guesses[0]) ?? ''
    }
  }

  if (!logoUrl || (failed && !company.logo_url)) {
    return (
      <div className="w-8 h-8 rounded-full bg-muted flex items-center justify-center shrink-0">
        <Building2 className="w-4 h-4 text-muted-foreground" />
      </div>
    )
  }

  return (
    <img
      src={logoUrl}
      alt={company.company}
      className="w-8 h-8 rounded-full bg-muted object-contain shrink-0"
      onError={() => setFailed(true)}
    />
  )
}

export function AttendancePage() {
  const [searchParams, setSearchParams] = useSearchParams()
  const search = searchParams.get('search') || ''
  const selectedEvents = searchParams.get('events')?.split(',').filter(Boolean) || []

  const [sortKey, setSortKey] = useState<SortKey>('attendee_count')
  const [sortDir, setSortDir] = useState<SortDir>('desc')

  function updateParams(updates: Record<string, string | undefined>) {
    setSearchParams((prev) => {
      const next = new URLSearchParams(prev)
      for (const [key, val] of Object.entries(updates)) {
        if (val === undefined || val === '') next.delete(key)
        else next.set(key, val)
      }
      return next
    }, { replace: true })
  }

  const { data, isLoading } = useQuery({
    queryKey: ['attendance-companies', selectedEvents.join(',')],
    queryFn: () => getAttendanceCompanies({
      events: selectedEvents.length > 0 ? selectedEvents : undefined,
    }),
  })

  // Client-side search and sort
  const companies = useMemo(() => {
    if (!data?.companies) return []
    let items = data.companies

    if (search) {
      const q = search.toLowerCase()
      items = items.filter((c) =>
        c.company.toLowerCase().includes(q) ||
        c.titles.some((t) => t.toLowerCase().includes(q))
      )
    }

    items = [...items].sort((a, b) => {
      let cmp = 0
      if (sortKey === 'company') {
        cmp = a.company.localeCompare(b.company)
      } else if (sortKey === 'attendee_count') {
        cmp = a.attendee_count - b.attendee_count
      } else if (sortKey === 'event_count') {
        cmp = a.event_count - b.event_count
      }
      return sortDir === 'desc' ? -cmp : cmp
    })

    return items
  }, [data?.companies, search, sortKey, sortDir])

  function toggleSort(key: SortKey) {
    if (sortKey === key) {
      setSortDir((d) => (d === 'asc' ? 'desc' : 'asc'))
    } else {
      setSortKey(key)
      setSortDir(key === 'company' ? 'asc' : 'desc')
    }
  }

  function sortIndicator(key: SortKey) {
    if (sortKey !== key) return ''
    return sortDir === 'asc' ? ' \u2191' : ' \u2193'
  }

  function toggleEvent(eventId: string) {
    const next = selectedEvents.includes(eventId)
      ? selectedEvents.filter((e) => e !== eventId)
      : [...selectedEvents, eventId]
    updateParams({ events: next.length > 0 ? next.join(',') : undefined })
  }

  return (
    <div className="space-y-4">
      <PageHeader title="Attendance" />

      {/* Stats */}
      {data && (
        <div className="flex gap-6 text-sm text-muted-foreground">
          <span className="flex items-center gap-1.5">
            <Building2 className="w-4 h-4" />
            {data.total_companies} companies
          </span>
          <span className="flex items-center gap-1.5">
            <Users className="w-4 h-4" />
            {data.total_attendees} attendees
          </span>
          <span className="flex items-center gap-1.5">
            <CalendarDays className="w-4 h-4" />
            {data.events.length} events
          </span>
          {data.no_company_count > 0 && (
            <span>{data.no_company_count} without company</span>
          )}
        </div>
      )}

      {/* Event filter pills */}
      {data?.events && data.events.length > 1 && (
        <div className="flex flex-wrap gap-2">
          {data.events.map((ev) => (
            <Badge
              key={ev.event_id}
              variant={selectedEvents.includes(ev.event_id) ? 'default' : 'outline'}
              className="cursor-pointer"
              onClick={() => toggleEvent(ev.event_id)}
            >
              {ev.event_name || ev.event_id}
            </Badge>
          ))}
          {selectedEvents.length > 0 && (
            <Badge
              variant="secondary"
              className="cursor-pointer"
              onClick={() => updateParams({ events: undefined })}
            >
              Clear filter
            </Badge>
          )}
        </div>
      )}

      {/* Search */}
      <div className="flex items-center gap-3">
        <SearchInput
          value={search}
          onValueChange={(v) => updateParams({ search: v || undefined })}
          placeholder="Search companies or titles..."
          className="w-64"
        />
      </div>

      {/* Table */}
      {isLoading ? (
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Company</TableHead>
              <TableHead>Attendees</TableHead>
              <TableHead>Events</TableHead>
              <TableHead>Titles</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {Array.from({ length: 8 }).map((_, i) => (
              <TableRow key={i}>
                <TableCell><Skeleton className="h-4 w-32" /></TableCell>
                <TableCell><Skeleton className="h-4 w-12" /></TableCell>
                <TableCell><Skeleton className="h-4 w-12" /></TableCell>
                <TableCell><Skeleton className="h-4 w-48" /></TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      ) : companies.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-12 text-center">
          <Building2 className="w-12 h-12 text-muted-foreground/30 mb-3" />
          <p className="text-sm text-muted-foreground">No companies found</p>
          {search && (
            <p className="mt-1 text-sm text-muted-foreground/80">
              Try a different search term
            </p>
          )}
        </div>
      ) : (
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead
                className="cursor-pointer select-none"
                onClick={() => toggleSort('company')}
              >
                Company{sortIndicator('company')}
              </TableHead>
              <TableHead
                className="cursor-pointer select-none w-[100px]"
                onClick={() => toggleSort('attendee_count')}
              >
                Attendees{sortIndicator('attendee_count')}
              </TableHead>
              <TableHead
                className="cursor-pointer select-none w-[100px]"
                onClick={() => toggleSort('event_count')}
              >
                Events{sortIndicator('event_count')}
              </TableHead>
              <TableHead>Titles</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {companies.map((c) => (
              <TableRow key={c.company}>
                <TableCell>
                  <div className="flex items-center gap-3">
                    <CompanyLogo company={c} />
                    <span>{c.company}</span>
                  </div>
                </TableCell>
                <TableCell>{c.attendee_count}</TableCell>
                <TableCell>{c.event_count}</TableCell>
                <TableCell>
                  <span className="text-muted-foreground text-sm">
                    {c.titles.slice(0, 3).join(', ')}
                    {c.titles.length > 3 && ` +${c.titles.length - 3}`}
                  </span>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      )}

      {/* Count */}
      {!isLoading && companies.length > 0 && (
        <p className="text-sm text-muted-foreground">
          Showing {companies.length} {companies.length === 1 ? 'company' : 'companies'}
        </p>
      )}
    </div>
  )
}
