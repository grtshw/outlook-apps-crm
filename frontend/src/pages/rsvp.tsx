import { useState, useRef, useEffect, useMemo } from 'react'
import { useParams, useLocation } from 'react-router'
import { useQuery, useMutation } from '@tanstack/react-query'
import { getRSVPInfo, submitRSVP } from '@/lib/api-public'
import type { RSVPSubmission, PublicTheme } from '@/lib/api-public'
import { RSVPForwardDrawer } from '@/components/rsvp-forward-drawer'
import type { DietaryRequirement, AccessibilityRequirement } from '@/lib/pocketbase'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Checkbox } from '@/components/ui/checkbox'
import { Loader2, CircleCheck, XCircle, ChevronDown } from 'lucide-react'
import gsap from 'gsap'

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

/** Parse a hex color string like "#E95139" to [r, g, b] */
function hexToRgb(hex: string): [number, number, number] {
  const h = hex.replace('#', '')
  return [parseInt(h.slice(0, 2), 16), parseInt(h.slice(2, 4), 16), parseInt(h.slice(4, 6), 16)]
}

/** Build CSS custom properties from theme API data */
function buildThemeStyle(theme: PublicTheme | null): React.CSSProperties {
  if (!theme) return {}
  return {
    '--theme-primary': theme.color_primary,
    '--theme-bg': theme.color_background,
    '--theme-surface': theme.color_surface,
    '--theme-text': theme.color_text,
    '--theme-text-muted': theme.color_text_muted,
    '--theme-border': theme.color_border,
  } as React.CSSProperties
}

