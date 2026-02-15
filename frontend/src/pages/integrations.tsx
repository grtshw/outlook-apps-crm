import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { fetchJSON } from '@/lib/api'
import { PageHeader } from '@/components/ui/page-header'
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
import { Loader2, RefreshCw, Mail } from 'lucide-react'

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
// These question IDs map additional fields to CRM contact fields
const FIELD_MAPPINGS: Record<string, Record<string, string>> = {
  // Default mapping based on The Outlook 2026 event
  default: {
    first_name: '65599f1116aac86ec758fcc7',
    last_name: '65599f1116aac86ec758fcc8',
    email: '65599f1116aac86ec758fcc9',
    phone: '65599f1116aac86ec758fcca',
    organisation: '65599f1116aac86ec758fccb',
    job_title: '6559a4046aa6091b396e00c8',
  },
}

export function IntegrationsPage() {
  const queryClient = useQueryClient()
  const [selectedEvent, setSelectedEvent] = useState('')

  const { data: events = [], isLoading: eventsLoading } = useQuery({
    queryKey: ['humanitix-events'],
    queryFn: () => fetchJSON<HumanitixEvent[]>('/api/admin/humanitix/events'),
  })

  const { data: syncLogs = [], isLoading: logsLoading } = useQuery({
    queryKey: ['humanitix-sync-logs'],
    queryFn: () => fetchJSON<SyncLog[]>('/api/admin/humanitix/sync-logs'),
    refetchInterval: 5000, // Poll while syncs are running
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

  const mailchimpSyncMutation = useMutation({
    mutationFn: () =>
      fetchJSON<{ message: string }>('/api/admin/mailchimp/sync', {
        method: 'POST',
      }),
    onSuccess: () => toast.success('Mailchimp sync started'),
    onError: (error: Error) => toast.error(error.message),
  })

  return (
    <div>
      <PageHeader title="Integrations" />

      <div className="p-6 space-y-8">
        {/* Humanitix section */}
        <section className="space-y-4">
          <h2 className="text-lg">Humanitix</h2>
          <p className="text-sm text-muted-foreground">
            Sync attendee and ticket data from Humanitix events into CRM contacts.
          </p>

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

          {/* Sync logs */}
          <div>
            <h3 className="text-sm text-muted-foreground mb-2">Sync history</h3>
            {logsLoading ? (
              <div className="flex items-center gap-2 text-sm text-muted-foreground py-4">
                <Loader2 className="w-4 h-4 animate-spin" /> Loading...
              </div>
            ) : syncLogs.length === 0 ? (
              <p className="text-sm text-muted-foreground py-4">No syncs yet.</p>
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
          </div>
        </section>

        {/* Mailchimp section */}
        <section className="space-y-4">
          <h2 className="text-lg">Mailchimp</h2>
          <p className="text-sm text-muted-foreground">
            Sync active contacts to your Mailchimp audience. Subscription changes and email engagement are received via webhook.
          </p>

          <Button
            onClick={() => mailchimpSyncMutation.mutate()}
            disabled={mailchimpSyncMutation.isPending}
          >
            {mailchimpSyncMutation.isPending ? (
              <>
                <Loader2 className="w-4 h-4 animate-spin mr-2" /> Syncing...
              </>
            ) : (
              <>
                <Mail className="w-4 h-4 mr-2" /> Sync all active contacts
              </>
            )}
          </Button>
        </section>
      </div>
    </div>
  )
}
