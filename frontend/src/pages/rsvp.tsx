import { useState } from 'react'
import { useParams } from 'react-router'
import { useQuery, useMutation } from '@tanstack/react-query'
import { getRSVPInfo, submitRSVP } from '@/lib/api-public'
import type { RSVPSubmission } from '@/lib/api-public'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Checkbox } from '@/components/ui/checkbox'
import { Loader2, CircleCheck, XCircle } from 'lucide-react'

export function RSVPPage() {
  const { token } = useParams<{ token: string }>()

  const [name, setName] = useState('')
  const [email, setEmail] = useState('')
  const [phone, setPhone] = useState('')
  const [dietary, setDietary] = useState('')
  const [plusOne, setPlusOne] = useState(false)
  const [plusOneName, setPlusOneName] = useState('')
  const [plusOneDietary, setPlusOneDietary] = useState('')
  const [invitedBy, setInvitedBy] = useState('')
  const [prefilled, setPrefilled] = useState(false)
  const [submitted, setSubmitted] = useState(false)
  const [submittedResponse, setSubmittedResponse] = useState<'accepted' | 'declined' | ''>('')

  const { data: info, error, isLoading } = useQuery({
    queryKey: ['rsvp-info', token],
    queryFn: () => getRSVPInfo(token!),
    enabled: !!token,
    retry: false,
  })

  // Pre-fill form when info loads
  if (info && !prefilled) {
    if (info.prefilled_name) setName(info.prefilled_name)
    if (info.prefilled_email) setEmail(info.prefilled_email)
    if (info.prefilled_phone) setPhone(info.prefilled_phone)
    if (info.rsvp_dietary) setDietary(info.rsvp_dietary)
    if (info.rsvp_plus_one) {
      setPlusOne(true)
      if (info.rsvp_plus_one_name) setPlusOneName(info.rsvp_plus_one_name)
      if (info.rsvp_plus_one_dietary) setPlusOneDietary(info.rsvp_plus_one_dietary)
    }
    setPrefilled(true)
  }

  const mutation = useMutation({
    mutationFn: (data: RSVPSubmission) => submitRSVP(token!, data),
    onSuccess: (_, variables) => {
      setSubmitted(true)
      setSubmittedResponse(variables.response)
    },
  })

  const handleSubmit = (response: 'accepted' | 'declined') => {
    mutation.mutate({
      name: name.trim(),
      email: email.trim(),
      phone: phone.trim() || undefined,
      dietary: dietary.trim() || undefined,
      plus_one: plusOne,
      plus_one_name: plusOne ? plusOneName.trim() : undefined,
      plus_one_dietary: plusOne ? plusOneDietary.trim() : undefined,
      response,
      invited_by: info?.type === 'generic' ? invitedBy.trim() || undefined : undefined,
    })
  }

  const errorMessage = error instanceof Error ? error.message : error ? String(error) : ''

  if (isLoading) {
    return (
      <div className="min-h-screen flex items-center justify-center">
        <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
      </div>
    )
  }

  if (error) {
    return (
      <div className="min-h-screen flex items-center justify-center p-4">
        <div className="text-center space-y-3 max-w-md">
          <p className="text-lg">RSVP not available</p>
          <p className="text-sm text-muted-foreground">{errorMessage}</p>
        </div>
      </div>
    )
  }

  if (!info) return null

  // Success screen
  if (submitted) {
    return (
      <div className="min-h-screen flex items-center justify-center p-4">
        <div className="max-w-md w-full space-y-6 text-center">
          {submittedResponse === 'accepted' ? (
            <CircleCheck className="h-16 w-16 text-green-600 mx-auto" />
          ) : (
            <XCircle className="h-16 w-16 text-muted-foreground mx-auto" />
          )}
          <div className="space-y-2">
            <p className="text-xl">
              {submittedResponse === 'accepted' ? "You're confirmed" : 'Response received'}
            </p>
            <p className="text-sm text-muted-foreground">
              {submittedResponse === 'accepted'
                ? `Thanks ${name}, we look forward to seeing you at ${info.event_name || info.list_name}.`
                : `Thanks for letting us know, ${name}.`}
            </p>
          </div>
          <p className="text-xs text-muted-foreground">The Outlook</p>
        </div>
      </div>
    )
  }

  // Already responded — show status with option to update
  const alreadyResponded = info.already_responded && info.rsvp_status

  return (
    <div className="min-h-screen bg-background">
      <div className="max-w-lg mx-auto px-4 py-12">
        {/* Header */}
        <div className="text-center space-y-2 mb-8">
          {info.event_name ? (
            <>
              <p className="text-xl">{info.event_name}</p>
              <p className="text-sm text-muted-foreground">{info.list_name}</p>
            </>
          ) : (
            <p className="text-xl">{info.list_name}</p>
          )}
          {alreadyResponded && (
            <p className="text-sm text-muted-foreground">
              You previously {info.rsvp_status === 'accepted' ? 'accepted' : 'declined'} this invitation. You can update your response below.
            </p>
          )}
        </div>

        {/* Form */}
        <div className="space-y-5">
          <div>
            <label className="block text-sm text-muted-foreground mb-1.5">Name</label>
            <Input
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="Your full name"
              required
            />
          </div>

          <div>
            <label className="block text-sm text-muted-foreground mb-1.5">Email</label>
            <Input
              type="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              placeholder="your@email.com"
              required
            />
          </div>

          <div>
            <label className="block text-sm text-muted-foreground mb-1.5">Phone (optional)</label>
            <Input
              type="tel"
              value={phone}
              onChange={(e) => setPhone(e.target.value)}
              placeholder="Your phone number"
            />
          </div>

          <div>
            <label className="block text-sm text-muted-foreground mb-1.5">Dietary requirements (optional)</label>
            <Textarea
              value={dietary}
              onChange={(e) => setDietary(e.target.value)}
              placeholder="Any dietary requirements or allergies"
              rows={2}
            />
          </div>

          {/* Plus one */}
          <div
            className="flex items-start gap-3 cursor-pointer rounded-lg border border-border p-4"
            onClick={() => setPlusOne((v) => !v)}
          >
            <Checkbox
              checked={plusOne}
              onCheckedChange={(checked) => setPlusOne(checked === true)}
              className="mt-0.5"
            />
            <div>
              <span className="text-sm">I'd like to bring a plus-one</span>
            </div>
          </div>

          {plusOne && (
            <div className="space-y-4 pl-4 border-l-2 border-border">
              <div>
                <label className="block text-sm text-muted-foreground mb-1.5">Plus-one name</label>
                <Input
                  value={plusOneName}
                  onChange={(e) => setPlusOneName(e.target.value)}
                  placeholder="Their full name"
                />
              </div>
              <div>
                <label className="block text-sm text-muted-foreground mb-1.5">
                  Plus-one dietary requirements (optional)
                </label>
                <Textarea
                  value={plusOneDietary}
                  onChange={(e) => setPlusOneDietary(e.target.value)}
                  placeholder="Any dietary requirements or allergies"
                  rows={2}
                />
              </div>
            </div>
          )}

          {/* Who invited you (generic only) */}
          {info.type === 'generic' && (
            <div>
              <label className="block text-sm text-muted-foreground mb-1.5">Who invited you? (optional)</label>
              <Input
                value={invitedBy}
                onChange={(e) => setInvitedBy(e.target.value)}
                placeholder="Name of the person who invited you"
              />
            </div>
          )}

          {/* Error */}
          {mutation.isError && (
            <p className="text-sm text-destructive">
              {mutation.error instanceof Error ? mutation.error.message : 'Something went wrong'}
            </p>
          )}

          {/* Response buttons */}
          <div className="flex gap-3 pt-2">
            <Button
              className="flex-1"
              onClick={() => handleSubmit('accepted')}
              disabled={mutation.isPending || !name.trim() || !email.trim()}
            >
              {mutation.isPending ? 'Submitting...' : 'Accept'}
            </Button>
            <Button
              variant="outline"
              className="flex-1"
              onClick={() => handleSubmit('declined')}
              disabled={mutation.isPending || !name.trim() || !email.trim()}
            >
              Decline
            </Button>
          </div>
        </div>

        {/* Footer */}
        <div className="mt-12 pt-6 border-t text-center">
          <p className="text-xs text-muted-foreground">The Outlook</p>
        </div>
      </div>
    </div>
  )
}
