import { useState } from 'react'
import { useParams } from 'react-router'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import {
  getShareInfo,
  sendOTP,
  verifyOTP,
  getSharedGuestList,
  updateSharedGuestListItem,
  updateSharedGuestListLanding,
} from '@/lib/api-public'
import type { ProgramItem } from '@/lib/pocketbase'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Checkbox } from '@/components/ui/checkbox'
import { Textarea } from '@/components/ui/textarea'
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
import { OTPInput } from '@/components/otp-input'
import { RichTextEditor } from '@/components/rich-text-editor'
import { ProgramEditor } from '@/components/program-editor'
import { Loader2, ExternalLink, CircleCheck, XCircle } from 'lucide-react'
import { cn } from '@/lib/utils'

type Tab = 'guests' | 'landing'

export function SharedGuestListPage() {
  const { token } = useParams<{ token: string }>()
  const queryClient = useQueryClient()

  const [otpSent, setOtpSent] = useState(false)
  const [verifying, setVerifying] = useState(false)
  const [policyAccepted, setPolicyAccepted] = useState(false)
  const [editingNotesId, setEditingNotesId] = useState<string | null>(null)
  const [editingNotesValue, setEditingNotesValue] = useState('')
  const [activeTab, setActiveTab] = useState<Tab>('guests')

  // Landing page form state
  const [landingForm, setLandingForm] = useState<{
    landing_headline: string
    landing_description: string
    landing_image_url: string
    landing_program: ProgramItem[]
    landing_content: string
  } | null>(null)

  // Session token stored in sessionStorage (cleared on tab close)
  const sessionKey = `gl-session-${token}`
  const getSession = () => sessionStorage.getItem(sessionKey)
  const setSession = (t: string) => sessionStorage.setItem(sessionKey, t)

  // Fetch share info
  const { data: shareInfo, error: shareError } = useQuery({
    queryKey: ['share-info', token],
    queryFn: () => getShareInfo(token!),
    enabled: !!token,
    retry: false,
  })

  // When share info loads, check for existing session
  const { data: listData, isLoading: listLoading } = useQuery({
    queryKey: ['shared-guest-list', token],
    queryFn: () => getSharedGuestList(token!, getSession()!),
    enabled: !!token && !!getSession(),
    retry: false,
  })

  const errorMessage = shareError instanceof Error ? shareError.message : shareError ? String(shareError) : ''

  // Determine view state from data
  const effectiveState = (() => {
    if (shareError) return 'error'
    if (!shareInfo) return 'loading'
    if (listData) return 'verified'
    if (getSession()) return 'verified'
    if (otpSent) return 'otp-verify'
    return 'otp-prompt'
  })()

  // Send OTP mutation
  const sendOtpMutation = useMutation({
    mutationFn: () => sendOTP(token!),
    onSuccess: (data) => {
      setOtpSent(true)
      toast.success(`Verification code sent to ${data.email}`)
    },
    onError: (error: Error) => {
      toast.error(error.message)
    },
  })

  // Verify OTP
  const handleVerify = async (code: string) => {
    setVerifying(true)
    try {
      const result = await verifyOTP(token!, code)
      if (result.verified) {
        setSession(result.session_token)
        queryClient.invalidateQueries({ queryKey: ['shared-guest-list', token] })
        toast.success('Verified successfully')
      }
    } catch (error: unknown) {
      const msg = error instanceof Error ? error.message : 'Verification failed'
      toast.error(msg)
    } finally {
      setVerifying(false)
    }
  }

  // Update item (invite round or client notes)
  const updateItemMutation = useMutation({
    mutationFn: ({ itemId, data }: { itemId: string; data: { invite_round?: string; client_notes?: string } }) =>
      updateSharedGuestListItem(token!, itemId, data, getSession()!),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['shared-guest-list', token] })
    },
    onError: (error: Error) => {
      toast.error(error.message)
    },
  })

  // Update landing page
  const updateLandingMutation = useMutation({
    mutationFn: () => {
      if (!landingForm) throw new Error('No changes')
      return updateSharedGuestListLanding(token!, landingForm, getSession()!)
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['shared-guest-list', token] })
      toast.success('Landing page updated')
    },
    onError: (error: Error) => {
      toast.error(error.message)
    },
  })

  // Initialize landing form from data
  const initLandingForm = () => {
    if (!listData) return
    if (!landingForm) {
      setLandingForm({
        landing_headline: listData.landing_headline || '',
        landing_description: listData.landing_description || '',
        landing_image_url: listData.landing_image_url || '',
        landing_program: listData.landing_program || [],
        landing_content: listData.landing_content || '',
      })
    }
  }

  // Error state
  if (effectiveState === 'error') {
    return (
      <div className="min-h-screen flex items-center justify-center p-4">
        <div className="text-center space-y-3 max-w-md">
          <p className="text-lg">{errorMessage.includes('expired') ? 'Link expired' : errorMessage.includes('revoked') ? 'Link revoked' : 'Link not found'}</p>
          <p className="text-sm text-muted-foreground">{errorMessage}</p>
        </div>
      </div>
    )
  }

  // Loading state
  if (effectiveState === 'loading') {
    return (
      <div className="min-h-screen flex items-center justify-center">
        <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
      </div>
    )
  }

  // OTP prompt — show list info, offer to send code
  if (effectiveState === 'otp-prompt' && shareInfo) {
    return (
      <div className="min-h-screen flex items-center justify-center p-4">
        <div className="max-w-md w-full space-y-6 text-center">
          <div className="space-y-2">
            <p className="text-lg">{shareInfo.list_name}</p>
            {shareInfo.event_name && (
              <p className="text-sm text-muted-foreground">{shareInfo.event_name}</p>
            )}
          </div>
          <div className="space-y-3">
            <p className="text-sm text-muted-foreground">
              To view this guest list, we need to verify your email address.
            </p>
            <p className="text-sm text-muted-foreground">
              We&apos;ll send a code to {shareInfo.masked_email}
            </p>
            <Button
              onClick={() => sendOtpMutation.mutate()}
              disabled={sendOtpMutation.isPending}
            >
              {sendOtpMutation.isPending ? 'Sending...' : 'Send verification code'}
            </Button>
          </div>
          <p className="text-xs text-muted-foreground">Shared by The Outlook</p>
        </div>
      </div>
    )
  }

  // OTP verify — enter code
  if (effectiveState === 'otp-verify' && shareInfo) {
    return (
      <div className="min-h-screen flex items-center justify-center p-4">
        <div className="max-w-md w-full space-y-6 text-center">
          <div className="space-y-2">
            <p className="text-lg">{shareInfo.list_name}</p>
            <p className="text-sm text-muted-foreground">Enter the verification code</p>
          </div>
          <OTPInput onComplete={handleVerify} disabled={verifying || !policyAccepted} />
          <div
            className="flex items-start gap-3 text-left cursor-pointer rounded-lg border border-border p-4"
            onClick={() => setPolicyAccepted((v) => !v)}
          >
            <Checkbox
              checked={policyAccepted}
              onCheckedChange={(checked) => setPolicyAccepted(checked === true)}
              className="mt-0.5"
            />
            <span className="font-sans text-sm text-muted-foreground leading-relaxed">
              I understand this guest list contains private and confidential information.
              I agree not to copy or download this data and acknowledge The Outlook&apos;s{' '}
              <a
                href="https://theoutlook.io/legal/privacy-policy"
                target="_blank"
                rel="noopener noreferrer"
                className="underline hover:text-foreground"
                onClick={(e) => e.stopPropagation()}
              >
                privacy policy
              </a>.
            </span>
          </div>
          {verifying && (
            <div className="flex items-center justify-center gap-2">
              <Loader2 className="h-4 w-4 animate-spin" />
              <span className="text-sm text-muted-foreground">Verifying...</span>
            </div>
          )}
          <div className="space-y-2">
            <p className="text-xs text-muted-foreground">
              Code sent to {shareInfo.masked_email}. It expires in 10 minutes.
            </p>
            <Button
              variant="ghost"
              size="sm"
              onClick={() => sendOtpMutation.mutate()}
              disabled={sendOtpMutation.isPending}
            >
              Resend code
            </Button>
          </div>
        </div>
      </div>
    )
  }

  // Verified — show guest list
  if (listLoading) {
    return (
      <div className="min-h-screen flex items-center justify-center">
        <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
      </div>
    )
  }

  if (!listData) {
    return (
      <div className="min-h-screen flex items-center justify-center p-4">
        <div className="text-center space-y-3">
          <p className="text-lg">Session expired</p>
          <p className="text-sm text-muted-foreground">Please verify your email again.</p>
          <Button onClick={() => { sessionStorage.removeItem(sessionKey); window.location.reload() }}>
            Start over
          </Button>
        </div>
      </div>
    )
  }

  const showLandingTab = listData.landing_enabled

  return (
    <div className="min-h-screen bg-background">
      <div className="px-4 py-8 sm:px-8">
        {/* Header */}
        <div className="mb-8 space-y-1">
          <p className="text-xl">{listData.list_name}</p>
          {listData.event_name && (
            <p className="text-sm text-muted-foreground">{listData.event_name}</p>
          )}
          <p className="text-sm text-muted-foreground">
            {listData.total_guests} {listData.total_guests === 1 ? 'guest' : 'guests'}
          </p>
        </div>

        {/* Tabs */}
        {showLandingTab && (
          <div className="flex gap-1 mb-6 border-b">
            <button
              type="button"
              onClick={() => setActiveTab('guests')}
              className={cn(
                'px-4 py-2 text-sm border-b-2 -mb-px cursor-pointer',
                activeTab === 'guests'
                  ? 'border-foreground text-foreground'
                  : 'border-transparent text-muted-foreground hover:text-foreground'
              )}
            >
              Guests
            </button>
            <button
              type="button"
              onClick={() => { setActiveTab('landing'); initLandingForm() }}
              className={cn(
                'px-4 py-2 text-sm border-b-2 -mb-px cursor-pointer',
                activeTab === 'landing'
                  ? 'border-foreground text-foreground'
                  : 'border-transparent text-muted-foreground hover:text-foreground'
              )}
            >
              Landing page
            </button>
          </div>
        )}

        {/* Guest table */}
        {activeTab === 'guests' && (
          <>
            {listData.items.length === 0 ? (
              <p className="text-center text-muted-foreground py-12">No guests in this list yet.</p>
            ) : (
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Name</TableHead>
                    <TableHead>Role</TableHead>
                    <TableHead>Company</TableHead>
                    <TableHead>LinkedIn</TableHead>
                    <TableHead className="w-32">Invite round</TableHead>
                    <TableHead className="w-24">RSVP</TableHead>
                    <TableHead>City</TableHead>
                    <TableHead>Notes</TableHead>
                    <TableHead>Your notes</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {listData.items.map((item) => (
                    <TableRow key={item.id}>
                      <TableCell>{item.name}</TableCell>
                      <TableCell className="text-muted-foreground">{item.role}</TableCell>
                      <TableCell className="text-muted-foreground">{item.company}</TableCell>
                      <TableCell>
                        {item.linkedin && (
                          <a
                            href={item.linkedin}
                            target="_blank"
                            rel="noopener noreferrer"
                            className="text-muted-foreground hover:text-foreground inline-flex items-center gap-1"
                          >
                            <ExternalLink className="h-3 w-3" />
                            <span className="text-sm">LinkedIn</span>
                          </a>
                        )}
                      </TableCell>
                      <TableCell>
                        <Select
                          value={item.invite_round || ''}
                          onValueChange={(v) =>
                            updateItemMutation.mutate({ itemId: item.id, data: { invite_round: v } })
                          }
                        >
                          <SelectTrigger className="h-8">
                            <SelectValue placeholder="Select" />
                          </SelectTrigger>
                          <SelectContent>
                            <SelectItem value="1st">1st</SelectItem>
                            <SelectItem value="2nd">2nd</SelectItem>
                            <SelectItem value="maybe">Maybe</SelectItem>
                          </SelectContent>
                        </Select>
                      </TableCell>
                      <TableCell>
                        {item.rsvp_status === 'accepted' ? (
                          <span className="inline-flex items-center gap-1 text-green-600">
                            <CircleCheck className="h-4 w-4" />
                            <span className="text-sm">Accepted</span>
                          </span>
                        ) : item.rsvp_status === 'declined' ? (
                          <span className="inline-flex items-center gap-1 text-muted-foreground">
                            <XCircle className="h-4 w-4" />
                            <span className="text-sm">Declined</span>
                          </span>
                        ) : (
                          <span className="text-muted-foreground">—</span>
                        )}
                      </TableCell>
                      <TableCell className="text-muted-foreground">{item.city}</TableCell>
                      <TableCell className="text-muted-foreground max-w-xs truncate">
                        {item.notes}
                      </TableCell>
                      <TableCell>
                        {editingNotesId === item.id ? (
                          <Textarea
                            value={editingNotesValue}
                            onChange={(e) => setEditingNotesValue(e.target.value)}
                            onBlur={() => {
                              if (editingNotesValue !== (item.client_notes || '')) {
                                updateItemMutation.mutate({
                                  itemId: item.id,
                                  data: { client_notes: editingNotesValue },
                                })
                              }
                              setEditingNotesId(null)
                            }}
                            autoFocus
                            rows={2}
                            className="min-w-[200px]"
                          />
                        ) : (
                          <button
                            type="button"
                            onClick={() => {
                              setEditingNotesId(item.id)
                              setEditingNotesValue(item.client_notes || '')
                            }}
                            className="text-left text-sm cursor-pointer min-w-[100px] min-h-[24px] rounded px-1 -mx-1 hover:bg-muted/50"
                          >
                            {item.client_notes ? (
                              <span className="line-clamp-2">{item.client_notes}</span>
                            ) : (
                              <span className="text-muted-foreground">Add note...</span>
                            )}
                          </button>
                        )}
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            )}
          </>
        )}

        {/* Landing page editor */}
        {activeTab === 'landing' && landingForm && (
          <div className="max-w-2xl space-y-6">
            <div>
              <label className="block text-sm text-muted-foreground mb-1.5">Headline</label>
              <Input
                value={landingForm.landing_headline}
                onChange={(e) =>
                  setLandingForm({ ...landingForm, landing_headline: e.target.value })
                }
                placeholder="Defaults to event name"
              />
            </div>

            <div>
              <label className="block text-sm text-muted-foreground mb-1.5">Hero image URL</label>
              <Input
                value={landingForm.landing_image_url}
                onChange={(e) =>
                  setLandingForm({ ...landingForm, landing_image_url: e.target.value })
                }
                placeholder="https://..."
              />
            </div>

            <div>
              <label className="block text-sm text-muted-foreground mb-1.5">Description</label>
              <RichTextEditor
                content={landingForm.landing_description}
                onChange={(html) =>
                  setLandingForm({ ...landingForm, landing_description: html })
                }
                placeholder="Event description..."
              />
            </div>

            <div>
              <label className="block text-sm text-muted-foreground mb-1.5">Program</label>
              <ProgramEditor
                items={landingForm.landing_program}
                onChange={(items) =>
                  setLandingForm({ ...landingForm, landing_program: items })
                }
              />
            </div>

            <div>
              <label className="block text-sm text-muted-foreground mb-1.5">
                Additional content
              </label>
              <RichTextEditor
                content={landingForm.landing_content}
                onChange={(html) =>
                  setLandingForm({ ...landingForm, landing_content: html })
                }
                placeholder="Additional content below the program..."
              />
            </div>

            <Button
              onClick={() => updateLandingMutation.mutate()}
              disabled={updateLandingMutation.isPending}
            >
              {updateLandingMutation.isPending ? 'Saving...' : 'Save landing page'}
            </Button>
          </div>
        )}

        {/* Footer */}
        <div className="mt-12 pt-6 border-t text-center">
          <p className="text-xs text-muted-foreground">Shared by The Outlook</p>
        </div>
      </div>
    </div>
  )
}
