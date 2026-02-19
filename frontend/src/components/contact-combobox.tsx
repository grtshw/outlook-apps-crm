import { useState, useRef, useEffect, useCallback } from 'react'
import { X } from 'lucide-react'
import { cn } from '@/lib/utils'
import { Input } from '@/components/ui/input'
import type { Contact } from '@/lib/pocketbase'

interface ContactComboboxProps {
  value: string[]
  contacts: Contact[]
  onChange: (contactIds: string[]) => void
  disabled?: boolean
  placeholder?: string
}

export function ContactCombobox({
  value,
  contacts,
  onChange,
  disabled,
  placeholder = 'Search contacts...',
}: ContactComboboxProps) {
  const [inputValue, setInputValue] = useState('')
  const [isOpen, setIsOpen] = useState(false)
  const [highlightIndex, setHighlightIndex] = useState(-1)
  const containerRef = useRef<HTMLDivElement>(null)
  const inputRef = useRef<HTMLInputElement>(null)
  const listRef = useRef<HTMLUListElement>(null)

  const selectedContacts = value
    .map((id) => contacts.find((c) => c.id === id))
    .filter(Boolean) as Contact[]

  const available = contacts.filter((c) => !value.includes(c.id))
  const filtered = inputValue.trim()
    ? available.filter(
        (c) =>
          c.name.toLowerCase().includes(inputValue.toLowerCase()) ||
          c.email?.toLowerCase().includes(inputValue.toLowerCase())
      )
    : available

  const handleSelect = useCallback(
    (contact: Contact) => {
      onChange([...value, contact.id])
      setInputValue('')
      setHighlightIndex(-1)
    },
    [onChange, value]
  )

  const handleRemove = (id: string) => {
    onChange(value.filter((v) => v !== id))
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
    if (!isOpen || filtered.length === 0) {
      return
    }

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

  return (
    <div ref={containerRef} className="space-y-2">
      {/* Selected pills */}
      {selectedContacts.length > 0 && (
        <div className="flex flex-wrap gap-1.5">
          {selectedContacts.map((contact) => (
            <span
              key={contact.id}
              className="inline-flex items-center gap-1.5 rounded-full border bg-muted/50 py-1 pl-1 pr-2.5 text-sm"
            >
              {contact.avatar_thumb_url ? (
                <img
                  src={contact.avatar_thumb_url}
                  alt=""
                  className="size-5 rounded-full object-cover"
                />
              ) : (
                <span className="inline-flex items-center justify-center size-5 rounded-full bg-muted text-muted-foreground text-[10px]">
                  {contact.name[0]?.toUpperCase()}
                </span>
              )}
              <span>{contact.name}</span>
              {!disabled && (
                <button
                  type="button"
                  onClick={() => handleRemove(contact.id)}
                  className="text-muted-foreground hover:text-foreground transition-colors ml-0.5"
                >
                  <X className="size-3.5" />
                </button>
              )}
            </span>
          ))}
        </div>
      )}

      {/* Search input */}
      <div className="relative">
        <Input
          ref={inputRef}
          value={inputValue}
          onChange={handleInputChange}
          onFocus={() => setIsOpen(true)}
          onBlur={handleInputBlur}
          onKeyDown={handleKeyDown}
          placeholder={placeholder}
          disabled={disabled}
        />
        {isOpen && filtered.length > 0 && (
          <ul
            ref={listRef}
            className="absolute top-full left-0 right-0 z-[100] mt-1 max-h-48 overflow-y-auto rounded-md border bg-popover text-popover-foreground shadow-md"
          >
            {filtered.slice(0, 20).map((contact, index) => (
              <li
                key={contact.id}
                className={cn(
                  'flex items-center gap-2 px-3 py-2 text-sm cursor-pointer',
                  index === highlightIndex
                    ? 'bg-accent text-accent-foreground'
                    : 'hover:bg-accent hover:text-accent-foreground'
                )}
                onMouseDown={(e) => {
                  e.preventDefault()
                  handleSelect(contact)
                }}
                onMouseEnter={() => setHighlightIndex(index)}
              >
                {contact.avatar_thumb_url ? (
                  <img
                    src={contact.avatar_thumb_url}
                    alt=""
                    className="size-5 rounded-full object-cover"
                  />
                ) : (
                  <span className="inline-flex items-center justify-center size-5 rounded-full bg-muted text-muted-foreground text-[10px]">
                    {contact.name[0]?.toUpperCase()}
                  </span>
                )}
                <span>{contact.name}</span>
                {contact.email && (
                  <span className="text-muted-foreground truncate ml-auto text-xs">{contact.email}</span>
                )}
              </li>
            ))}
          </ul>
        )}
      </div>
    </div>
  )
}
