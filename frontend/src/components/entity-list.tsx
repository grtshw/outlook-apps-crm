import type { ReactNode } from 'react'
import { Card } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'

interface EntityListProps<T> {
  items: T[]
  isLoading: boolean
  layout: 'list' | 'cards'
  getName: (item: T) => string
  renderListItem: (item: T) => ReactNode
  renderCard: (item: T) => ReactNode
  onItemClick: (item: T) => void
  emptyMessage?: string
}

export function EntityList<T extends { id: string }>({
  items,
  isLoading,
  layout,
  getName,
  renderListItem,
  renderCard,
  onItemClick,
  emptyMessage = 'No results found',
}: EntityListProps<T>) {
  if (isLoading) {
    return layout === 'list' ? (
      <div className="space-y-2">
        {Array.from({ length: 10 }).map((_, i) => (
          <Skeleton key={i} className="h-14 rounded-lg" />
        ))}
      </div>
    ) : (
      <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-4 xl:grid-cols-6 gap-4">
        {Array.from({ length: 12 }).map((_, i) => (
          <Skeleton key={i} className="aspect-square rounded-lg" />
        ))}
      </div>
    )
  }

  if (items.length === 0) {
    return (
      <p className="py-8 text-center text-muted-foreground">{emptyMessage}</p>
    )
  }

  if (layout === 'cards') {
    return (
      <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-4 xl:grid-cols-6 gap-4">
        {items.map((item) => (
          <Card
            key={item.id}
            className="cursor-pointer hover:shadow-md transition-shadow"
            onClick={() => onItemClick(item)}
          >
            {renderCard(item)}
          </Card>
        ))}
      </div>
    )
  }

  // List view with alphabetical dividers
  const groups: { letter: string; items: T[] }[] = []
  for (const item of items) {
    const letter = (getName(item)?.[0] || '#').toUpperCase()
    const last = groups[groups.length - 1]
    if (last && last.letter === letter) {
      last.items.push(item)
    } else {
      groups.push({ letter, items: [item] })
    }
  }

  return (
    <div className="space-y-1">
      {groups.map((group) => (
        <div key={group.letter}>
          <div className="px-2 py-1.5 text-xs text-muted-foreground tracking-widest uppercase">
            {group.letter}
          </div>
          {group.items.map((item) => (
            <div
              key={item.id}
              className="flex items-center gap-3 px-2 py-2 rounded-lg cursor-pointer hover:bg-accent transition-colors"
              onClick={() => onItemClick(item)}
            >
              {renderListItem(item)}
            </div>
          ))}
        </div>
      ))}
    </div>
  )
}
