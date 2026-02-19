import { useState, useEffect, useRef } from 'react'
import type { ProgramItem, Contact } from '@/lib/pocketbase'
import { getContacts } from '@/lib/api'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { RichTextEditor } from '@/components/rich-text-editor'
import { Avatar, AvatarFallback, AvatarImage } from '@/components/ui/avatar'
import { Plus, Trash2, ChevronDown, ChevronUp, X, Search } from 'lucide-react'
import { cn } from '@/lib/utils'

interface ProgramEditorProps {
  items: ProgramItem[]
  onChange: (items: ProgramItem[]) => void
}

export function ProgramEditor({ items, onChange }: ProgramEditorProps) {
  const [expandedIndex, setExpandedIndex] = useState<number | null>(null)

  const addItem = () => {
    onChange([...items, { time: '', title: '' }])
    setExpandedIndex(items.length)
  }

  const removeItem = (index: number) => {
    onChange(items.filter((_, i) => i !== index))
    if (expandedIndex === index) setExpandedIndex(null)
  }

  const updateItem = (index: number, updates: Partial<ProgramItem>) => {
    onChange(items.map((item, i) => (i === index ? { ...item, ...updates } : item)))
  }

  const moveItem = (from: number, to: number) => {
    if (to < 0 || to >= items.length) return
    const updated = [...items]
    const [moved] = updated.splice(from, 1)
    updated.splice(to, 0, moved)
    onChange(updated)
    setExpandedIndex(to)
  }

  return (
    <div className="space-y-2">
      {items.map((item, i) => (
        <div key={i} className="border rounded-lg">
          {/* Collapsed row */}
          <div className="flex items-center gap-2 p-3">
            <div className="flex flex-col gap-0.5">
              <button
                type="button"
                onClick={() => moveItem(i, i - 1)}
                disabled={i === 0}
                className="text-muted-foreground hover:text-foreground disabled:opacity-20 cursor-pointer"
              >
                <ChevronUp className="h-3 w-3" />
              </button>
              <button
                type="button"
                onClick={() => moveItem(i, i + 1)}
                disabled={i === items.length - 1}
                className="text-muted-foreground hover:text-foreground disabled:opacity-20 cursor-pointer"
              >
                <ChevronDown className="h-3 w-3" />
              </button>
            </div>
            <span className="text-xs text-muted-foreground w-[60px] shrink-0 font-mono">
              {item.time || '--:--'}
            </span>
            <span className="text-sm flex-1 truncate">
              {item.title || 'Untitled'}
            </span>
            {item.speaker_name && (
              <span className="text-xs text-muted-foreground truncate max-w-[120px]">
                {item.speaker_name}
              </span>
            )}
            <Button
              variant="ghost"
              size="icon"
              className="h-7 w-7"
              onClick={() => setExpandedIndex(expandedIndex === i ? null : i)}
            >
              <ChevronDown
                className={cn(
                  'h-4 w-4 transition-transform',
                  expandedIndex === i && 'rotate-180'
                )}
              />
            </Button>
            <Button
              variant="ghost"
              size="icon"
              className="h-7 w-7 text-muted-foreground hover:text-destructive"
              onClick={() => removeItem(i)}
            >
              <Trash2 className="h-3.5 w-3.5" />
            </Button>
          </div>

          {/* Expanded editor */}
          {expandedIndex === i && (
            <div className="border-t p-3 space-y-3">
              <div className="grid grid-cols-2 gap-3">
                <div>
                  <label className="block text-xs text-muted-foreground mb-1">Time</label>
                  <Input
                    value={item.time}
                    onChange={(e) => updateItem(i, { time: e.target.value })}
                    placeholder="5:30PM"
                  />
                </div>
                <div>
                  <label className="block text-xs text-muted-foreground mb-1">Title</label>
                  <Input
                    value={item.title}
                    onChange={(e) => updateItem(i, { title: e.target.value })}
                    placeholder="Session title"
                  />
                </div>
              </div>

              {/* Speaker — contact search */}
              <div>
                <label className="block text-xs text-muted-foreground mb-1">Speaker</label>
                {item.speaker_contact_id ? (
                  <div className="flex items-center gap-2 p-2 border rounded-md bg-muted/30">
                    {item.speaker_image_url && (
                      <Avatar className="h-7 w-7">
                        <AvatarImage src={item.speaker_image_url} />
                        <AvatarFallback>{(item.speaker_name || '?')[0]}</AvatarFallback>
                      </Avatar>
                    )}
                    <div className="flex-1 min-w-0">
                      <p className="text-sm truncate">{item.speaker_name}</p>
                      {item.speaker_org && (
                        <p className="text-xs text-muted-foreground truncate">{item.speaker_org}</p>
                      )}
                    </div>
                    <Button
                      variant="ghost"
                      size="icon"
                      className="h-6 w-6 shrink-0"
                      onClick={() => updateItem(i, {
                        speaker_contact_id: undefined,
                        speaker_name: undefined,
                        speaker_org: undefined,
                        speaker_image_url: undefined,
                      })}
                    >
                      <X className="h-3.5 w-3.5" />
                    </Button>
                  </div>
                ) : (
                  <ContactSearch
                    onSelect={(contact) => updateItem(i, {
                      speaker_contact_id: contact.id,
                      speaker_name: contact.name,
                      speaker_org: contact.organisation_name || undefined,
                      speaker_image_url: contact.avatar_small_url || contact.avatar_thumb_url || contact.avatar_url || undefined,
                    })}
                  />
                )}
              </div>

              <div>
                <label className="block text-xs text-muted-foreground mb-1">Description</label>
                <RichTextEditor
                  content={item.description || ''}
                  onChange={(html) => updateItem(i, { description: html })}
                  placeholder="Session description..."
                  minHeight={80}
                />
              </div>
            </div>
          )}
        </div>
      ))}

      <Button type="button" variant="outline" onClick={addItem} className="w-full">
        <Plus className="h-4 w-4 mr-1" /> Add program item
      </Button>
    </div>
  )
}

