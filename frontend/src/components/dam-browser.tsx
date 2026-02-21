import { useState, useEffect, useRef, useCallback } from 'react'
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'
import { Sheet, SheetContent, SheetHeader, SheetTitle, SheetFooter } from '@/components/ui/sheet'
import { Search, Check, ChevronLeft, ChevronRight, Loader2 } from 'lucide-react'
import { cn } from '@/lib/utils'

const DAM_URL = import.meta.env.VITE_DAM_PUBLIC_URL || 'https://outlook-apps-dam.fly.dev'

interface DamAsset {
  id: string
  name: string
  type: string
  url: string
  thumbnailUrl: string
}

interface DamBrowserProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  onSelect: (asset: DamAsset) => void
}

export function DamBrowser({ open, onOpenChange, onSelect }: DamBrowserProps) {
  const [assets, setAssets] = useState<DamAsset[]>([])
  const [loading, setLoading] = useState(false)
  const [search, setSearch] = useState('')
  const [page, setPage] = useState(1)
  const [totalPages, setTotalPages] = useState(0)
  const [totalItems, setTotalItems] = useState(0)
  const [selected, setSelected] = useState<DamAsset | null>(null)
  const searchTimeout = useRef<ReturnType<typeof setTimeout>>(undefined)

  const fetchAssets = useCallback(async (searchQuery?: string, pageNum = 1) => {
    setLoading(true)
    try {
      const url = new URL('/api/public/assets', DAM_URL)
      if (searchQuery) url.searchParams.set('search', searchQuery)
      url.searchParams.set('type', 'image')
      url.searchParams.set('page', String(pageNum))
      url.searchParams.set('perPage', '24')

      const res = await fetch(url.toString())
      if (!res.ok) throw new Error('Failed to fetch')
      const data = await res.json()

      setAssets(
        data.items.map((item: { id: string; name: string; type: string }) => ({
          id: item.id,
          name: item.name,
          type: item.type,
          url: `${DAM_URL}/api/assets/${item.id}/image/medium`,
          thumbnailUrl: `${DAM_URL}/api/assets/${item.id}/image/thumb`,
        }))
      )
      setTotalPages(data.totalPages || 0)
      setTotalItems(data.totalItems || 0)
    } catch {
      setAssets([])
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    if (open) {
      setSearch('')
      setPage(1)
      setSelected(null)
      fetchAssets(undefined, 1)
    }
  }, [open, fetchAssets])

  const handleSearchChange = (value: string) => {
    setSearch(value)
    clearTimeout(searchTimeout.current)
    searchTimeout.current = setTimeout(() => {
      setPage(1)
      fetchAssets(value || undefined, 1)
    }, 300)
  }

  const handlePageChange = (newPage: number) => {
    setPage(newPage)
    fetchAssets(search || undefined, newPage)
  }

  const handleConfirm = () => {
    if (!selected) return
    // Mark as used (fire and forget)
    fetch(`${DAM_URL}/api/public/assets/${selected.id}/mark-used`, { method: 'POST' }).catch(() => {})
    onSelect(selected)
    onOpenChange(false)
  }

  const perPage = 24
  const startItem = (page - 1) * perPage + 1
  const endItem = Math.min(page * perPage, totalItems)

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent>
        <SheetHeader>
          <SheetTitle>Select image from DAM</SheetTitle>
        </SheetHeader>

        <div className="flex-1 overflow-hidden flex flex-col p-6 gap-4">
          <div className="relative shrink-0">
            <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground" />
            <Input
              value={search}
              onChange={(e) => handleSearchChange(e.target.value)}
              placeholder="Search assets..."
              className="pl-9"
            />
          </div>

          {loading ? (
            <div className="flex-1 flex items-center justify-center">
              <Loader2 className="h-5 w-5 animate-spin text-muted-foreground" />
            </div>
          ) : assets.length === 0 ? (
            <div className="flex-1 flex items-center justify-center">
              <p className="text-sm text-muted-foreground">
                {search ? 'No assets found. Try a different search.' : 'No assets available.'}
              </p>
            </div>
          ) : (
            <>
              {totalItems > 0 && (
                <p className="text-xs text-muted-foreground shrink-0">
                  {startItem}–{endItem} of {totalItems} assets
                </p>
              )}
              <div className="flex-1 overflow-y-auto">
                <div className="grid grid-cols-3 gap-2">
                  {assets.map((asset) => {
                    const isSelected = selected?.id === asset.id
                    return (
                      <button
                        key={asset.id}
                        type="button"
                        className={cn(
                          'relative aspect-[4/3] rounded overflow-hidden border-2 transition-colors cursor-pointer group',
                          isSelected ? 'border-foreground' : 'border-transparent hover:border-foreground/20'
                        )}
                        onClick={() => setSelected(isSelected ? null : asset)}
                      >
                        <img
                          src={asset.thumbnailUrl}
                          alt={asset.name}
                          className="absolute inset-0 w-full h-full object-cover"
                          loading="lazy"
                        />
                        {isSelected && (
                          <div className="absolute top-1.5 right-1.5 w-5 h-5 bg-foreground text-background rounded-full flex items-center justify-center">
                            <Check className="w-3 h-3" />
                          </div>
                        )}
                        <div className="absolute inset-x-0 bottom-0 bg-gradient-to-t from-black/60 to-transparent p-1.5 opacity-0 group-hover:opacity-100 transition-opacity">
                          <p className="text-[10px] text-white truncate">{asset.name}</p>
                        </div>
                      </button>
                    )
                  })}
                </div>
              </div>
            </>
          )}

          {totalPages > 1 && (
            <div className="flex items-center justify-center gap-3 shrink-0 pt-2 border-t border-border">
              <Button
                variant="outline"
                size="sm"
                disabled={page <= 1}
                onClick={() => handlePageChange(page - 1)}
              >
                <ChevronLeft className="w-4 h-4" />
              </Button>
              <span className="text-xs text-muted-foreground">
                Page {page} of {totalPages}
              </span>
              <Button
                variant="outline"
                size="sm"
                disabled={page >= totalPages}
                onClick={() => handlePageChange(page + 1)}
              >
                <ChevronRight className="w-4 h-4" />
              </Button>
            </div>
          )}
        </div>

        <SheetFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <Button onClick={handleConfirm} disabled={!selected}>
            Select
          </Button>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  )
}
