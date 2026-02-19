import { useState } from 'react'
import { useMutation } from '@tanstack/react-query'
import { forwardRSVP } from '@/lib/api-public'
import type { RSVPForwardSubmission } from '@/lib/api-public'
import { Sheet, SheetContent } from '@/components/ui/sheet'
import { Input } from '@/components/ui/input'
import { CircleCheck } from 'lucide-react'

const inputClassName = 'bg-white/5 border-[#645C49]/30 text-white placeholder:text-[#A8A9B1]/50 focus-visible:ring-[#E95139]/40'

interface RSVPForwardDrawerProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  token: string
  eventName: string
  listName: string
}

export function RSVPForwardDrawer({ open, onOpenChange, token, eventName, listName }: RSVPForwardDrawerProps) {
  const [forwarderName, setForwarderName] = useState('')
  const [forwarderEmail, setForwarderEmail] = useState('')
  const [forwarderCompany, setForwarderCompany] = useState('')
  const [recipientName, setRecipientName] = useState('')
  const [recipientEmail, setRecipientEmail] = useState('')
  const [recipientCompany, setRecipientCompany] = useState('')
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

  const canSubmit = forwarderName.trim() && forwarderEmail.trim() && recipientName.trim() && recipientEmail.trim() && !mutation.isPending

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent
        showCloseButton={false}
        className="bg-[#1A1917] border-[#645C49]/30 text-white [--sheet-width:28rem] p-0"
      >
        {submitted ? (
          <div className="flex-1 flex flex-col items-center justify-center p-8 text-center gap-4">
            <CircleCheck className="h-16 w-16 text-[#E95139]" />
            <p className="text-2xl font-[family-name:var(--font-display)]">Invitation sent</p>
            <p className="text-sm text-[#A8A9B1]">
              We've sent an invitation to {recipientName} at {recipientEmail}. You've been CC'd on the email.
            </p>
            <button
              onClick={() => onOpenChange(false)}
              className="mt-4 text-sm text-[#A8A9B1] underline underline-offset-4 hover:text-white cursor-pointer"
            >
              Close
            </button>
          </div>
        ) : (
          <div className="flex flex-col h-full">
            {/* Header */}
            <div className="p-6 pb-4 border-b border-[#645C49]/30">
              <h2 className="text-2xl font-[family-name:var(--font-display)]">Forward this invitation</h2>
              <p className="text-sm text-[#A8A9B1] mt-1">
                Know someone who'd be a great fit for {eventName || listName}? Feel free to forward this invitation.
              </p>
            </div>

            {/* Form */}
            <div className="flex-1 overflow-y-auto p-6 space-y-8">
              {/* Your details */}
              <div>
                <p className="eyebrow text-[#A8A9B1] mb-3">Your details</p>
                <div className="space-y-3">
                  <div>
                    <label className="block text-sm text-white mb-1.5">Your name <span className="text-[#E95139]">*</span></label>
                    <Input
                      value={forwarderName}
                      onChange={(e) => setForwarderName(e.target.value)}
                      placeholder="Your full name"
                      className={inputClassName}
                    />
                  </div>
                  <div>
                    <label className="block text-sm text-white mb-1.5">Your email <span className="text-[#E95139]">*</span></label>
                    <Input
                      type="email"
                      value={forwarderEmail}
                      onChange={(e) => setForwarderEmail(e.target.value)}
                      placeholder="your@email.com"
                      className={inputClassName}
                    />
                  </div>
                  <div>
                    <label className="block text-sm text-white mb-1.5">Your company</label>
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
                <p className="eyebrow text-[#A8A9B1] mb-3">Their details</p>
                <div className="space-y-3">
                  <div>
                    <label className="block text-sm text-white mb-1.5">Their name <span className="text-[#E95139]">*</span></label>
                    <Input
                      value={recipientName}
                      onChange={(e) => setRecipientName(e.target.value)}
                      placeholder="Their full name"
                      className={inputClassName}
                    />
                  </div>
                  <div>
                    <label className="block text-sm text-white mb-1.5">Their email <span className="text-[#E95139]">*</span></label>
                    <Input
                      type="email"
                      value={recipientEmail}
                      onChange={(e) => setRecipientEmail(e.target.value)}
                      placeholder="their@email.com"
                      className={inputClassName}
                    />
                  </div>
                  <div>
                    <label className="block text-sm text-white mb-1.5">Their company</label>
                    <Input
                      value={recipientCompany}
                      onChange={(e) => setRecipientCompany(e.target.value)}
                      placeholder="Company name"
                      className={inputClassName}
                    />
                  </div>
                </div>
              </div>

              {/* Error */}
              {mutation.isError && (
                <p className="text-sm text-[#E95139]">
                  {mutation.error instanceof Error ? mutation.error.message : 'Something went wrong'}
                </p>
              )}
            </div>

            {/* Footer */}
            <div className="p-6 pt-4 border-t border-[#645C49]/30 flex items-center justify-between">
              <button
                onClick={() => onOpenChange(false)}
                className="text-sm text-[#A8A9B1] hover:text-white cursor-pointer"
              >
                Cancel
              </button>
              <button
                onClick={handleSubmit}
                disabled={!canSubmit}
                className="bg-[#E95139] hover:bg-[#E95139]/90 disabled:opacity-40 disabled:cursor-not-allowed text-white px-8 h-11 text-sm transition-colors cursor-pointer"
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
