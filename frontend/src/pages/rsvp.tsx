import { useState } from 'react'
import { useParams } from 'react-router'
import { useQuery, useMutation } from '@tanstack/react-query'
import { getRSVPInfo, submitRSVP } from '@/lib/api-public'
import type { RSVPSubmission } from '@/lib/api-public'
import type { DietaryRequirement, AccessibilityRequirement } from '@/lib/pocketbase'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Checkbox } from '@/components/ui/checkbox'
import { Loader2, CircleCheck, XCircle, ChevronDown } from 'lucide-react'

const DIETARY_OPTIONS: { value: DietaryRequirement; label: string }[] = [
  { value: 'vegetarian', label: 'Vegetarian' },
  { value: 'vegan', label: 'Vegan' },
  { value: 'gluten_free', label: 'Gluten free' },
  { value: 'dairy_free', label: 'Dairy free' },
  { value: 'nut_allergy', label: 'Nut allergy' },
  { value: 'halal', label: 'Halal' },
  { value: 'kosher', label: 'Kosher' },
]

const ACCESSIBILITY_OPTIONS: { value: AccessibilityRequirement; label: string }[] = [
  { value: 'wheelchair_access', label: 'Wheelchair access' },
  { value: 'hearing_assistance', label: 'Hearing assistance' },
  { value: 'vision_assistance', label: 'Vision assistance' },
  { value: 'sign_language_interpreter', label: 'Sign language interpreter' },
  { value: 'mobility_assistance', label: 'Mobility assistance' },
]

