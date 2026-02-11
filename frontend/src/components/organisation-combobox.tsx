import { useState, useRef, useEffect, useCallback } from 'react'
import { X } from 'lucide-react'
import { cn } from '@/lib/utils'
import { Input } from '@/components/ui/input'
import type { Organisation } from '@/lib/pocketbase'

interface OrganisationComboboxProps {
  value: string
  organisations: Organisation[]
  onChange: (orgId: string) => void
  disabled?: boolean
}

export function OrganisationCombobox({
  value,
  organisations,
  onChange,
  disabled,
}: OrganisationComboboxProps) {
  const [inputValue, setInputValue] = useState('')
  const [isOpen, setIsOpen] = useState(false)
  const [highlightIndex, setHighlightIndex] = useState(-1)
  const containerRef = useRef<HTMLDivElement>(null)
  const inputRef = useRef<HTMLInputElement>(null)
  const listRef = useRef<HTMLUListElement>(null)

  const selectedOrg = value
    ? organisations.find((o) => o.id === value) ?? null
    : null

  const filtered = inputValue.trim()
    ? organisations.filter((org) =>
        org.name.toLowerCase().includes(inputValue.toLowerCase())
      )
    : organisations

  const handleSelect = useCallback(
    (org: Organisation) => {
      setInputValue('')
      setIsOpen(false)
      setHighlightIndex(-1)
      onChange(org.id)
    },
    [onChange]
  )

  const handleClear = () => {
    onChange('')
    setInputValue('')
    setIsOpen(false)
    setTimeout(() => inputRef.current?.focus(), 0)
  }

  const handleInputChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    setInputValue(e.target.value)
    setIsOpen(true)
    setHighlightIndex(-1)
  }

  const handleInputBlur = () => {
    setTimeout(() => {
      setIsOpen(false)
      setHighlightIndex(-1)
    }, 200)
  }

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (!isOpen || filtered.length === 0) return

    if (e.key === 'ArrowDown') {
      e.preventDefault()
      setHighlightIndex((i) => Math.min(i + 1, filtered.length - 1))
    } else if (e.key === 'ArrowUp') {
      e.preventDefault()
      setHighlightIndex((i) => Math.max(i - 1, 0))
    } else if (e.key === 'Enter' && highlightIndex >= 0) {
      e.preventDefault()
      handleSelect(filtered[highlightIndex])
    } else if (e.key === 'Escape') {
      setIsOpen(false)
      setHighlightIndex(-1)
    }
  }

  // Scroll highlighted item into view
  useEffect(() => {
    if (highlightIndex >= 0 && listRef.current) {
      const item = listRef.current.children[highlightIndex] as HTMLElement
      item?.scrollIntoView({ block: 'nearest' })
    }
  }, [highlightIndex])

  // Close on click outside
  useEffect(() => {
    const handleClickOutside = (e: MouseEvent) => {
      if (containerRef.current && !containerRef.current.contains(e.target as Node)) {
        setIsOpen(false)
      }
    }
    document.addEventListener('mousedown', handleClickOutside)
    return () => document.removeEventListener('mousedown', handleClickOutside)
  }, [])

  if (selectedOrg) {
    return (
      <div className="flex items-center gap-2">
        <span className="inline-flex items-center gap-2 rounded-full border bg-muted/50 py-1 pl-1 pr-3 text-sm">
          {selectedOrg.logo_square_url ? (
            <img
              src={selectedOrg.logo_square_url}
              alt=""
              className="size-6 rounded-full object-contain bg-white"
            />
          ) : (
            <span className="inline-flex items-center justify-center size-6 rounded-full bg-muted text-muted-foreground text-xs">
              {selectedOrg.name[0]?.toUpperCase()}
            </span>
          )}
          <span>{selectedOrg.name}</span>
        </span>
        {!disabled && (
          <button
            type="button"
            onClick={handleClear}
            className="text-muted-foreground hover:text-foreground transition-colors"
          >
            <X className="size-4" />
          </button>
        )}
      </div>
    )
  }

  return (
    <div ref={containerRef} className="relative">
      <Input
        ref={inputRef}
        value={inputValue}
        onChange={handleInputChange}
        onFocus={() => setIsOpen(true)}
        onBlur={handleInputBlur}
        onKeyDown={handleKeyDown}
        placeholder="Search organisations..."
        disabled={disabled}
      />
      {isOpen && filtered.length > 0 && (
        <ul
          ref={listRef}
          className="absolute top-full left-0 right-0 z-[100] mt-1 max-h-48 overflow-y-auto rounded-md border bg-popover text-popover-foreground shadow-md"
        >
          {filtered.slice(0, 20).map((org, index) => (
            <li
              key={org.id}
              className={cn(
                'flex items-center gap-2 px-3 py-2 text-sm cursor-pointer',
                index === highlightIndex
                  ? 'bg-accent text-accent-foreground'
                  : 'hover:bg-accent hover:text-accent-foreground'
              )}
              onMouseDown={(e) => {
                e.preventDefault()
                handleSelect(org)
              }}
              onMouseEnter={() => setHighlightIndex(index)}
            >
              {org.logo_square_url ? (
                <img
                  src={org.logo_square_url}
                  alt=""
                  className="size-5 rounded-full object-contain bg-white"
                />
              ) : (
                <span className="inline-flex items-center justify-center size-5 rounded-full bg-muted text-muted-foreground text-[10px]">
                  {org.name[0]?.toUpperCase()}
                </span>
              )}
              <span>{org.name}</span>
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}
