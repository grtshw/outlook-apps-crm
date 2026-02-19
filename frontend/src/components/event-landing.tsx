import { useState } from 'react'
import type { RSVPInfo } from '@/lib/api-public'
import type { ProgramItem } from '@/lib/pocketbase'
import { ChevronDown } from 'lucide-react'
import { cn } from '@/lib/utils'

interface EventLandingProps {
  info: RSVPInfo
  children: React.ReactNode
}

export function EventLanding({ info, children }: EventLandingProps) {
  const headline = info.landing_headline || info.event_name || info.list_name
  const [expandedProgram, setExpandedProgram] = useState<number | null>(null)

  const venueString = [info.event_venue, info.event_venue_city, info.event_venue_country]
    .filter(Boolean)
    .join(', ')

  return (
    <div className="rsvp-theme min-h-screen bg-[#020202]">
      {/* Hero section */}
      <div className="relative">
        {info.landing_image_url && (
          <div className="absolute inset-0 overflow-hidden">
            <img
              src={info.landing_image_url}
              alt=""
              className="w-full h-full object-cover opacity-30"
            />
          </div>
        )}
        <div className="relative max-w-3xl mx-auto px-4 pt-12 pb-10 text-center">
          <img src="/images/logo-white.svg" alt="The Outlook" className="h-8 mx-auto mb-8" />
          <h1 className="text-3xl text-white mb-4 font-[family-name:var(--font-display)]">{headline}</h1>
          {(info.event_date || info.event_start_date || venueString) && (
            <div className="text-[#A8A9B1] space-y-1 text-sm">
              {formatEventDate(info) && <p>{formatEventDate(info)}</p>}
              {venueString && <p>{venueString}</p>}
            </div>
          )}
        </div>
      </div>

      {/* Description */}
      {info.landing_description && (
        <div className="max-w-3xl mx-auto px-4 py-8">
          <div
            className="prose prose-invert prose-sm max-w-none"
            dangerouslySetInnerHTML={{ __html: info.landing_description }}
          />
        </div>
      )}

      {/* Program */}
      {info.landing_program?.length > 0 && (
        <div className="max-w-3xl mx-auto px-4 py-8">
          <p className="eyebrow text-[#A8A9B1] mb-4">Program</p>
          <div className="space-y-px">
            {info.landing_program.map((item, i) => (
              <ProgramRow
                key={i}
                item={item}
                isFirst={i === 0}
                expanded={expandedProgram === i}
                onToggle={() => setExpandedProgram(expandedProgram === i ? null : i)}
              />
            ))}
          </div>
        </div>
      )}

      {/* Additional content */}
      {info.landing_content && (
        <div className="max-w-3xl mx-auto px-4 py-8">
          <div
            className="prose prose-invert prose-sm max-w-none"
            dangerouslySetInnerHTML={{ __html: info.landing_content }}
          />
        </div>
      )}

      {/* RSVP form */}
      <div className="max-w-2xl mx-auto px-4 py-12">
        {children}
      </div>
    </div>
  )
}

function ProgramRow({
  item,
  isFirst,
  expanded,
  onToggle,
}: {
  item: ProgramItem
  isFirst: boolean
  expanded: boolean
  onToggle: () => void
}) {
  const hasExpandable = !!item.description

  return (
    <div
      className={cn(
        'rounded-lg px-5 py-4',
        isFirst
          ? 'bg-[#020202] border border-[#645C49]/20'
          : 'bg-[#2F2D29]'
      )}
    >
      <div
        role={hasExpandable ? 'button' : undefined}
        tabIndex={hasExpandable ? 0 : undefined}
        onClick={hasExpandable ? onToggle : undefined}
        onKeyDown={hasExpandable ? (e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); onToggle() } } : undefined}
        className={cn(
          'flex items-center gap-4 w-full text-left',
          hasExpandable && 'cursor-pointer'
        )}
      >
        {/* Time */}
        <span className="font-mono text-xs shrink-0 w-[80px] tracking-wider text-[#A8A9B1]">
          {item.time}
        </span>

        {/* Title + speaker */}
        <div className="flex-1 min-w-0">
          <p className="text-sm text-white">
            {item.title}
          </p>
          {(item.speaker_name || item.speaker_org) && (
            <p className="text-xs mt-0.5 text-[#A8A9B1]/60">
              {[item.speaker_name, item.speaker_org].filter(Boolean).join(': ')}
            </p>
          )}
        </div>

        {/* Speaker avatar */}
        {item.speaker_image_url && (
          <img
            src={item.speaker_image_url}
            alt={item.speaker_name || ''}
            className="h-10 w-10 rounded-full object-cover shrink-0"
          />
        )}

        {/* Chevron */}
        {hasExpandable && (
          <ChevronDown
            className={cn(
              'h-4 w-4 shrink-0 transition-transform text-[#A8A9B1]/40',
              expanded && 'rotate-180'
            )}
          />
        )}
      </div>

      {/* Expanded description */}
      {expanded && item.description && (
        <div
          className="mt-3 ml-[96px] text-sm text-[#A8A9B1]"
          dangerouslySetInnerHTML={{ __html: item.description }}
        />
      )}
    </div>
  )
}

function formatEventDate(info: RSVPInfo): string {
  const dateStr = info.event_start_date || info.event_date
  if (!dateStr) return ''

  const parts: string[] = []

  // Try to parse and format the date nicely
  try {
    const date = new Date(dateStr)
    if (!isNaN(date.getTime())) {
      parts.push(
        date.toLocaleDateString('en-AU', {
          weekday: 'long',
          day: 'numeric',
          month: 'long',
          year: 'numeric',
        })
      )
    } else {
      parts.push(dateStr)
    }
  } catch {
    parts.push(dateStr)
  }

  if (info.event_start_time) {
    parts.push(info.event_start_time)
    if (info.event_end_time) {
      parts.push(`– ${info.event_end_time}`)
    }
  }

  return parts.join(' ')
}