function ContactSearch({ onSelect }: { onSelect: (contact: Contact) => void }) {
  const [query, setQuery] = useState('')
  const [results, setResults] = useState<Contact[]>([])
  const [loading, setLoading] = useState(false)
  const [open, setOpen] = useState(false)
  const debounceRef = useRef<ReturnType<typeof setTimeout>>(null)
  const containerRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (query.length < 2) {
      setResults([])
      setOpen(false)
      return
    }
    if (debounceRef.current) clearTimeout(debounceRef.current)
    debounceRef.current = setTimeout(async () => {
      setLoading(true)
      try {
        const res = await getContacts({ search: query, perPage: 8, status: 'active' })
        setResults(res.items)
        setOpen(true)
      } catch {
        setResults([])
      } finally {
        setLoading(false)
      }
    }, 300)
    return () => { if (debounceRef.current) clearTimeout(debounceRef.current) }
  }, [query])

  // Close dropdown on outside click
  useEffect(() => {
    const handler = (e: MouseEvent) => {
      if (containerRef.current && !containerRef.current.contains(e.target as Node)) {
        setOpen(false)
      }
    }
    document.addEventListener('mousedown', handler)
    return () => document.removeEventListener('mousedown', handler)
  }, [])

  return (
    <div ref={containerRef} className="relative">
      <div className="relative">
        <Search className="absolute left-2.5 top-1/2 -translate-y-1/2 h-3.5 w-3.5 text-muted-foreground" />
        <Input
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          placeholder="Search contacts..."
          className="pl-8"
          onFocus={() => results.length > 0 && setOpen(true)}
        />
      </div>
      {open && results.length > 0 && (
        <div className="absolute z-50 top-full mt-1 w-full bg-popover border rounded-md shadow-md max-h-[200px] overflow-y-auto">
          {results.map((contact) => (
            <button
              key={contact.id}
              type="button"
              className="w-full flex items-center gap-2 px-3 py-2 hover:bg-accent text-left cursor-pointer"
              onClick={() => {
                onSelect(contact)
                setQuery('')
                setOpen(false)
              }}
            >
              <Avatar className="h-6 w-6">
                <AvatarImage src={contact.avatar_small_url || contact.avatar_thumb_url || contact.avatar_url} />
                <AvatarFallback className="text-[10px]">{(contact.first_name || '?')[0]}</AvatarFallback>
              </Avatar>
              <div className="flex-1 min-w-0">
                <p className="text-sm truncate">{contact.name}</p>
                {contact.organisation_name && (
                  <p className="text-xs text-muted-foreground truncate">{contact.organisation_name}</p>
                )}
              </div>
            </button>
          ))}
        </div>
      )}
      {open && loading && (
        <div className="absolute z-50 top-full mt-1 w-full bg-popover border rounded-md shadow-md p-3 text-xs text-muted-foreground text-center">
          Searching...
        </div>
      )}
    </div>
  )
}
