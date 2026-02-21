import { useState } from 'react'
import { useMutation } from '@tanstack/react-query'
import { forwardRSVP } from '@/lib/api-public'
import type { RSVPForwardSubmission, PublicTheme } from '@/lib/api-public'
import { Sheet, SheetContent } from '@/components/ui/sheet'
import { Input } from '@/components/ui/input'
import { Checkbox } from '@/components/ui/checkbox'
import { CircleCheck } from 'lucide-react'

interface RSVPForwardDrawerProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  token: string
  eventName: string
  listName: string
  theme?: PublicTheme | null
}

export function RSVPForwardDrawer({ open, onOpenChange, token, eventName, listName, theme }: RSVPForwardDrawerProps) {
  const isDark = theme?.is_dark ?? true

  const inputClassName = isDark
    ? 'bg-white/5 border-[var(--theme-border)]/30 text-[var(--theme-text)] placeholder:text-[var(--theme-text-muted)]/50 focus-visible:ring-[var(--theme-primary)]/40'
    : 'bg-[var(--theme-bg)] border-[var(--theme-border)] text-[var(--theme-text)] placeholder:text-[var(--theme-text-muted)]/50 focus-visible:ring-[var(--theme-primary)]/40'

  const themeStyle = theme ? {
    '--theme-primary': theme.color_primary,
    '--theme-bg': theme.color_background,
    '--theme-surface': theme.color_surface,
    '--theme-text': theme.color_text,
    '--theme-text-muted': theme.color_text_muted,
    '--theme-border': theme.color_border,
  } as React.CSSProperties : {}

  const [forwarderName, setForwarderName] = useState('')
  const [forwarderEmail, setForwarderEmail] = useState('')
  const [forwarderCompany, setForwarderCompany] = useState('')
  const [recipientName, setRecipientName] = useState('')
  const [recipientEmail, setRecipientEmail] = useState('')
  const [recipientCompany, setRecipientCompany] = useState('')
  const [policyAccepted, setPolicyAccepted] = useState(false)
  const [submitted, setSubmitted] = useState(false)

  const mutation = useMutation({
    mutationFn: (data: RSVPForwardSubmission) => forwardRSVP(token, data),
    onSuccess: () => setSubmitted(true),
  })

  const handleSubmit = () => {
    if (!forwarderName.trim() || !forwarderEmail.trim() || !recipientName.trim() || !recipientEmail.trim()) return
    mutation.mutate({
      forwarder_name: forwarderName.trim(),
      forwarder_email: forwarderEmail.trim(),
      forwarder_company: forwarderCompany.trim() || undefined,
      recipient_name: recipientName.trim(),
      recipient_email: recipientEmail.trim(),
      recipient_company: recipientCompany.trim() || undefined,
    })
  }

  const canSubmit = forwarderName.trim() && forwarderEmail.trim() && recipientName.trim() && recipientEmail.trim() && policyAccepted && !mutation.isPending

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent
        showCloseButton={false}
        className="rsvp-forward-drawer bg-[var(--theme-surface)] border-[var(--theme-border)]/30 text-[var(--theme-text)] [--sheet-width:28rem] p-0"
        style={themeStyle}
      >
        {submitted ? (
          <div className="flex-1 flex flex-col items-center justify-center p-8 text-center gap-4">
            <CircleCheck className="h-16 w-16 text-[var(--theme-primary)]" />
            <p className="text-2xl font-[family-name:var(--font-display)]">Invitation sent</p>
            <p className="text-sm text-[var(--theme-text-muted)]">
              We've sent an invitation to {recipientName} at {recipientEmail}. You've been CC'd on the email.
            </p>
            <button
              onClick={() => onOpenChange(false)}
              className="mt-4 text-sm text-[var(--theme-text-muted)] underline underline-offset-4 hover:text-[var(--theme-text)] cursor-pointer"
            >
              Close
            </button>
          </div>
        ) : (
          <div className="flex flex-col h-full">
            {/* Header */}
            <div className="p-6 pb-4 border-b border-[var(--theme-border)]/30">
              <h2 className="text-2xl font-[family-name:var(--font-display)]">Forward this invitation</h2>
              <p className="text-sm text-[var(--theme-text-muted)] mt-1">
                Know someone who'd be a great fit for {eventName || listName}? Feel free to forward this invitation.
              </p>
            </div>

            {/* Form */}
            <div className="flex-1 overflow-y-auto p-6 space-y-8">
              {/* Your details */}
              <div>
                <h3 className="text-2xl text-[var(--theme-text)] font-[family-name:var(--font-display)] mb-4">Your details</h3>
                <div className="space-y-3">
                  <div>
                    <label className="block text-sm text-[var(--theme-text)] mb-1.5">Your name <span className="text-[var(--theme-primary)]">*</span></label>
                    <Input
                      value={forwarderName}
                      onChange={(e) => setForwarderName(e.target.value)}
                      placeholder="Your full name"
                      className={inputClassName}
                    />
                  </div>
                  <div>
                    <label className="block text-sm text-[var(--theme-text)] mb-1.5">Your email <span className="text-[var(--theme-primary)]">*</span></label>
                    <Input
                      type="email"
                      value={forwarderEmail}
                      onChange={(e) => setForwarderEmail(e.target.value)}
                      placeholder="your@email.com"
                      className={inputClassName}
                    />
                  </div>
                  <div>
                    <label className="block text-sm text-[var(--theme-text)] mb-1.5">Your company</label>
                    <Input
                      value={forwarderCompany}
                      onChange={(e) => setForwarderCompany(e.target.value)}
                      placeholder="Company name"
                      className={inputClassName}
                    />
                  </div>
                </div>
              </div>

              {/* Their details */}
              <div>
                <h3 className="text-2xl text-[var(--theme-text)] font-[family-name:var(--font-display)] mb-4">Their details</h3>
                <div className="space-y-3">
                  <div>
                    <label className="block text-sm text-[var(--theme-text)] mb-1.5">Their name <span className="text-[var(--theme-primary)]">*</span></label>
                    <Input
                      value={recipientName}
                      onChange={(e) => setRecipientName(e.target.value)}
                      placeholder="Their full name"
                      className={inputClassName}
                    />
                  </div>
                  <div>
                    <label className="block text-sm text-[var(--theme-text)] mb-1.5">Their email <span className="text-[var(--theme-primary)]">*</span></label>
                    <Input
                      type="email"
                      value={recipientEmail}
                      onChange={(e) => setRecipientEmail(e.target.value)}
                      placeholder="their@email.com"
                      className={inputClassName}
                    />
                  </div>
                  <div>
                    <label className="block text-sm text-[var(--theme-text)] mb-1.5">Their company</label>
                    <Input
                      value={recipientCompany}
                      onChange={(e) => setRecipientCompany(e.target.value)}
                      placeholder="Company name"
                      className={inputClassName}
                    />
                  </div>
                </div>
              </div>

              {/* Privacy policy */}
              <div
                className="flex items-start gap-3 cursor-pointer"
                onClick={() => setPolicyAccepted((v) => !v)}
              >
                <Checkbox
                  checked={policyAccepted}
                  onCheckedChange={(checked) => setPolicyAccepted(checked === true)}
                  className="mt-0.5"
                />
                <span className="text-sm text-[var(--theme-text-muted)] leading-relaxed">
                  I agree to The Outlook&apos;s{' '}
                  <a
                    href="https://theoutlook.io/legal/privacy-policy"
                    target="_blank"
                    rel="noopener noreferrer"
                    className="underline text-[var(--theme-text)] hover:text-[var(--theme-primary)]"
                    onClick={(e) => e.stopPropagation()}
                  >
                    privacy policy
                  </a>
                </span>
              </div>

              {/* Error */}
              {mutation.isError && (
                <p className="text-sm text-[var(--theme-primary)]">
                  {mutation.error instanceof Error ? mutation.error.message : 'Something went wrong'}
                </p>
              )}
            </div>

            {/* Footer */}
            <div className="p-6 pt-4 border-t border-[var(--theme-border)]/30 flex items-center justify-between">
              <button
                onClick={() => onOpenChange(false)}
                className="text-sm text-[var(--theme-text-muted)] hover:text-[var(--theme-text)] cursor-pointer"
              >
                Cancel
              </button>
              <button
                onClick={handleSubmit}
                disabled={!canSubmit}
                className="bg-[var(--theme-primary)] hover:bg-[var(--theme-primary)]/90 disabled:opacity-40 disabled:cursor-not-allowed text-white px-8 h-11 text-sm transition-colors cursor-pointer"
              >
                {mutation.isPending ? 'Sending...' : 'Send invitation'}
              </button>
            </div>
          </div>
        )}
      </SheetContent>
    </Sheet>
  )
}
