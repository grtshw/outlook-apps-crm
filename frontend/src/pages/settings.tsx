import { useState, useCallback, useEffect } from 'react'
import { useParams, useNavigate } from 'react-router'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import {
  fetchJSON,
  getMailchimpStatus,
  getMailchimpLists,
  getMailchimpMergeFields,
  getMailchimpSettings,
  saveMailchimpSettings,
} from '@/lib/api'
import type { MergeFieldMapping } from '@/lib/api'
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { Tabs, TabsList, TabsTrigger, TabsContent } from '@/components/ui/tabs'
import { PageHeader } from '@/components/ui/page-header'
import { Skeleton } from '@/components/ui/skeleton'
import { Loader2, RefreshCw, Mail } from 'lucide-react'

// ── Humanitix types ──

interface HumanitixEvent {
  id: string
  name: string
  slug: string
  start_date: string
  end_date: string
}

interface SyncLog {
  id: string
  sync_type: string
  event_id: string
  event_name: string
  records_processed: number
  records_created: number
  records_updated: number
  errors: string[] | null
  status: 'running' | 'completed' | 'failed'
  started_at: string
  completed_at: string
  created: string
}

// Known field mappings for The Outlook events on Humanitix
const FIELD_MAPPINGS: Record<string, Record<string, string>> = {
  default: {
    first_name: '65599f1116aac86ec758fcc7',
    last_name: '65599f1116aac86ec758fcc8',
    email: '65599f1116aac86ec758fcc9',
    phone: '65599f1116aac86ec758fcca',
    organisation: '65599f1116aac86ec758fccb',
    job_title: '6559a4046aa6091b396e00c8',
  },
}

// CRM contact fields available for merge field mapping
const CRM_FIELDS = [
  { value: 'first_name', label: 'First name' },
  { value: 'last_name', label: 'Last name' },
  { value: 'email', label: 'Email' },
  { value: 'phone', label: 'Phone' },
  { value: 'organisation_name', label: 'Organisation' },
  { value: 'job_title', label: 'Job title' },
  { value: 'location', label: 'Location' },
  { value: 'pronouns', label: 'Pronouns' },
]

const NO_MAPPING_VALUE = '__none__'

// ── Main page ──

const VALID_TABS = ['humanitix', 'mailchimp'] as const

export default function SettingsPage() {
  const { tab } = useParams<{ tab?: string }>()
  const navigate = useNavigate()

  const activeTab = VALID_TABS.includes(tab as typeof VALID_TABS[number]) ? tab! : 'humanitix'

  const handleTabChange = useCallback(
    (value: string) => {
      navigate(value === 'humanitix' ? '/settings' : `/settings/${value}`, { replace: true })
    },
    [navigate]
  )

  return (
    <div className="space-y-6">
      <PageHeader title="Settings" />

      <Tabs value={activeTab} onValueChange={handleTabChange} orientation="vertical" className="gap-8">
        <div className="w-44 shrink-0 border-r pr-4">
          <TabsList
            variant="line"
            className="w-full [&>*]:after:hidden [&>*]:rounded-none [&>*]:border-transparent [&>*:not(:last-child)]:!border-b-border"
          >
            <TabsTrigger value="humanitix">Humanitix</TabsTrigger>
            <TabsTrigger value="mailchimp">Mailchimp</TabsTrigger>
          </TabsList>
        </div>

        <TabsContent value="humanitix" className="space-y-6">
          <h2 className="text-xl tracking-tight">Humanitix</h2>
          <HumanitixTab />
        </TabsContent>

        <TabsContent value="mailchimp" className="space-y-6">
          <h2 className="text-xl tracking-tight">Mailchimp</h2>
          <MailchimpTab />
        </TabsContent>
      </Tabs>
    </div>
  )
}

// ── Humanitix tab ──