export function RSVPPage() {
  const { token } = useParams<{ token: string }>()

  const [firstName, setFirstName] = useState('')
  const [lastName, setLastName] = useState('')
  const [email, setEmail] = useState('')
  const [phone, setPhone] = useState('')
  const [dietaryRequirements, setDietaryRequirements] = useState<DietaryRequirement[]>([])
  const [dietaryOther, setDietaryOther] = useState('')
  const [accessibilityRequirements, setAccessibilityRequirements] = useState<AccessibilityRequirement[]>([])
  const [accessibilityOther, setAccessibilityOther] = useState('')
  const [plusOne, setPlusOne] = useState(false)
  const [plusOneName, setPlusOneName] = useState('')
  const [plusOneLastName, setPlusOneLastName] = useState('')
  const [plusOneJobTitle, setPlusOneJobTitle] = useState('')
  const [plusOneCompany, setPlusOneCompany] = useState('')
  const [plusOneEmail, setPlusOneEmail] = useState('')
  const [plusOneDietary, setPlusOneDietary] = useState('')
  const [invitedBy, setInvitedBy] = useState('')
  const [comments, setComments] = useState('')
  const [response, setResponse] = useState<'accepted' | 'declined' | null>(null)
  const [policyAccepted, setPolicyAccepted] = useState(false)
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
    if (info.prefilled_first_name) setFirstName(info.prefilled_first_name)
    if (info.prefilled_last_name) setLastName(info.prefilled_last_name)
    if (info.prefilled_email) setEmail(info.prefilled_email)
    if (info.prefilled_phone) setPhone(info.prefilled_phone)
    if (info.prefilled_dietary_requirements?.length) setDietaryRequirements(info.prefilled_dietary_requirements as DietaryRequirement[])
    if (info.prefilled_dietary_requirements_other) setDietaryOther(info.prefilled_dietary_requirements_other)
    if (info.prefilled_accessibility_requirements?.length) setAccessibilityRequirements(info.prefilled_accessibility_requirements as AccessibilityRequirement[])
    if (info.prefilled_accessibility_requirements_other) setAccessibilityOther(info.prefilled_accessibility_requirements_other)
    if (info.rsvp_status) setResponse(info.rsvp_status === 'accepted' ? 'accepted' : 'declined')
    if (info.rsvp_plus_one) {
      setPlusOne(true)
      if (info.rsvp_plus_one_name) setPlusOneName(info.rsvp_plus_one_name)
      if (info.rsvp_plus_one_last_name) setPlusOneLastName(info.rsvp_plus_one_last_name)
      if (info.rsvp_plus_one_job_title) setPlusOneJobTitle(info.rsvp_plus_one_job_title)
      if (info.rsvp_plus_one_company) setPlusOneCompany(info.rsvp_plus_one_company)
      if (info.rsvp_plus_one_email) setPlusOneEmail(info.rsvp_plus_one_email)
      if (info.rsvp_plus_one_dietary) setPlusOneDietary(info.rsvp_plus_one_dietary)
    }
    if (info.rsvp_comments) setComments(info.rsvp_comments)
    setPrefilled(true)
  }

  const mutation = useMutation({
    mutationFn: (data: RSVPSubmission) => submitRSVP(token!, data),
    onSuccess: (_, variables) => {
      setSubmitted(true)
      setSubmittedResponse(variables.response)
    },
  })

  const displayName = [firstName.trim(), lastName.trim()].filter(Boolean).join(' ')

  const handleSubmit = () => {
    if (!response) return
    mutation.mutate({
      first_name: firstName.trim(),
      last_name: lastName.trim(),
      email: email.trim(),
      phone: phone.trim() || undefined,
      dietary_requirements: dietaryRequirements.length > 0 ? dietaryRequirements : undefined,
      dietary_requirements_other: dietaryOther.trim() || undefined,
      accessibility_requirements: accessibilityRequirements.length > 0 ? accessibilityRequirements : undefined,
      accessibility_requirements_other: accessibilityOther.trim() || undefined,
      plus_one: plusOne,
      plus_one_name: plusOne ? plusOneName.trim() : undefined,
      plus_one_last_name: plusOne ? plusOneLastName.trim() : undefined,
      plus_one_job_title: plusOne ? plusOneJobTitle.trim() : undefined,
      plus_one_company: plusOne ? plusOneCompany.trim() : undefined,
      plus_one_email: plusOne ? plusOneEmail.trim() : undefined,
      plus_one_dietary: plusOne ? plusOneDietary.trim() : undefined,
      response,
      invited_by: info?.type === 'generic' ? invitedBy.trim() || undefined : undefined,
      comments: comments.trim() || undefined,
    })
  }

  const toggleDietary = (value: DietaryRequirement) => {
    setDietaryRequirements((prev) =>
      prev.includes(value) ? prev.filter((v) => v !== value) : [...prev, value]
    )
  }

  const toggleAccessibility = (value: AccessibilityRequirement) => {
    setAccessibilityRequirements((prev) =>
      prev.includes(value) ? prev.filter((v) => v !== value) : [...prev, value]
    )
  }

  const errorMessage = error instanceof Error ? error.message : error ? String(error) : ''

  if (isLoading) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-brand-black">
        <Loader2 className="h-6 w-6 animate-spin text-white/40" />
      </div>
    )
  }

  if (error) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-brand-black p-4">
        <div className="text-center space-y-3 max-w-md">
          <img src="/images/logo-white.svg" alt="The Outlook" className="h-8 mx-auto mb-6" />
          <p className="text-lg text-white">RSVP not available</p>
          <p className="text-sm text-white/60">{errorMessage}</p>
        </div>
      </div>
    )
  }

  if (!info) return null

  // Success screen
  if (submitted) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-brand-black p-4">
        <div className="max-w-md w-full space-y-6 text-center">
          <img src="/images/logo-white.svg" alt="The Outlook" className="h-8 mx-auto mb-6" />
          {submittedResponse === 'accepted' ? (
            <CircleCheck className="h-16 w-16 text-green-600 mx-auto" />
          ) : (
            <XCircle className="h-16 w-16 text-white/40 mx-auto" />
          )}
          <div className="space-y-2">
            <p className="text-xl text-white">
              {submittedResponse === 'accepted' ? "You're confirmed" : 'Response received'}
            </p>
            <p className="text-sm text-white/60">
              {submittedResponse === 'accepted'
                ? `Thanks ${displayName}, we look forward to seeing you at ${info.event_name || info.list_name}.`
                : `Thanks for letting us know, ${displayName}.`}
            </p>
          </div>
        </div>
      </div>
    )
  }

  // Already responded — show status with option to update
  const alreadyResponded = info.already_responded && info.rsvp_status

  return (
    <div className="min-h-screen bg-brand-black">
      <div className="max-w-2xl mx-auto px-4 py-12">
        {/* Logo */}
        <img src="/images/logo-white.svg" alt="The Outlook" className="h-8 mx-auto mb-8" />

        {/* Header */}
        <div className="text-center space-y-2 mb-8">
          {info.event_name ? (
            <>
              <p className="text-xl text-white">{info.event_name}</p>
              <p className="text-sm text-white/60">{info.list_name}</p>
            </>
          ) : (
            <p className="text-xl text-white">{info.list_name}</p>
          )}
          {info.description && (
            <p className="text-sm text-white/60">{info.description}</p>
          )}
          {alreadyResponded && (
            <p className="text-sm text-white/60">
              You previously {info.rsvp_status === 'accepted' ? 'accepted' : 'declined'} this invitation. You can update your response below.
            </p>
          )}
        </div>

        {/* Form */}
        <div className="rounded-lg bg-white p-6 space-y-6">
          {/* Response selection */}
          <div className="grid grid-cols-2 gap-3">
            <button
              type="button"
              onClick={() => setResponse('accepted')}
              className={`rounded-lg border-2 p-4 text-center transition-colors cursor-pointer ${
                response === 'accepted'
                  ? 'border-green-600 bg-green-50'
                  : 'border-gray-200 hover:border-gray-400'
              }`}
            >
              <CircleCheck className={`h-6 w-6 mx-auto mb-1.5 ${response === 'accepted' ? 'text-green-600' : 'text-gray-400'}`} />
              <span className={`text-sm ${response === 'accepted' ? 'text-green-600' : 'text-gray-900'}`}>Can make it</span>
            </button>
            <button
              type="button"
              onClick={() => setResponse('declined')}
              className={`rounded-lg border-2 p-4 text-center transition-colors cursor-pointer ${
                response === 'declined'
                  ? 'border-gray-900 bg-gray-50'
                  : 'border-gray-200 hover:border-gray-400'
              }`}
            >
              <XCircle className={`h-6 w-6 mx-auto mb-1.5 ${response === 'declined' ? 'text-gray-900' : 'text-gray-400'}`} />
              <span className="text-sm text-gray-900">Sorry, can't make it</span>
            </button>
          </div>

          <hr className="border-gray-200" />

          {/* Your details */}
          <div className="rounded-lg border border-gray-200 p-5">
            <h3 className="text-base text-gray-900 mb-4">Your details</h3>
            <div className="space-y-4">
              <div className="grid grid-cols-2 gap-3">
                <div>
                  <label className="block text-sm text-gray-900 mb-1.5">First name <span className="text-red-500">*</span></label>
                  <Input
                    value={firstName}
                    onChange={(e) => setFirstName(e.target.value)}
                    placeholder="First name"
                    required
                  />
                </div>
                <div>
                  <label className="block text-sm text-gray-900 mb-1.5">Last name</label>
                  <Input
                    value={lastName}
                    onChange={(e) => setLastName(e.target.value)}
                    placeholder="Last name"
                  />
                </div>
              </div>

              <div>
                <label className="block text-sm text-gray-900 mb-1.5">Email <span className="text-red-500">*</span></label>
                <Input
                  type="email"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  placeholder="your@email.com"
                  required
                />
              </div>

              {response !== 'declined' && (
                <div>
                  <label className="block text-sm text-gray-900 mb-1.5">Phone</label>
                  <Input
                    type="tel"
                    value={phone}
                    onChange={(e) => setPhone(e.target.value)}
                    placeholder="Your phone number"
                  />
                </div>
              )}
            </div>
          </div>

          {/* Additional requirements — only when accepting */}
          {response === 'accepted' && (
            <div className="rounded-lg border border-gray-200 p-5">
              <h3 className="text-base text-gray-900 mb-4">Additional requirements</h3>
              <div className="space-y-4">
                {/* Dietary requirements */}
                <details className="group rounded-lg border border-gray-200">
                  <summary className="flex items-center justify-between cursor-pointer p-4 list-none">
                    <span className="text-sm text-gray-900">Dietary requirements {dietaryRequirements.length > 0 || dietaryOther ? `(${dietaryRequirements.length + (dietaryOther ? 1 : 0)})` : ''}</span>
                    <ChevronDown className="h-4 w-4 text-gray-400 transition-transform group-open:rotate-180" />
                  </summary>
                  <div className="px-4 pb-4 space-y-2">
                    <div className="flex flex-wrap gap-x-4 gap-y-2">
                      {DIETARY_OPTIONS.map(({ value, label }) => (
                        <label key={value} className="flex items-center gap-1.5 cursor-pointer">
                          <Checkbox
                            checked={dietaryRequirements.includes(value)}
                            onCheckedChange={() => toggleDietary(value)}
                          />
                          <span className="text-sm">{label}</span>
                        </label>
                      ))}
                    </div>
                    <Input
                      placeholder="Other dietary requirements"
                      value={dietaryOther}
                      onChange={(e) => setDietaryOther(e.target.value)}
                    />
                  </div>
                </details>

                {/* Accessibility requirements */}
                <details className="group rounded-lg border border-gray-200">
                  <summary className="flex items-center justify-between cursor-pointer p-4 list-none">
                    <span className="text-sm text-gray-900">Accessibility requirements {accessibilityRequirements.length > 0 || accessibilityOther ? `(${accessibilityRequirements.length + (accessibilityOther ? 1 : 0)})` : ''}</span>
                    <ChevronDown className="h-4 w-4 text-gray-400 transition-transform group-open:rotate-180" />
                  </summary>
                  <div className="px-4 pb-4 space-y-2">
                    <div className="flex flex-wrap gap-x-4 gap-y-2">
                      {ACCESSIBILITY_OPTIONS.map(({ value, label }) => (
                        <label key={value} className="flex items-center gap-1.5 cursor-pointer">
                          <Checkbox
                            checked={accessibilityRequirements.includes(value)}
                            onCheckedChange={() => toggleAccessibility(value)}
                          />
                          <span className="text-sm">{label}</span>
                        </label>
                      ))}
                    </div>
                    <Input
                      placeholder="Other accessibility requirements"
                      value={accessibilityOther}
                      onChange={(e) => setAccessibilityOther(e.target.value)}
                    />
                  </div>
                </details>
              </div>
            </div>
          )}

          {/* Invitation — plus one + who invited you, only when accepting */}
          {response === 'accepted' && (
            <div className="rounded-lg border border-gray-200 p-5">
              <h3 className="text-base text-gray-900 mb-4">Invitation</h3>
              <div className="space-y-4">
                {/* Who invited you (generic only) */}
                {info.type === 'generic' && (
                  <div>
                    <label className="block text-sm text-gray-900 mb-1.5">Who invited you?</label>
                    <Input
                      value={invitedBy}
                      onChange={(e) => setInvitedBy(e.target.value)}
                      placeholder="Name of the person who invited you"
                    />
                  </div>
                )}

                {/* Plus one */}
                <div
                  className="flex items-start gap-3 cursor-pointer rounded-lg border border-gray-200 p-4"
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
                  <div className="space-y-4 pl-4 border-l-2 border-gray-200">
                    <div className="grid grid-cols-2 gap-3">
                      <div>
                        <label className="block text-sm text-gray-900 mb-1.5">First name</label>
                        <Input
                          value={plusOneName}
                          onChange={(e) => setPlusOneName(e.target.value)}
                          placeholder="First name"
                        />
                      </div>
                      <div>
                        <label className="block text-sm text-gray-900 mb-1.5">Last name</label>
                        <Input
                          value={plusOneLastName}
                          onChange={(e) => setPlusOneLastName(e.target.value)}
                          placeholder="Last name"
                        />
                      </div>
                    </div>
                    <div>
                      <label className="block text-sm text-gray-900 mb-1.5">Job title</label>
                      <Input
                        value={plusOneJobTitle}
                        onChange={(e) => setPlusOneJobTitle(e.target.value)}
                        placeholder="Job title"
                      />
                    </div>
                    <div>
                      <label className="block text-sm text-gray-900 mb-1.5">Company</label>
                      <Input
                        value={plusOneCompany}
                        onChange={(e) => setPlusOneCompany(e.target.value)}
                        placeholder="Company"
                      />
                    </div>
                    <div>
                      <label className="block text-sm text-gray-900 mb-1.5">Email</label>
                      <Input
                        type="email"
                        value={plusOneEmail}
                        onChange={(e) => setPlusOneEmail(e.target.value)}
                        placeholder="their@email.com"
                      />
                    </div>
                    <div>
                      <label className="block text-sm text-gray-900 mb-1.5">Dietary requirements</label>
                      <Textarea
                        value={plusOneDietary}
                        onChange={(e) => setPlusOneDietary(e.target.value)}
                        placeholder="Any dietary requirements or allergies"
                        rows={2}
                      />
                    </div>
                  </div>
                )}
              </div>
            </div>
          )}

          {/* Comments */}
          <div>
            <label className="block text-sm text-gray-900 mb-1.5">Comments</label>
            <Textarea
              value={comments}
              onChange={(e) => setComments(e.target.value)}
              placeholder="Anything else you'd like us to know"
              rows={3}
            />
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
            <div className="text-sm leading-relaxed">
              <span className="text-gray-900">
                I agree to The Outlook&apos;s{' '}
                <a
                  href="https://theoutlook.io/legal/privacy-policy"
                  target="_blank"
                  rel="noopener noreferrer"
                  className="underline"
                  onClick={(e) => e.stopPropagation()}
                >
                  privacy policy
                </a>
              </span>
              <p className="text-gray-900 mt-1">
                Your details may only be shared with the event partner and no one else. All personal information is encrypted and access is strictly limited.
              </p>
            </div>
          </div>

          {/* Error */}
          {mutation.isError && (
            <p className="text-sm text-destructive">
              {mutation.error instanceof Error ? mutation.error.message : 'Something went wrong'}
            </p>
          )}

          {/* Submit button */}
          <Button
            onClick={handleSubmit}
            disabled={mutation.isPending || !policyAccepted || !response || !firstName.trim() || !email.trim()}
          >
            {mutation.isPending ? 'Submitting...' : 'Submit'}
          </Button>
        </div>
      </div>
    </div>
  )
}