export function RSVPPage() {
  const { token } = useParams<{ token: string }>()
  const location = useLocation()
  const [forwardOpen, setForwardOpen] = useState(location.pathname.endsWith('/forward'))

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
  const [response, setResponse] = useState<'accepted' | 'declined' | null>('accepted')
  const [policyAccepted, setPolicyAccepted] = useState(false)
  const [prefilled, setPrefilled] = useState(false)
  const [submitted, setSubmitted] = useState(false)
  const [submittedResponse, setSubmittedResponse] = useState<'accepted' | 'declined' | ''>('')
  const [expandedProgram, setExpandedProgram] = useState<number | null>(null)
  const [mobileProgramOpen, setMobileProgramOpen] = useState(false)
  const [mobileRsvpOpen, setMobileRsvpOpen] = useState(true)

  const { data: info, error, isLoading } = useQuery({
    queryKey: ['rsvp-info', token],
    queryFn: () => getRSVPInfo(token!),
    enabled: !!token,
    retry: false,
  })

  // Derive theme config
  const theme = info?.theme ?? null
  const isDark = theme?.is_dark ?? true
  const themeStyle = useMemo(() => buildThemeStyle(theme), [theme])
  const logoSrc = isDark ? (theme?.logo_light_url || '/images/logo-white.svg') : (theme?.logo_url || '/images/logo.svg')
  const heroImage = info?.landing_image_url || theme?.hero_image_url || '/images/rsvp-hero.jpg'
  const brandName = theme?.name || 'The Outlook After Dark'

  const inputClassName = isDark
    ? 'bg-white/5 border-[var(--theme-border)]/30 text-[var(--theme-text)] placeholder:text-[var(--theme-text-muted)]/50 focus-visible:ring-[var(--theme-primary)]/40'
    : 'bg-[var(--theme-bg)] border-[var(--theme-border)] text-[var(--theme-text)] placeholder:text-[var(--theme-text-muted)]/50 focus-visible:ring-[var(--theme-primary)]/40'
  const textareaClassName = inputClassName

  const heroPaneRef = useRef<HTMLDivElement>(null)
  const formPaneRef = useRef<HTMLDivElement>(null)
  const heroImageRef = useRef<HTMLDivElement>(null)
  const heroContentRef = useRef<HTMLDivElement>(null)
  const formInnerRef = useRef<HTMLDivElement>(null)
  const heroAnimated = useRef(false)

  // Detect scroll: transition hero bg from primary to background + parallax
  useEffect(() => {
    const primaryColor = theme?.color_primary ?? '#E95139'
    const bgColor = theme?.color_background ?? '#020202'
    const [pr, pg, pb] = hexToRgb(primaryColor)
    const [br, bg, bb] = hexToRgb(bgColor)

    let ticking = false
    const onScroll = () => {
      if (ticking) return
      ticking = true
      requestAnimationFrame(() => {
        ticking = false
        if (!heroPaneRef.current) return
        const progress = Math.min(Math.max(window.scrollY / window.innerHeight, 0), 1)

        const r = Math.round(pr + (br - pr) * progress)
        const g = Math.round(pg + (bg - pg) * progress)
        const b = Math.round(pb + (bb - pb) * progress)
        heroPaneRef.current.style.backgroundColor = `rgb(${r},${g},${b})`

        // Parallax: translate hero content upward as user scrolls
        const parallaxY = window.scrollY * 0.15
        if (heroContentRef.current) heroContentRef.current.style.transform = `translateY(-${parallaxY}px)`
        if (heroImageRef.current) heroImageRef.current.style.transform = `translateY(-${parallaxY * 0.5}px)`
      })
    }
    window.addEventListener('scroll', onScroll, { passive: true })
    return () => window.removeEventListener('scroll', onScroll)
  }, [theme])

  // Hero entrance animation — staggered fade-in on load
  useEffect(() => {
    if (!info || heroAnimated.current) return
    heroAnimated.current = true

    if (heroImageRef.current) {
      gsap.fromTo(heroImageRef.current,
        { opacity: 0, scale: 1.04 },
        { opacity: 1, scale: 1, duration: 1.4, ease: 'power2.out' }
      )
    }

    if (heroContentRef.current) {
      const children = heroContentRef.current.children
      gsap.fromTo(children,
        { opacity: 0, y: 24 },
        { opacity: 1, y: 0, duration: 0.8, ease: 'power2.out', stagger: 0.15, delay: 0.3 }
      )
    }
  }, [info])

  // Slow Ken Burns zoom on hero image
  useEffect(() => {
    if (!heroImageRef.current) return
    const img = heroImageRef.current.querySelector('img')
    if (!img) return

    gsap.fromTo(img,
      { scale: 1 },
      { scale: 1.08, duration: 20, ease: 'none', repeat: -1, yoyo: true }
    )
  }, [])

  // Form pane entrance — fade up when scrolled into view
  useEffect(() => {
    if (!formInnerRef.current) return
    const el = formInnerRef.current

    gsap.set(el.children, { opacity: 0, y: 30 })

    const observer = new IntersectionObserver(
      ([entry]) => {
        if (entry.isIntersecting) {
          gsap.to(el.children, {
            opacity: 1,
            y: 0,
            duration: 0.7,
            ease: 'power2.out',
            stagger: 0.1,
          })
          observer.disconnect()
        }
      },
      { threshold: 0.1 }
    )
    observer.observe(el)
    return () => observer.disconnect()
  }, [])

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
    if (plusOne && (!plusOneName.trim() || !plusOneEmail.trim())) return
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
      <div className="rsvp-theme min-h-screen flex items-center justify-center bg-[var(--theme-bg)]" style={themeStyle}>
        <Loader2 className="h-6 w-6 animate-spin text-[var(--theme-text-muted)]/40" />
      </div>
    )
  }

  if (error) {
    return (
      <div className="rsvp-theme min-h-screen flex items-center justify-center bg-[var(--theme-bg)] p-4" style={themeStyle}>
        <div className="text-center space-y-3 max-w-md">
          <img src={logoSrc} alt="The Outlook" className="h-8 mx-auto mb-6" />
          <p className="text-lg text-[var(--theme-text)] font-[family-name:var(--font-display)]">RSVP not available</p>
          <p className="text-sm text-[var(--theme-text-muted)]">{errorMessage}</p>
        </div>
      </div>
    )
  }

  if (!info) return null

  const alreadyResponded = info.already_responded && info.rsvp_status

  const successContent = (
    <div className="max-w-md w-full space-y-6 text-center mx-auto">
      {submittedResponse === 'accepted' ? (
        <CircleCheck className="h-16 w-16 text-[var(--theme-primary)] mx-auto" />
      ) : (
        <XCircle className="h-16 w-16 text-[var(--theme-text-muted)]/40 mx-auto" />
      )}
      <div className="space-y-2">
        <p className="text-2xl text-[var(--theme-text)] font-[family-name:var(--font-display)]">
          {submittedResponse === 'accepted' ? "You're confirmed" : 'Response received'}
        </p>
        <p className="text-sm text-[var(--theme-text-muted)]">
          {submittedResponse === 'accepted'
            ? `Thanks ${firstName.trim() || 'so much'}, we look forward to seeing you at ${info.event_name || info.list_name}.`
            : `Thanks for letting us know, ${firstName.trim() || displayName}.`}
        </p>
      </div>
    </div>
  )

  const formContent = (
    <>
      {alreadyResponded && (
        <div className="text-center mb-8">
          <p className="text-sm text-[var(--theme-text-muted)]">
            You previously {info.rsvp_status === 'accepted' ? 'accepted' : 'declined'} this invitation. You can update your response below.
          </p>
        </div>
      )}
      <h2 className="hidden lg:block text-4xl lg:text-5xl text-[var(--theme-text)] font-[family-name:var(--font-display)] leading-[1.1] mb-8">RSVP</h2>

      {/* Response selection */}
      <div className="grid grid-cols-2 gap-3 mb-6">
        <label
          className={`flex items-center gap-3 p-4 cursor-pointer transition-colors border border-[var(--theme-border)]/30 ${
            response === 'accepted' ? 'bg-[var(--theme-primary)] border-[var(--theme-primary)]' : 'hover:border-[var(--theme-border)]'
          }`}
        >
          <input
            type="radio"
            name="rsvp-response"
            checked={response === 'accepted'}
            onChange={() => setResponse('accepted')}
            className="sr-only"
          />
          <div className={`w-4 h-4 rounded-full border-2 flex items-center justify-center shrink-0 ${
            response === 'accepted' ? 'border-[var(--theme-text-muted)]' : 'border-[var(--theme-text-muted)]/40'
          }`}>
            {response === 'accepted' && <div className="w-2 h-2 rounded-full bg-[var(--theme-text-muted)]" />}
          </div>
          <span className="text-sm text-[var(--theme-text)]">I can make it</span>
        </label>
        <label
          className={`flex items-center gap-3 p-4 cursor-pointer transition-colors border border-[var(--theme-border)]/30 ${
            response === 'declined' ? 'bg-[var(--theme-bg)]/40 border-[var(--theme-text-muted)]/60' : 'hover:border-[var(--theme-border)]'
          }`}
        >
          <input
            type="radio"
            name="rsvp-response"
            checked={response === 'declined'}
            onChange={() => setResponse('declined')}
            className="sr-only"
          />
          <div className={`w-4 h-4 rounded-full border-2 flex items-center justify-center shrink-0 ${
            response === 'declined' ? 'border-[var(--theme-text-muted)]' : 'border-[var(--theme-text-muted)]/40'
          }`}>
            {response === 'declined' && <div className="w-2 h-2 rounded-full bg-[var(--theme-text-muted)]" />}
          </div>
          <span className="text-sm text-[var(--theme-text)]">I can't make it</span>
        </label>
      </div>

      {response && (
      <div className="space-y-6 pb-16">

          {/* Your details */}
          <div className="border-t border-[var(--theme-border)]/30 pt-10 mt-4">
            <h3 className="text-2xl text-[var(--theme-text)] font-[family-name:var(--font-display)] mb-4">Your details</h3>
            <div className="space-y-4">
              <div className="grid grid-cols-2 gap-3">
                <div>
                  <label className="block text-sm text-[var(--theme-text)] mb-1.5">First name <span className="text-[var(--theme-primary)]">*</span></label>
                  <Input
                    value={firstName}
                    onChange={(e) => setFirstName(e.target.value)}
                    placeholder="First name"
                    required
                    className={inputClassName}
                  />
                </div>
                <div>
                  <label className="block text-sm text-[var(--theme-text)] mb-1.5">Last name</label>
                  <Input
                    value={lastName}
                    onChange={(e) => setLastName(e.target.value)}
                    placeholder="Last name"
                    className={inputClassName}
                  />
                </div>
              </div>

              <div>
                <label className="block text-sm text-[var(--theme-text)] mb-1.5">Email <span className="text-[var(--theme-primary)]">*</span></label>
                <Input
                  type="email"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  placeholder="your@email.com"
                  required
                  className={inputClassName}
                />
              </div>

              {response !== 'declined' && (
                <div>
                  <label className="block text-sm text-[var(--theme-text)] mb-1.5">Phone</label>
                  <Input
                    type="tel"
                    value={phone}
                    onChange={(e) => setPhone(e.target.value)}
                    placeholder="Your phone number"
                    className={inputClassName}
                  />
                </div>
              )}
            </div>
          </div>

          {/* Additional requirements — only when accepting */}
          {response === 'accepted' && (
            <details className="group border-t border-[var(--theme-border)]/30 pt-6 mt-10">
              <summary className="flex items-center justify-between cursor-pointer list-none">
                <h3 className="text-2xl text-[var(--theme-text)] font-[family-name:var(--font-display)]">Additional requirements</h3>
                <ChevronDown className="h-5 w-5 text-[var(--theme-text-muted)]/40 transition-transform group-open:rotate-180" />
              </summary>
              <div className="space-y-3 mt-4">
                {/* Dietary requirements */}
                <details className="group rounded-lg border border-[var(--theme-border)]/20">
                  <summary className="flex items-center justify-between cursor-pointer p-4 list-none">
                    <span className="text-sm text-[var(--theme-text)]">Dietary requirements {dietaryRequirements.length > 0 || dietaryOther ? `(${dietaryRequirements.length + (dietaryOther ? 1 : 0)})` : ''}</span>
                    <ChevronDown className="h-4 w-4 text-[var(--theme-text-muted)]/40 transition-transform group-open:rotate-180" />
                  </summary>
                  <div className="px-4 pb-4 space-y-2">
                    <div className="flex flex-wrap gap-x-4 gap-y-2">
                      {DIETARY_OPTIONS.map(({ value, label }) => (
                        <label key={value} className="flex items-center gap-1.5 cursor-pointer">
                          <Checkbox
                            checked={dietaryRequirements.includes(value)}
                            onCheckedChange={() => toggleDietary(value)}
                          />
                          <span className="text-sm text-[var(--theme-text)]">{label}</span>
                        </label>
                      ))}
                    </div>
                    <Input
                      placeholder="Other dietary requirements"
                      value={dietaryOther}
                      onChange={(e) => setDietaryOther(e.target.value)}
                      className={inputClassName}
                    />
                  </div>
                </details>

                {/* Accessibility requirements */}
                <details className="group rounded-lg border border-[var(--theme-border)]/20">
                  <summary className="flex items-center justify-between cursor-pointer p-4 list-none">
                    <span className="text-sm text-[var(--theme-text)]">Accessibility requirements {accessibilityRequirements.length > 0 || accessibilityOther ? `(${accessibilityRequirements.length + (accessibilityOther ? 1 : 0)})` : ''}</span>
                    <ChevronDown className="h-4 w-4 text-[var(--theme-text-muted)]/40 transition-transform group-open:rotate-180" />
                  </summary>
                  <div className="px-4 pb-4 space-y-2">
                    <div className="flex flex-wrap gap-x-4 gap-y-2">
                      {ACCESSIBILITY_OPTIONS.map(({ value, label }) => (
                        <label key={value} className="flex items-center gap-1.5 cursor-pointer">
                          <Checkbox
                            checked={accessibilityRequirements.includes(value)}
                            onCheckedChange={() => toggleAccessibility(value)}
                          />
                          <span className="text-sm text-[var(--theme-text)]">{label}</span>
                        </label>
                      ))}
                    </div>
                    <Input
                      placeholder="Other accessibility requirements"
                      value={accessibilityOther}
                      onChange={(e) => setAccessibilityOther(e.target.value)}
                      className={inputClassName}
                    />
                  </div>
                </details>
              </div>
            </details>
          )}

          {/* Invitation — plus one + who invited you */}
          {response === 'accepted' && (
            <div className="border-t border-[var(--theme-border)]/30 pt-10 mt-10">
              <h3 className="text-2xl text-[var(--theme-text)] font-[family-name:var(--font-display)] mb-4">Invitation</h3>
              <div className="space-y-4">
                {/* Plus one */}
                <div
                  className="flex items-start gap-3 cursor-pointer rounded-lg border border-[var(--theme-border)]/20 p-4"
                  onClick={() => setPlusOne((v) => !v)}
                >
                  <Checkbox
                    checked={plusOne}
                    onCheckedChange={(checked) => setPlusOne(checked === true)}
                    className="mt-0.5"
                  />
                  <div>
                    <span className="text-sm text-[var(--theme-text)]">I'd like to bring a plus-one</span>
                    <p className="text-xs text-[var(--theme-text-muted)] mt-1">We'll review each plus-one request and share an invite if we can squeeze them in.</p>
                  </div>
                </div>

                {plusOne && (
                  <div className="space-y-4 pl-4 border-l-2 border-[var(--theme-border)]/30">
                    <div className="grid grid-cols-2 gap-3">
                      <div>
                        <label className="block text-sm text-[var(--theme-text)] mb-1.5">First name <span className="text-[var(--theme-primary)]">*</span></label>
                        <Input
                          value={plusOneName}
                          onChange={(e) => setPlusOneName(e.target.value)}
                          placeholder="First name"
                          required
                          className={inputClassName}
                        />
                      </div>
                      <div>
                        <label className="block text-sm text-[var(--theme-text)] mb-1.5">Last name</label>
                        <Input
                          value={plusOneLastName}
                          onChange={(e) => setPlusOneLastName(e.target.value)}
                          placeholder="Last name"
                          className={inputClassName}
                        />
                      </div>
                    </div>
                    <div>
                      <label className="block text-sm text-[var(--theme-text)] mb-1.5">Job title</label>
                      <Input
                        value={plusOneJobTitle}
                        onChange={(e) => setPlusOneJobTitle(e.target.value)}
                        placeholder="Job title"
                        className={inputClassName}
                      />
                    </div>
                    <div>
                      <label className="block text-sm text-[var(--theme-text)] mb-1.5">Company</label>
                      <Input
                        value={plusOneCompany}
                        onChange={(e) => setPlusOneCompany(e.target.value)}
                        placeholder="Company"
                        className={inputClassName}
                      />
                    </div>
                    <div>
                      <label className="block text-sm text-[var(--theme-text)] mb-1.5">Email <span className="text-[var(--theme-primary)]">*</span></label>
                      <Input
                        type="email"
                        value={plusOneEmail}
                        onChange={(e) => setPlusOneEmail(e.target.value)}
                        placeholder="their@email.com"
                        required
                        className={inputClassName}
                      />
                    </div>
                    <div>
                      <label className="block text-sm text-[var(--theme-text)] mb-1.5">Dietary requirements</label>
                      <Textarea
                        value={plusOneDietary}
                        onChange={(e) => setPlusOneDietary(e.target.value)}
                        placeholder="Any dietary requirements or allergies"
                        rows={2}
                        className={textareaClassName}
                      />
                    </div>
                  </div>
                )}

                {/* Who invited you — generic invites only */}
                {info.type === 'generic' && (
                  <div>
                    <label className="block text-sm text-[var(--theme-text)] mb-1.5">Who invited you?</label>
                    <Input
                      value={invitedBy}
                      onChange={(e) => setInvitedBy(e.target.value)}
                      placeholder="Name of the person who invited you"
                      className={inputClassName}
                    />
                  </div>
                )}
              </div>
            </div>
          )}

          {/* Anything else? — comments */}
          <div className="border-t border-[var(--theme-border)]/30 pt-10 mt-10">
            <h3 className="text-2xl text-[var(--theme-text)] font-[family-name:var(--font-display)] mb-4">Anything else?</h3>
            <div className="space-y-4">
              <div>
                <label className="block text-sm text-[var(--theme-text)] mb-1.5">Comments</label>
                <Textarea
                  value={comments}
                  onChange={(e) => setComments(e.target.value)}
                  placeholder="Anything else you'd like us to know"
                  rows={3}
                  className={textareaClassName}
                />
              </div>
            </div>
          </div>

          {/* Privacy policy */}
          <div
            className="flex items-start gap-3 cursor-pointer border-t border-[var(--theme-border)]/30 pt-6 mt-6"
            onClick={() => setPolicyAccepted((v) => !v)}
          >
            <Checkbox
              checked={policyAccepted}
              onCheckedChange={(checked) => setPolicyAccepted(checked === true)}
              className="mt-0.5"
            />
            <div className="text-sm leading-relaxed">
              <span className="text-[var(--theme-text-muted)]">
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
          </div>

          {/* Error */}
          {mutation.isError && (
            <p className="text-sm text-[var(--theme-primary)]">
              {mutation.error instanceof Error ? mutation.error.message : 'Something went wrong'}
            </p>
          )}

          {/* Submit button */}
          <button
            onClick={handleSubmit}
            disabled={mutation.isPending || !policyAccepted || !response || !firstName.trim() || !email.trim()}
            className="bg-[var(--theme-primary)] hover:bg-[var(--theme-primary)]/90 disabled:opacity-40 disabled:cursor-not-allowed text-white px-12 h-12 text-base transition-colors cursor-pointer"
          >
            {mutation.isPending ? 'Submitting...' : 'Submit'}
          </button>
        </div>
      )}
    </>
  )

  // Editorial hero + form layout
  return (
    <div className="rsvp-theme bg-[var(--theme-bg)]" style={themeStyle}>
      {/* Pane 1: Full-viewport magazine spread — sticky for card-over effect */}
      <div ref={heroPaneRef} className="h-screen sticky top-0 bg-[var(--theme-primary)] p-6 lg:p-10 flex flex-col">
        {/* Main spread */}
        <div className="flex-1 flex flex-col lg:flex-row gap-6 lg:gap-10 min-h-0">
          <div ref={heroImageRef} className="h-[35vh] lg:h-auto lg:flex-[2] min-w-0 overflow-hidden shrink-0 relative">
            <img
              src={heroImage}
              alt=""
              className="absolute inset-0 w-full h-full object-cover"
            />
            <span className="absolute top-8 left-8 bg-black/20 text-white/60 text-xs font-mono tracking-wider uppercase z-10 px-3 py-1.5">{info.prefilled_first_name ? `${info.prefilled_first_name}, you're invited` : "You're invited"}</span>
          </div>
          <div ref={heroContentRef} className="flex-1 lg:flex-[1] min-w-0 flex flex-col gap-5 items-center text-center lg:px-10">
            <div className="pt-10 lg:pt-8">
              <img src={logoSrc} alt="The Outlook" className="h-8 opacity-80 mx-auto mb-10 lg:mb-14" />
              {info.organisation_logo_url && (
                <img
                  src={info.organisation_logo_url}
                  alt={info.organisation_name || ''}
                  className="h-8 mb-4 object-contain mx-auto"
                />
              )}
              <h1 className="text-3xl lg:text-5xl xl:text-6xl text-[var(--theme-text)] font-[family-name:var(--font-display)] leading-[1]">
                {info.list_name}
              </h1>
            </div>
            <div className="text-[var(--theme-text)]/80 text-lg lg:text-xl leading-relaxed">
              {info.description ? (
                <p>{info.description}</p>
              ) : (
                <p>An intimate evening of conversation, connection and great food.</p>
              )}
            </div>
            {(info.event_date || info.event_time || info.event_location) && (
              <div className="flex flex-col gap-1.5 text-sm lg:text-base text-[var(--theme-text)]/70 mt-auto">
                {info.event_date && <span>{info.event_date}</span>}
                {info.event_time && <span>{info.event_time}</span>}
                {info.event_location && <span>{info.event_location}{info.event_location_address ? `, ${info.event_location_address}` : ''}</span>}
              </div>
            )}
            <div className="w-[20%] h-px bg-[var(--theme-text)]/30 mx-auto" />
            <button
              onClick={() => formPaneRef.current?.scrollIntoView({ behavior: 'smooth' })}
              className="flex flex-col items-center gap-1.5 mb-8 cursor-pointer group"
            >
              <span className="text-sm text-[var(--theme-text)]/80 tracking-wider uppercase font-mono group-hover:text-[var(--theme-text)] transition-colors">RSVP</span>
              <ChevronDown className="h-5 w-5 text-[var(--theme-text)]/60 group-hover:text-[var(--theme-text)] transition-colors" />
            </button>
          </div>
        </div>

        {/* Bottom bar — desktop only */}
        <div className="hidden lg:flex items-end justify-between mt-4">
          <span className="text-[var(--theme-bg)] text-2xl font-[family-name:var(--font-display)]">{brandName}</span>
          <span className="text-[var(--theme-text)]/50 text-xs font-mono">{info.event_name}</span>
        </div>
      </div>

      {/* Pane 2: Program + RSVP form — slides over sticky hero on mobile */}
      <div ref={formPaneRef} className="min-h-screen relative z-10 overflow-hidden">
        {/* Decorative bg visible as border — only for dark themes with hero flowers */}
        {isDark && (
          <div className="absolute inset-0 bg-cover bg-center bg-fixed" style={{ backgroundImage: 'url(/images/rsvp-hero-flowers.jpg)' }} />
        )}

        {/* Surface inner panel */}
        <div ref={formInnerRef} className="relative min-h-screen bg-[var(--theme-surface)] p-6 lg:p-16">
          {/* Mobile: stacked accordions */}
          <div className="lg:hidden px-2 pt-8 pb-8 space-y-4">
            {/* Program accordion */}
            {(info.landing_program?.length > 0 || info.description || info.event_date || info.event_time || info.event_location) && (
              <div>
                <button
                  type="button"
                  className="w-full flex items-center justify-between py-4 cursor-pointer"
                  onClick={() => setMobileProgramOpen((v) => !v)}
                >
                  <span className="text-2xl text-[var(--theme-text)] font-[family-name:var(--font-display)]">The evening</span>
                  <ChevronDown className={`h-5 w-5 text-[var(--theme-text-muted)]/40 transition-transform ${mobileProgramOpen ? 'rotate-180' : ''}`} />
                </button>
                {mobileProgramOpen && (
                  <div className="space-y-6 pb-4">
                    {(info.event_date || info.event_time || info.event_location) && (
                      <div className="flex flex-wrap gap-x-6 gap-y-2 text-lg text-[var(--theme-text)] pb-6 border-b border-[var(--theme-border)]/30">
                        {info.event_date && <span>{info.event_date}</span>}
                        {info.event_time && <span>{info.event_time}</span>}
                        {info.event_location && <span>{info.event_location}{info.event_location_address ? `, ${info.event_location_address}` : ''}</span>}
                      </div>
                    )}
                    {info.program_description && (
                      <p className="text-[var(--theme-text)]/80 text-base leading-relaxed">{info.program_description}</p>
                    )}
                    {info.landing_program?.length > 0 && (
                      <div>
                        {info.landing_program.map((item, i) => {
                          const isEdge = i === 0 || i === (info.landing_program?.length ?? 0) - 1
                          return (
                          <div key={i} className={`${isEdge ? 'bg-[var(--theme-bg)]' : 'bg-[var(--theme-bg)]/80'} ${i > 0 ? 'border-t border-[var(--theme-border)]/30' : ''}`}>
                            <button
                              type="button"
                              className="w-full flex items-start gap-3 px-4 py-3 text-left cursor-pointer"
                              onClick={() => setExpandedProgram(expandedProgram === i ? null : i)}
                            >
                              <span className="font-mono text-xs text-[var(--theme-text)]/70 w-[60px] shrink-0 tracking-wider pt-0.5">
                                {item.time}
                              </span>
                              <div className="flex-1 min-w-0">
                                <p className="text-lg font-[family-name:var(--font-display)] text-[var(--theme-text)]">{item.title}</p>
                                {(item.speaker_name || item.speaker_org) && (
                                  <p className="text-xs mt-0.5 text-[var(--theme-text)]/50">
                                    {[item.speaker_name, item.speaker_org].filter(Boolean).join(', ')}
                                  </p>
                                )}
                              </div>
                              {item.speaker_image_url ? (
                                <img src={item.speaker_image_url} alt={item.speaker_name || ''} className="w-8 h-8 rounded-full object-cover shrink-0" />
                              ) : (
                                <div className="w-8 h-8 rounded-full shrink-0 bg-[var(--theme-text-muted)]/10" />
                              )}
                            </button>
                            {item.description && expandedProgram === i && (
                              <div className="px-4 pb-3 pt-0 ml-[72px]">
                                <div className="text-sm leading-relaxed text-[var(--theme-text)]/60" dangerouslySetInnerHTML={{ __html: item.description }} />
                              </div>
                            )}
                          </div>
                        )})}
                      </div>
                    )}
                    {info.landing_content && (
                      <div className="border-t border-[var(--theme-border)]/30 pt-6">
                        <div className="text-base leading-relaxed text-[var(--theme-text)]/80" dangerouslySetInnerHTML={{ __html: info.landing_content }} />
                      </div>
                    )}
                  </div>
                )}
              </div>
            )}

            {/* RSVP accordion */}
            <div className="border-t border-[var(--theme-border)]/30">
              <button
                type="button"
                className="w-full flex items-center justify-between py-4 cursor-pointer"
                onClick={() => setMobileRsvpOpen((v) => !v)}
              >
                <span className="text-2xl text-[var(--theme-text)] font-[family-name:var(--font-display)]">RSVP</span>
                <ChevronDown className={`h-5 w-5 text-[var(--theme-text-muted)]/40 transition-transform ${mobileRsvpOpen ? 'rotate-180' : ''}`} />
              </button>
              {mobileRsvpOpen && (
                <div className="pb-4">
                  {submitted ? successContent : formContent}
                </div>
              )}
            </div>
          </div>

          {/* Desktop: side-by-side layout */}
          <div className="hidden lg:flex px-10 pt-16 pb-16 gap-20 items-start">
              {/* Left: Program / event info — sticky while form scrolls */}
              <div className="flex-1 flex flex-col justify-center sticky top-16">
                <h2 className="text-4xl lg:text-5xl text-[var(--theme-text)] font-[family-name:var(--font-display)] leading-[1.1] mb-8">
                  The evening
                </h2>
                {(info.event_date || info.event_time || info.event_location) && (
                  <div className="flex flex-wrap gap-x-6 gap-y-2 text-lg text-[var(--theme-text)] mb-8 pb-8 border-b border-[var(--theme-border)]/30">
                    {info.event_date && <span>{info.event_date}</span>}
                    {info.event_time && <span>{info.event_time}</span>}
                    {info.event_location && <span>{info.event_location}{info.event_location_address ? `, ${info.event_location_address}` : ''}</span>}
                  </div>
                )}

                {info.program_description && (
                  <p className="text-[var(--theme-text)]/80 text-lg leading-relaxed mb-8">{info.program_description}</p>
                )}

                {info.landing_program?.length > 0 && (
                  <>
                    <div>
                      {info.landing_program.map((item, i) => {
                        const isEdge = i === 0 || i === (info.landing_program?.length ?? 0) - 1
                        return (
                        <div key={i} className={`${isEdge ? 'bg-[var(--theme-bg)]' : 'bg-[var(--theme-bg)]/80'} ${i > 0 ? 'border-t border-[var(--theme-border)]/30' : ''}`}>
                          <button
                            type="button"
                            className="w-full flex items-start gap-4 px-5 py-4 text-left cursor-pointer"
                            onClick={() => setExpandedProgram(expandedProgram === i ? null : i)}
                          >
                            <span className="font-mono text-sm text-[var(--theme-text)]/70 w-[80px] shrink-0 tracking-wider pt-0.5">
                              {item.time}
                            </span>
                            <div className="flex-1 min-w-0">
                              <p className="text-2xl font-[family-name:var(--font-display)] text-[var(--theme-text)]">{item.title}</p>
                              {(item.speaker_name || item.speaker_org) && (
                                <p className="text-sm mt-0.5 text-[var(--theme-text)]/50">
                                  {[item.speaker_name, item.speaker_org].filter(Boolean).join(', ')}
                                </p>
                              )}
                            </div>
                            {item.speaker_image_url ? (
                              <img
                                src={item.speaker_image_url}
                                alt={item.speaker_name || ''}
                                className="w-10 h-10 rounded-full object-cover shrink-0"
                              />
                            ) : item.speaker_contact_id ? (
                              <div className="w-10 h-10 rounded-full shrink-0 bg-[var(--theme-text-muted)]/10" />
                            ) : null}
                            {item.description && (
                              <ChevronDown className={`h-4 w-4 shrink-0 transition-transform text-[var(--theme-text-muted)]/40 ${expandedProgram === i ? 'rotate-180' : ''}`} />
                            )}
                          </button>
                          {item.description && expandedProgram === i && (
                            <div className="px-5 pb-3 pt-0 ml-[96px]">
                              <div className="text-sm leading-relaxed text-[var(--theme-text)]/60" dangerouslySetInnerHTML={{ __html: item.description }} />
                            </div>
                          )}
                        </div>
                      )})}
                    </div>
                  </>
                )}

                {info.landing_content && (
                  <div className="border-t border-[var(--theme-border)]/30 pt-8 mt-8">
                    <div className="text-base leading-relaxed text-[var(--theme-text)]/80" dangerouslySetInnerHTML={{ __html: info.landing_content }} />
                  </div>
                )}
              </div>

              <div className="w-px bg-[var(--theme-border)]/30 self-stretch" />

              {/* Right: RSVP form */}
              <div className="w-full flex-1">
                {submitted ? successContent : formContent}
              </div>
            </div>
        </div>
      </div>

      {/* Footer */}
      <footer className="relative z-10 bg-[#0d0d0d] text-[#777] text-xs px-6 lg:px-10 py-8">
        <p className="text-left text-[#999] leading-relaxed mb-6">
          The Outlook acknowledges Aboriginal Traditional Owners of Country throughout Australia and pays respect to their cultures and Elders past and present.
        </p>
        <div className="flex flex-col lg:flex-row items-start lg:items-center justify-between gap-4 font-mono uppercase tracking-wider">
          <span>&copy; 2021–2026 The Outlook Pty Ltd — ABN 72 655 333 403 <button onClick={() => setForwardOpen(true)} className="cursor-default">·</button></span>
          <div className="flex gap-6">
            <a href="https://theoutlook.io/contact-us" target="_blank" rel="noopener noreferrer" className="hover:text-white transition-colors">Contact us</a>
            <a href="https://theoutlook.io/about/about-the-outlook" target="_blank" rel="noopener noreferrer" className="hover:text-white transition-colors">About</a>
            <a href="https://theoutlook.io/legal/privacy-policy" target="_blank" rel="noopener noreferrer" className="hover:text-white transition-colors">Privacy policy</a>
          </div>
        </div>
      </footer>

      <RSVPForwardDrawer
        open={forwardOpen}
        onOpenChange={setForwardOpen}
        token={token!}
        eventName={info.event_name}
        listName={info.list_name}
        theme={theme}
      />
    </div>
  )
}