function HumanitixTab() {
  const queryClient = useQueryClient()
  const [selectedEvent, setSelectedEvent] = useState('')

  const { data: events = [], isLoading: eventsLoading } = useQuery({
    queryKey: ['humanitix-events'],
    queryFn: () => fetchJSON<HumanitixEvent[]>('/api/admin/humanitix/events'),
  })

  const { data: syncLogs = [], isLoading: logsLoading } = useQuery({
    queryKey: ['humanitix-sync-logs'],
    queryFn: () => fetchJSON<SyncLog[]>('/api/admin/humanitix/sync-logs'),
    refetchInterval: (query) =>
      query.state.data?.some((l) => l.status === 'running') ? 5000 : false,
  })

  const syncMutation = useMutation({
    mutationFn: (eventId: string) =>
      fetchJSON<{ sync_log_id: string }>('/api/admin/humanitix/sync', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          event_id: eventId,
          field_mapping: FIELD_MAPPINGS.default,
        }),
      }),
    onSuccess: () => {
      toast.success('Sync started')
      queryClient.invalidateQueries({ queryKey: ['humanitix-sync-logs'] })
    },
    onError: (error: Error) => toast.error(error.message),
  })

  const handleSync = () => {
    if (!selectedEvent) {
      toast.error('Select an event first')
      return
    }
    syncMutation.mutate(selectedEvent)
  }

  const isAnySyncRunning = syncLogs.some((l) => l.status === 'running')

  return (
    <>
      <Card>
        <CardHeader>
          <CardTitle>Sync attendees</CardTitle>
          <CardDescription>
            Sync attendee and ticket data from Humanitix events into CRM contacts
          </CardDescription>
        </CardHeader>
        <CardContent>
          <div className="flex items-end gap-3">
            <div className="flex-1 max-w-sm">
              <label className="block text-sm text-muted-foreground mb-1.5">Event</label>
              {eventsLoading ? (
                <div className="flex items-center gap-2 h-10 text-sm text-muted-foreground">
                  <Loader2 className="w-4 h-4 animate-spin" /> Loading events...
                </div>
              ) : (
                <Select value={selectedEvent} onValueChange={setSelectedEvent}>
                  <SelectTrigger>
                    <SelectValue placeholder="Select event" />
                  </SelectTrigger>
                  <SelectContent>
                    {events.map((event) => (
                      <SelectItem key={event.id} value={event.id}>
                        {event.name}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              )}
            </div>
            <Button
              onClick={handleSync}
              disabled={!selectedEvent || syncMutation.isPending || isAnySyncRunning}
            >
              {syncMutation.isPending ? (
                <>
                  <Loader2 className="w-4 h-4 animate-spin mr-2" /> Syncing...
                </>
              ) : (
                <>
                  <RefreshCw className="w-4 h-4 mr-2" /> Sync attendees
                </>
              )}
            </Button>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Sync history</CardTitle>
        </CardHeader>
        <CardContent>
          {logsLoading ? (
            <div className="flex items-center gap-2 text-sm text-muted-foreground py-4">
              <Loader2 className="w-4 h-4 animate-spin" /> Loading...
            </div>
          ) : syncLogs.length === 0 ? (
            <p className="text-sm text-muted-foreground text-center py-6">No syncs yet</p>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Event</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead className="text-right">Processed</TableHead>
                  <TableHead className="text-right">Created</TableHead>
                  <TableHead className="text-right">Updated</TableHead>
                  <TableHead>Started</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {syncLogs.map((log) => (
                  <TableRow key={log.id}>
                    <TableCell>{log.event_name || log.event_id}</TableCell>
                    <TableCell>
                      <Badge
                        variant={
                          log.status === 'completed'
                            ? 'default'
                            : log.status === 'running'
                            ? 'secondary'
                            : 'destructive'
                        }
                      >
                        {log.status}
                      </Badge>
                    </TableCell>
                    <TableCell className="text-right">{log.records_processed}</TableCell>
                    <TableCell className="text-right">{log.records_created}</TableCell>
                    <TableCell className="text-right">{log.records_updated}</TableCell>
                    <TableCell>
                      {log.started_at
                        ? new Date(log.started_at).toLocaleString()
                        : new Date(log.created).toLocaleString()}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>
    </>
  )
}

// ── Mailchimp tab ──

function MailchimpTab() {
  const queryClient = useQueryClient()

  const { data: status, isLoading: statusLoading } = useQuery({
    queryKey: ['mailchimp-status'],
    queryFn: getMailchimpStatus,
  })

  const { data: settings, isLoading: settingsLoading } = useQuery({
    queryKey: ['mailchimp-settings'],
    queryFn: getMailchimpSettings,
    enabled: status?.configured ?? false,
  })

  const { data: listsData } = useQuery({
    queryKey: ['mailchimp-lists'],
    queryFn: getMailchimpLists,
    enabled: status?.configured ?? false,
  })

  const [selectedListId, setSelectedListId] = useState('')
  const [mappings, setMappings] = useState<MergeFieldMapping[]>([])
  const [hasChanges, setHasChanges] = useState(false)

  // Load merge fields when a list is selected
  const activeListId = selectedListId || settings?.list_id || ''

  const { data: mergeFieldsData } = useQuery({
    queryKey: ['mailchimp-merge-fields', activeListId],
    queryFn: () => getMailchimpMergeFields(activeListId),
    enabled: !!activeListId,
  })

  // Initialise local state from settings once loaded
  const [initialised, setInitialised] = useState(false)
  useEffect(() => {
    if (settings && !initialised) {
      if (settings.list_id) setSelectedListId(settings.list_id)
      if (settings.merge_field_mappings?.length) setMappings(settings.merge_field_mappings)
      setInitialised(true)
    }
  }, [settings, initialised])

  const lists = listsData?.lists || []
  const mergeFields = mergeFieldsData?.merge_fields || []

  const handleListChange = (listId: string) => {
    setSelectedListId(listId)
    setMappings([]) // Reset mappings when list changes
    setHasChanges(true)
  }

  const handleMappingChange = (mailchimpTag: string, crmField: string) => {
    setMappings((prev) => {
      const filtered = prev.filter((m) => m.mailchimp_tag !== mailchimpTag)
      if (crmField && crmField !== NO_MAPPING_VALUE) {
        filtered.push({ mailchimp_tag: mailchimpTag, crm_field: crmField })
      }
      return filtered
    })
    setHasChanges(true)
  }

  const saveMutation = useMutation({
    mutationFn: () => {
      const selectedList = lists.find((l) => l.id === selectedListId)
      return saveMailchimpSettings({
        list_id: selectedListId,
        list_name: selectedList?.name || '',
        merge_field_mappings: mappings,
      })
    },
    onSuccess: () => {
      toast.success('Mailchimp settings saved')
      setHasChanges(false)
      queryClient.invalidateQueries({ queryKey: ['mailchimp-settings'] })
      queryClient.invalidateQueries({ queryKey: ['mailchimp-status'] })
    },
    onError: (error: Error) => toast.error(error.message),
  })

  const syncMutation = useMutation({
    mutationFn: () =>
      fetchJSON<{ message: string }>('/api/admin/mailchimp/sync', {
        method: 'POST',
      }),
    onSuccess: () => toast.success('Mailchimp sync started'),
    onError: (error: Error) => toast.error(error.message),
  })

  if (statusLoading || settingsLoading) {
    return (
      <div className="space-y-6">
        <Skeleton className="h-48 w-full" />
        <Skeleton className="h-32 w-full" />
      </div>
    )
  }

  if (!status?.configured) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>Mailchimp integration</CardTitle>
        </CardHeader>
        <CardContent>
          <p className="text-sm text-muted-foreground text-center py-6">
            Mailchimp not configured. Set the MAILCHIMP_API_KEY environment variable to enable.
          </p>
        </CardContent>
      </Card>
    )
  }

  return (
    <>
      {/* Configuration card */}
      <Card>
        <CardHeader>
          <CardTitle>Configuration</CardTitle>
          <CardDescription>Select which Mailchimp audience to sync contacts to and map fields</CardDescription>
        </CardHeader>
        <CardContent className="space-y-6">
          {/* Status + list selector */}
          <div className="flex items-center gap-3">
            <Badge
              variant="outline"
              className="bg-green-50 text-green-700 border-green-200"
            >
              Connected
            </Badge>
          </div>

          <div className="max-w-sm">
            <label className="block text-sm text-muted-foreground mb-1.5">Audience</label>
            <Select value={selectedListId} onValueChange={handleListChange}>
              <SelectTrigger>
                <SelectValue placeholder="Select audience" />
              </SelectTrigger>
              <SelectContent>
                {lists.map((list) => (
                  <SelectItem key={list.id} value={list.id}>
                    {list.name} ({list.member_count.toLocaleString()} members)
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          {/* Merge field mappings */}
          {activeListId && mergeFields.length > 0 && (
            <div>
              <label className="block text-sm text-muted-foreground mb-2">Merge field mappings</label>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Mailchimp field</TableHead>
                    <TableHead>CRM field</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {mergeFields.map((field) => {
                    const currentMapping = mappings.find(
                      (m) => m.mailchimp_tag === field.tag
                    )
                    return (
                      <TableRow key={field.tag}>
                        <TableCell>
                          <span className="text-sm">{field.name}</span>
                          <span className="text-xs text-muted-foreground ml-2">({field.tag})</span>
                        </TableCell>
                        <TableCell>
                          <Select
                            value={currentMapping?.crm_field || NO_MAPPING_VALUE}
                            onValueChange={(value) => handleMappingChange(field.tag, value)}
                          >
                            <SelectTrigger className="w-48">
                              <SelectValue placeholder="Not mapped" />
                            </SelectTrigger>
                            <SelectContent>
                              <SelectItem value={NO_MAPPING_VALUE}>Not mapped</SelectItem>
                              {CRM_FIELDS.map((f) => (
                                <SelectItem key={f.value} value={f.value}>
                                  {f.label}
                                </SelectItem>
                              ))}
                            </SelectContent>
                          </Select>
                        </TableCell>
                      </TableRow>
                    )
                  })}
                </TableBody>
              </Table>
            </div>
          )}

          {hasChanges && (
            <Button onClick={() => saveMutation.mutate()} disabled={saveMutation.isPending || !selectedListId}>
              {saveMutation.isPending ? 'Saving...' : 'Save settings'}
            </Button>
          )}
        </CardContent>
      </Card>

      {/* Sync card */}
      <Card>
        <CardHeader>
          <CardTitle>Sync contacts</CardTitle>
          <CardDescription>
            Push all active contacts to your Mailchimp audience. Subscription changes and email engagement are received
            via webhook.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <Button
            onClick={() => syncMutation.mutate()}
            disabled={syncMutation.isPending || !status.has_list}
          >
            {syncMutation.isPending ? (
              <>
                <Loader2 className="w-4 h-4 animate-spin mr-2" /> Syncing...
              </>
            ) : (
              <>
                <Mail className="w-4 h-4 mr-2" /> Sync all active contacts
              </>
            )}
          </Button>
          {!status.has_list && (
            <p className="text-sm text-muted-foreground mt-2">
              Select an audience above before syncing.
            </p>
          )}
        </CardContent>
      </Card>
    </>
  )
}
