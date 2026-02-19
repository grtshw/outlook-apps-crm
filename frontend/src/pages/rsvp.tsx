import { useState, useRef, useEffect } from 'react'
import { useParams, useLocation } from 'react-router'
import { useQuery, useMutation } from '@tanstack/react-query'
import { getRSVPInfo, submitRSVP } from '@/lib/api-public'
import type { RSVPSubmission } from '@/lib/api-public'
import { RSVPForwardDrawer } from '@/components/rsvp-forward-drawer'
import type { DietaryRequirement, AccessibilityRequirement } from '@/lib/pocketbase'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Checkbox } from '@/components/ui/checkbox'
import { Loader2, CircleCheck, XCircle, ChevronDown, ChevronUp } from 'lucide-react'
import gsap from 'gsap'
import { ScrollToPlugin } from 'gsap/ScrollToPlugin'

gsap.registerPlugin(ScrollToPlugin)

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

const inputClassName = 'bg-white/5 border-[#645C49]/30 text-white placeholder:text-[#A8A9B1]/50 focus-visible:ring-[#E95139]/40'
const textareaClassName = inputClassName

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

  const heroPaneRef = useRef<HTMLDivElement>(null)
  const formPaneRef = useRef<HTMLDivElement>(null)
  const downBtnRef = useRef<HTMLButtonElement>(null)
  const upBtnRef = useRef<HTMLButtonElement>(null)
  const heroImageRef = useRef<HTMLDivElement>(null)
  const heroContentRef = useRef<HTMLDivElement>(null)
  const formInnerRef = useRef<HTMLDivElement>(null)
  const heroAnimated = useRef(false)

  // Detect scroll: swap buttons + transition hero bg from terracotta to black
  useEffect(() => {
    let ticking = false
    const onScroll = () => {
      if (ticking) return
      ticking = true
      requestAnimationFrame(() => {
        ticking = false
        if (!downBtnRef.current || !upBtnRef.current || !heroPaneRef.current) return
        const progress = Math.min(Math.max(window.scrollY / window.innerHeight, 0), 1)
        const scrolledPastHero = progress > 0.5

        // Swap buttons
        if (scrolledPastHero) {
          gsap.set(downBtnRef.current, { opacity: 0, pointerEvents: 'none' })
          gsap.set(upBtnRef.current, { opacity: 1, pointerEvents: 'auto' })
        } else {
          gsap.set(downBtnRef.current, { opacity: 1, pointerEvents: 'auto' })
          gsap.set(upBtnRef.current, { opacity: 0, pointerEvents: 'none' })
        }

        // Interpolate hero bg: #E95139 → #020202
        const r = Math.round(0xE9 + (0x02 - 0xE9) * progress)
        const g = Math.round(0x51 + (0x02 - 0x51) * progress)
        const b = Math.round(0x39 + (0x02 - 0x39) * progress)
        heroPaneRef.current.style.backgroundColor = `rgb(${r},${g},${b})`

        // Mobile parallax: translate hero content upward as user scrolls
        if (window.innerWidth < 1024) {
          const parallaxY = window.scrollY * 0.15
          if (heroContentRef.current) heroContentRef.current.style.transform = `translateY(-${parallaxY}px)`
          if (heroImageRef.current) heroImageRef.current.style.transform = `translateY(-${parallaxY * 0.5}px)`
        }
      })
    }
    window.addEventListener('scroll', onScroll, { passive: true })
    return () => window.removeEventListener('scroll', onScroll)
  }, [])

  const scrollDown = () => {
    if (!formPaneRef.current || !downBtnRef.current || !upBtnRef.current) return
    gsap.set(downBtnRef.current, { pointerEvents: 'none' })
    gsap.to(downBtnRef.current, { opacity: 0, duration: 0.3, ease: 'power2.in' })
    gsap.to(window, {
      scrollTo: { y: formPaneRef.current, autoKill: false },
      duration: 1.2,
      ease: 'power3.inOut',
      onComplete: () => {
        gsap.set(upBtnRef.current!, { pointerEvents: 'auto' })
        gsap.fromTo(upBtnRef.current, { opacity: 0 }, { opacity: 1, duration: 0.4, ease: 'power2.out' })
      },
    })
  }

  const scrollUp = () => {
    if (!heroPaneRef.current || !downBtnRef.current || !upBtnRef.current) return
    gsap.set(upBtnRef.current, { pointerEvents: 'none' })
    gsap.to(upBtnRef.current, { opacity: 0, duration: 0.3, ease: 'power2.in' })
    gsap.to(window, {
      scrollTo: { y: heroPaneRef.current, autoKill: false },
      duration: 1.2,
      ease: 'power3.inOut',
      onComplete: () => {
        gsap.set(downBtnRef.current!, { pointerEvents: 'auto' })
        gsap.fromTo(downBtnRef.current, { opacity: 0 }, { opacity: 1, duration: 0.4, ease: 'power2.out' })
      },
    })
  }

  // Hero entrance animation — staggered fade-in on load
  useEffect(() => {
    if (!info || heroAnimated.current) return
    heroAnimated.current = true

    // Hero image container: subtle scale + fade
    if (heroImageRef.current) {
      gsap.fromTo(heroImageRef.current,
        { opacity: 0, scale: 1.04 },
        { opacity: 1, scale: 1, duration: 1.4, ease: 'power2.out' }
      )
    }

    // Hero right column content: stagger children up
    if (heroContentRef.current) {
      const children = heroContentRef.current.children
      gsap.fromTo(children,
        { opacity: 0, y: 24 },
        { opacity: 1, y: 0, duration: 0.8, ease: 'power2.out', stagger: 0.15, delay: 0.3 }
      )
    }
  }, [info])

  // Carousel crossfade + noise animation
  useEffect(() => {
    if (!heroImageRef.current) return

    const images = heroImageRef.current.querySelectorAll('img[data-carousel-index]') as NodeListOf<HTMLImageElement>
    if (images.length < 2) return

    const hold = 5      // seconds each image is fully visible
    const fade = 1.5    // crossfade duration

    const tl = gsap.timeline({ repeat: -1 })

    images.forEach((img, i) => {
      const next = images[(i + 1) % images.length]

      // Hold current image, then crossfade: fade in next while fading out current
      // Also apply subtle Ken Burns scale on incoming image
      tl.to({}, { duration: hold })
        .set(next, { scale: 1.05 })
        .to(next, { opacity: 1, scale: 1, duration: fade, ease: 'power1.inOut' }, `>`)
        .to(img, { opacity: 0, duration: fade, ease: 'power1.inOut' }, `<`)
    })

    return () => {
      tl.kill()
    }
  }, [])

  // Form pane entrance — fade up when scrolled into view
  useEffect(() => {
    if (!formInnerRef.current) return
    const el = formInnerRef.current

    // Start hidden
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
      <div className="rsvp-theme min-h-screen flex items-center justify-center bg-[#020202]">
        <Loader2 className="h-6 w-6 animate-spin text-[#A8A9B1]/40" />
      </div>
    )
  }

  if (error) {
    return (
      <div className="rsvp-theme min-h-screen flex items-center justify-center bg-[#020202] p-4">
        <div className="text-center space-y-3 max-w-md">
          <img src="/images/logo-white.svg" alt="The Outlook" className="h-8 mx-auto mb-6" />
          <p className="text-lg text-white font-[family-name:var(--font-display)]">RSVP not available</p>
          <p className="text-sm text-[#A8A9B1]">{errorMessage}</p>
        </div>
      </div>
    )
  }

  if (!info) return null

  const alreadyResponded = info.already_responded && info.rsvp_status

  const successContent = (
    <div className="max-w-md w-full space-y-6 text-center mx-auto">
      {submittedResponse === 'accepted' ? (
        <CircleCheck className="h-16 w-16 text-[#E95139] mx-auto" />
      ) : (
        <XCircle className="h-16 w-16 text-[#A8A9B1]/40 mx-auto" />
      )}
      <div className="space-y-2">
        <p className="text-2xl text-white font-[family-name:var(--font-display)]">
          {submittedResponse === 'accepted' ? "You're confirmed" : 'Response received'}
        </p>
        <p className="text-sm text-[#A8A9B1]">
          {submittedResponse === 'accepted'
            ? `Thanks ${displayName}, we look forward to seeing you at ${info.event_name || info.list_name}.`
            : `Thanks for letting us know, ${displayName}.`}
        </p>
      </div>
      {submittedResponse === 'accepted' && (
        <button
          onClick={() => setForwardOpen(true)}
          className="text-sm text-[#A8A9B1] underline underline-offset-4 hover:text-white cursor-pointer"
        >
          Know someone who'd be interested? Forward this invitation
        </button>
      )}
    </div>
  )

  const formContent = (
    <>
      {alreadyResponded && (
        <div className="text-center mb-8">
          <p className="text-sm text-[#A8A9B1]">
            You previously {info.rsvp_status === 'accepted' ? 'accepted' : 'declined'} this invitation. You can update your response below.
          </p>
        </div>
      )}
      <h2 className="hidden lg:block text-4xl lg:text-5xl text-white font-[family-name:var(--font-display)] leading-[1.1] mb-8">RSVP</h2>

      {/* Response selection */}
      <div className="grid grid-cols-2 gap-3 mb-6">
        <label
          className={`flex items-center gap-3 p-4 cursor-pointer transition-colors border border-[#645C49]/30 ${
            response === 'accepted' ? 'bg-[#E95139] border-[#E95139]' : 'hover:border-[#645C49]'
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
            response === 'accepted' ? 'border-[#A8A9B1]' : 'border-[#A8A9B1]/40'
          }`}>
            {response === 'accepted' && <div className="w-2 h-2 rounded-full bg-[#A8A9B1]" />}
          </div>
          <span className="text-sm text-white">I can make it</span>
        </label>
        <label
          className={`flex items-center gap-3 p-4 cursor-pointer transition-colors border border-[#645C49]/30 ${
            response === 'declined' ? 'bg-[#020202]/40 border-[#A8A9B1]/60' : 'hover:border-[#645C49]'
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
            response === 'declined' ? 'border-[#A8A9B1]' : 'border-[#A8A9B1]/40'
          }`}>
            {response === 'declined' && <div className="w-2 h-2 rounded-full bg-[#A8A9B1]" />}
          </div>
          <span className="text-sm text-white">I can't make it</span>
        </label>
      </div>

      {response && (
      <div className="space-y-6 pb-16">

          {/* Your details */}
          <div className="border-t border-[#645C49]/30 pt-10 mt-4">
            <h3 className="text-2xl text-white font-[family-name:var(--font-display)] mb-4">Your details</h3>
            <div className="space-y-4">
              <div className="grid grid-cols-2 gap-3">
                <div>
                  <label className="block text-sm text-white mb-1.5">First name <span className="text-[#E95139]">*</span></label>
                  <Input
                    value={firstName}
                    onChange={(e) => setFirstName(e.target.value)}
                    placeholder="First name"
                    required
                    className={inputClassName}
                  />
                </div>
                <div>
                  <label className="block text-sm text-white mb-1.5">Last name</label>
                  <Input
                    value={lastName}
                    onChange={(e) => setLastName(e.target.value)}
                    placeholder="Last name"
                    className={inputClassName}
                  />
                </div>
              </div>

              <div>
                <label className="block text-sm text-white mb-1.5">Email <span className="text-[#E95139]">*</span></label>
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
                  <label className="block text-sm text-white mb-1.5">Phone</label>
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
            <details className="group border-t border-[#645C49]/30 pt-6 mt-10">
              <summary className="flex items-center justify-between cursor-pointer list-none">
                <h3 className="text-2xl text-white font-[family-name:var(--font-display)]">Additional requirements</h3>
                <ChevronDown className="h-5 w-5 text-[#A8A9B1]/40 transition-transform group-open:rotate-180" />
              </summary>
              <div className="space-y-3 mt-4">
                {/* Dietary requirements */}
                <details className="group rounded-lg border border-[#645C49]/20">
                  <summary className="flex items-center justify-between cursor-pointer p-4 list-none">
                    <span className="text-sm text-white">Dietary requirements {dietaryRequirements.length > 0 || dietaryOther ? `(${dietaryRequirements.length + (dietaryOther ? 1 : 0)})` : ''}</span>
                    <ChevronDown className="h-4 w-4 text-[#A8A9B1]/40 transition-transform group-open:rotate-180" />
                  </summary>
                  <div className="px-4 pb-4 space-y-2">
                    <div className="flex flex-wrap gap-x-4 gap-y-2">
                      {DIETARY_OPTIONS.map(({ value, label }) => (
                        <label key={value} className="flex items-center gap-1.5 cursor-pointer">
                          <Checkbox
                            checked={dietaryRequirements.includes(value)}
                            onCheckedChange={() => toggleDietary(value)}
                          />
                          <span className="text-sm text-white">{label}</span>
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
                <details className="group rounded-lg border border-[#645C49]/20">
                  <summary className="flex items-center justify-between cursor-pointer p-4 list-none">
                    <span className="text-sm text-white">Accessibility requirements {accessibilityRequirements.length > 0 || accessibilityOther ? `(${accessibilityRequirements.length + (accessibilityOther ? 1 : 0)})` : ''}</span>
                    <ChevronDown className="h-4 w-4 text-[#A8A9B1]/40 transition-transform group-open:rotate-180" />
                  </summary>
                  <div className="px-4 pb-4 space-y-2">
                    <div className="flex flex-wrap gap-x-4 gap-y-2">
                      {ACCESSIBILITY_OPTIONS.map(({ value, label }) => (
                        <label key={value} className="flex items-center gap-1.5 cursor-pointer">
                          <Checkbox
                            checked={accessibilityRequirements.includes(value)}
                            onCheckedChange={() => toggleAccessibility(value)}
                          />
                          <span className="text-sm text-white">{label}</span>
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
            <div className="border-t border-[#645C49]/30 pt-10 mt-10">
              <h3 className="text-2xl text-white font-[family-name:var(--font-display)] mb-4">Invitation</h3>
              <div className="space-y-4">
                {/* Plus one */}
                <div
                  className="flex items-start gap-3 cursor-pointer rounded-lg border border-[#645C49]/20 p-4"
                  onClick={() => setPlusOne((v) => !v)}
                >
                  <Checkbox
                    checked={plusOne}
                    onCheckedChange={(checked) => setPlusOne(checked === true)}
                    className="mt-0.5"
                  />
                  <div>
                    <span className="text-sm text-white">I'd like to bring a plus-one</span>
                  </div>
                </div>

                {plusOne && (
                  <div className="space-y-4 pl-4 border-l-2 border-[#645C49]/30">
                    <div className="grid grid-cols-2 gap-3">
                      <div>
                        <label className="block text-sm text-white mb-1.5">First name</label>
                        <Input
                          value={plusOneName}
                          onChange={(e) => setPlusOneName(e.target.value)}
                          placeholder="First name"
                          className={inputClassName}
                        />
                      </div>
                      <div>
                        <label className="block text-sm text-white mb-1.5">Last name</label>
                        <Input
                          value={plusOneLastName}
                          onChange={(e) => setPlusOneLastName(e.target.value)}
                          placeholder="Last name"
                          className={inputClassName}
                        />
                      </div>
                    </div>
                    <div>
                      <label className="block text-sm text-white mb-1.5">Job title</label>
                      <Input
                        value={plusOneJobTitle}
                        onChange={(e) => setPlusOneJobTitle(e.target.value)}
                        placeholder="Job title"
                        className={inputClassName}
                      />
                    </div>
                    <div>
                      <label className="block text-sm text-white mb-1.5">Company</label>
                      <Input
                        value={plusOneCompany}
                        onChange={(e) => setPlusOneCompany(e.target.value)}
                        placeholder="Company"
                        className={inputClassName}
                      />
                    </div>
                    <div>
                      <label className="block text-sm text-white mb-1.5">Email</label>
                      <Input
                        type="email"
                        value={plusOneEmail}
                        onChange={(e) => setPlusOneEmail(e.target.value)}
                        placeholder="their@email.com"
                        className={inputClassName}
                      />
                    </div>
                    <div>
                      <label className="block text-sm text-white mb-1.5">Dietary requirements</label>
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
                    <label className="block text-sm text-white mb-1.5">Who invited you?</label>
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
          <div className="border-t border-[#645C49]/30 pt-10 mt-10">
            <h3 className="text-2xl text-white font-[family-name:var(--font-display)] mb-4">Anything else?</h3>
            <div className="space-y-4">
              {/* Comments */}
              <div>
                <label className="block text-sm text-white mb-1.5">Comments</label>
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
            className="flex items-start gap-3 cursor-pointer border-t border-[#645C49]/30 pt-6 mt-6"
            onClick={() => setPolicyAccepted((v) => !v)}
          >
            <Checkbox
              checked={policyAccepted}
              onCheckedChange={(checked) => setPolicyAccepted(checked === true)}
              className="mt-0.5"
            />
            <div className="text-sm leading-relaxed">
              <span className="text-[#A8A9B1]">
                I agree to The Outlook&apos;s{' '}
                <a
                  href="https://theoutlook.io/legal/privacy-policy"
                  target="_blank"
                  rel="noopener noreferrer"
                  className="underline text-white hover:text-[#E95139]"
                  onClick={(e) => e.stopPropagation()}
                >
                  privacy policy
                </a>
              </span>
            </div>
          </div>

          {/* Error */}
          {mutation.isError && (
            <p className="text-sm text-[#E95139]">
              {mutation.error instanceof Error ? mutation.error.message : 'Something went wrong'}
            </p>
          )}

          {/* Submit button */}
          <button
            onClick={handleSubmit}
            disabled={mutation.isPending || !policyAccepted || !response || !firstName.trim() || !email.trim()}
            className="bg-[#E95139] hover:bg-[#E95139]/90 disabled:opacity-40 disabled:cursor-not-allowed text-white px-12 h-12 text-base transition-colors cursor-pointer"
          >
            {mutation.isPending ? 'Submitting...' : 'Submit'}
          </button>
        </div>
      )}
    </>
  )

  // Editorial hero + form layout
  return (
    <div className="rsvp-theme bg-[#E95139]">
      {/* Fixed semicircle nav — desktop only */}
      <div className="hidden lg:flex fixed bottom-0 left-1/2 -translate-x-1/2 z-50 flex-col items-center">
        {/* Graphite semicircle — scroll down */}
        <button
          ref={downBtnRef}
          onClick={scrollDown}
          className="w-20 h-10 rounded-t-full bg-[#1A1917] flex items-center justify-center cursor-pointer hover:scale-105 transition-transform"
          aria-label="Scroll to RSVP form"
        >
          <ChevronDown className="h-5 w-5 text-white/70" />
        </button>
        {/* Orange semicircle — scroll up (hidden initially) */}
        <button
          ref={upBtnRef}
          onClick={scrollUp}
          className="w-20 h-10 rounded-t-full bg-[#E95139] flex items-center justify-center cursor-pointer hover:scale-105 transition-transform absolute bottom-0"
          style={{ opacity: 0, pointerEvents: 'none' }}
          aria-label="Scroll back to top"
        >
          <ChevronUp className="h-5 w-5 text-white/70" />
        </button>
      </div>

      {/* Pane 1: Full-viewport magazine spread — sticky on mobile for card-over effect */}
      <div ref={heroPaneRef} className="h-screen sticky top-0 lg:relative bg-[#E95139] p-6 lg:p-10 flex flex-col">
        {/* Top bar */}
        <div className="flex items-center justify-between mb-4">
          <span className="text-white/80 text-sm font-[family-name:var(--font-display)] lg:hidden">The Outlook After Dark</span>
          <span className="text-white/60 text-xs font-mono tracking-wider uppercase max-lg:ml-auto">{info.prefilled_first_name ? `${info.prefilled_first_name}, you're invited` : "You're invited"}</span>
        </div>

        {/* Main spread */}
        <div className="flex-1 flex flex-col lg:flex-row gap-6 lg:gap-10 min-h-0">
          <div ref={heroImageRef} className="h-[35vh] lg:h-auto lg:flex-[2] min-w-0 overflow-hidden shrink-0 relative saturate-[0.3]">
            {/* Carousel images — stacked, crossfade via GSAP */}
            {['/images/rsvp-hero-dinner.jpg', '/images/rsvp-hero-flowers.jpg', '/images/rsvp-hero-flowers-dark.jpg'].map((src, i) => (
              <img
                key={src}
                src={src}
                alt=""
                data-carousel-index={i}
                className="absolute inset-0 w-full h-full object-cover"
                style={{ opacity: i === 0 ? 1 : 0 }}
              />
            ))}
          </div>
          <div ref={heroContentRef} className="flex-1 lg:flex-[1] min-w-0 flex flex-col gap-5 items-center text-center">
            <div className="pt-6 lg:pt-4">
              <img src="/images/logo-white.svg" alt="The Outlook" className="h-8 opacity-80 mx-auto mb-6 lg:mb-10" />
              {info.organisation_logo_url && (
                <img
                  src={info.organisation_logo_url}
                  alt={info.organisation_name || ''}
                  className="h-8 mb-4 object-contain mx-auto"
                />
              )}
              <h1 className="text-3xl lg:text-5xl xl:text-6xl text-white font-[family-name:var(--font-display)] leading-[1]">
                {info.list_name}
              </h1>
            </div>
            <div className="text-white/80 text-lg lg:text-xl leading-relaxed">
              {info.description ? (
                <p>{info.description}</p>
              ) : (
                <p>An intimate evening of conversation, connection and great food.</p>
              )}
            </div>
            {(info.event_date || info.event_time || info.event_location) && (
              <div className="flex flex-col gap-1.5 text-sm lg:text-base text-white/70 mt-auto mb-8">
                {info.event_date && <span>{info.event_date}</span>}
                {info.event_time && <span>{info.event_time}</span>}
                {info.event_location && <span>{info.event_location}{info.event_location_address ? `, ${info.event_location_address}` : ''}</span>}
              </div>
            )}
          </div>
        </div>

        {/* Bottom bar — desktop only */}
        <div className="hidden lg:flex items-end justify-between mt-4">
          <span className="text-white/80 text-sm font-[family-name:var(--font-display)]">The Outlook After Dark</span>
          <span className="text-white/50 text-xs font-mono">{info.event_name}</span>
        </div>
      </div>

      {/* Pane 2: Program + RSVP form — slides over sticky hero on mobile */}
      <div ref={formPaneRef} className="min-h-screen relative z-10 overflow-hidden lg:overflow-visible">
        {/* Flower bg visible as border */}
        <div className="absolute inset-0 bg-cover bg-center bg-fixed" style={{ backgroundImage: 'url(/images/rsvp-hero-flowers.jpg)' }} />

        {/* Graphite inner panel */}
        <div ref={formInnerRef} className="relative min-h-screen bg-[#1A1917] p-4 lg:p-10">
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
                  <span className="text-2xl text-white font-[family-name:var(--font-display)]">The evening</span>
                  <ChevronDown className={`h-5 w-5 text-[#A8A9B1]/40 transition-transform ${mobileProgramOpen ? 'rotate-180' : ''}`} />
                </button>
                {mobileProgramOpen && (
                  <div className="space-y-6 pb-4">
                    {(info.event_date || info.event_time || info.event_location) && (
                      <div className="flex flex-wrap gap-x-6 gap-y-2 text-lg text-white pb-6 border-b border-[#645C49]/30">
                        {info.event_date && <span>{info.event_date}</span>}
                        {info.event_time && <span>{info.event_time}</span>}
                        {info.event_location && <span>{info.event_location}{info.event_location_address ? `, ${info.event_location_address}` : ''}</span>}
                      </div>
                    )}
                    {info.landing_program?.length > 0 && (
                      <div>
                        {info.landing_program.map((item, i) => {
                          const isEdge = i === 0 || i === (info.landing_program?.length ?? 0) - 1
                          return (
                          <div key={i} className={`${isEdge ? 'bg-[#020202]' : 'bg-[#020202]/80'} ${i > 0 ? 'border-t border-[#645C49]/30' : ''}`}>
                            <button
                              type="button"
                              className="w-full flex items-start gap-3 px-4 py-3 text-left cursor-pointer"
                              onClick={() => setExpandedProgram(expandedProgram === i ? null : i)}
                            >
                              <span className="font-mono text-xs text-white/70 w-[60px] shrink-0 tracking-wider pt-0.5">
                                {item.time}
                              </span>
                              <div className="flex-1 min-w-0">
                                <p className="text-lg font-[family-name:var(--font-display)] text-white">{item.title}</p>
                                {(item.speaker_name || item.speaker_org) && (
                                  <p className="text-xs mt-0.5 text-white/50">
                                    {[item.speaker_name, item.speaker_org].filter(Boolean).join(', ')}
                                  </p>
                                )}
                              </div>
                              {item.speaker_image_url ? (
                                <img src={item.speaker_image_url} alt={item.speaker_name || ''} className="w-8 h-8 rounded-full object-cover shrink-0" />
                              ) : (
                                <div className="w-8 h-8 rounded-full shrink-0 bg-[#A8A9B1]/10" />
                              )}
                            </button>
                            {item.description && expandedProgram === i && (
                              <div className="px-4 pb-3 pt-0 ml-[72px]">
                                <div className="text-sm leading-relaxed text-white/60" dangerouslySetInnerHTML={{ __html: item.description }} />
                              </div>
                            )}
                          </div>
                        )})}
                      </div>
                    )}
                    {info.landing_content && (
                      <div className="border-t border-[#645C49]/30 pt-6">
                        <div className="text-base leading-relaxed text-white/80" dangerouslySetInnerHTML={{ __html: info.landing_content }} />
                      </div>
                    )}
                  </div>
                )}
              </div>
            )}

            {/* RSVP accordion */}
            <div className="border-t border-[#645C49]/30">
              <button
                type="button"
                className="w-full flex items-center justify-between py-4 cursor-pointer"
                onClick={() => setMobileRsvpOpen((v) => !v)}
              >
                <span className="text-2xl text-white font-[family-name:var(--font-display)]">RSVP</span>
                <ChevronDown className={`h-5 w-5 text-[#A8A9B1]/40 transition-transform ${mobileRsvpOpen ? 'rotate-180' : ''}`} />
              </button>
              {mobileRsvpOpen && (
                <div className="pb-4">
                  {submitted ? successContent : formContent}
                </div>
              )}
            </div>
          </div>

          {/* Desktop: side-by-side layout */}
          <div className="hidden lg:flex px-6 pt-16 pb-16 gap-12">
              {/* Left: Program / event info */}
              <div className="flex-1 flex flex-col justify-center sticky top-20 self-start">
                <h2 className="text-4xl lg:text-5xl text-white font-[family-name:var(--font-display)] leading-[1.1] mb-8">
                  {info.event_name || info.list_name}
                </h2>
                {(info.event_date || info.event_time || info.event_location) && (
                  <div className="flex flex-wrap gap-x-6 gap-y-2 text-lg text-white mb-8 pb-8 border-b border-[#645C49]/30">
                    {info.event_date && <span>{info.event_date}</span>}
                    {info.event_time && <span>{info.event_time}</span>}
                    {info.event_location && <span>{info.event_location}{info.event_location_address ? `, ${info.event_location_address}` : ''}</span>}
                  </div>
                )}

                {info.landing_program?.length > 0 && (
                  <>
                    <p className="eyebrow text-white/70 mb-4">The evening</p>
                    <div>
                      {info.landing_program.map((item, i) => {
                        const isEdge = i === 0 || i === (info.landing_program?.length ?? 0) - 1
                        return (
                        <div key={i} className={`${isEdge ? 'bg-[#020202]' : 'bg-[#020202]/80'} ${i > 0 ? 'border-t border-[#645C49]/30' : ''}`}>
                          <button
                            type="button"
                            className="w-full flex items-start gap-4 px-5 py-4 text-left cursor-pointer"
                            onClick={() => setExpandedProgram(expandedProgram === i ? null : i)}
                          >
                            <span className="font-mono text-sm text-white/70 w-[80px] shrink-0 tracking-wider pt-0.5">
                              {item.time}
                            </span>
                            <div className="flex-1 min-w-0">
                              <p className="text-2xl font-[family-name:var(--font-display)] text-white">{item.title}</p>
                              {(item.speaker_name || item.speaker_org) && (
                                <p className="text-sm mt-0.5 text-white/50">
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
                              <div className="w-10 h-10 rounded-full shrink-0 bg-[#A8A9B1]/10" />
                            ) : null}
                            {item.description && (
                              <ChevronDown className={`h-4 w-4 shrink-0 transition-transform text-[#A8A9B1]/40 ${expandedProgram === i ? 'rotate-180' : ''}`} />
                            )}
                          </button>
                          {item.description && expandedProgram === i && (
                            <div className="px-5 pb-3 pt-0 ml-[96px]">
                              <div className="text-sm leading-relaxed text-white/60" dangerouslySetInnerHTML={{ __html: item.description }} />
                            </div>
                          )}
                        </div>
                      )})}
                    </div>
                  </>
                )}

                {info.landing_content && (
                  <div className="border-t border-[#645C49]/30 pt-8 mt-8">
                    <div className="text-base leading-relaxed text-white/80" dangerouslySetInnerHTML={{ __html: info.landing_content }} />
                  </div>
                )}
              </div>

              <div className="w-px bg-[#645C49]/30 self-stretch" />

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
      />
    </div>
  )
}
