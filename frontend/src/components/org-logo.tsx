import { useState } from 'react'
import { Building2 } from 'lucide-react'
import { cn } from '@/lib/utils'
import { getClearbitLogoUrl } from '@/lib/logo'
import type { Organisation } from '@/lib/pocketbase'

interface OrgLogoProps {
  org: Pick<Organisation, 'name' | 'website' | 'logo_square_url' | 'logo_standard_url'>
  size: number | 'full'
  iconSize: number
  className?: string
  rounded?: string
}

export function OrgLogo({ org, size, iconSize, className, rounded = 'rounded' }: OrgLogoProps) {
  const [imgFailed, setImgFailed] = useState(false)

  const damUrl = org.logo_square_url || org.logo_standard_url
  const clearbitUrl = !damUrl && org.website ? getClearbitLogoUrl(org.website) : null
  const imgSrc = damUrl || (clearbitUrl && !imgFailed ? clearbitUrl : null)

  const containerStyle = size === 'full'
    ? undefined
    : { width: size, height: size }

  return (
    <div
      className={cn(
        'bg-muted flex items-center justify-center overflow-hidden shrink-0',
        rounded,
        size === 'full' && 'w-full h-full',
        !containerStyle && !size && 'h-8 w-8',
      )}
      style={containerStyle}
    >
      {imgSrc ? (
        <img
          src={imgSrc}
          alt={org.name}
          className={cn('max-w-full max-h-full object-contain', className)}
          onError={() => {
            if (!damUrl) setImgFailed(true)
          }}
        />
      ) : (
        <Building2
          style={{ width: iconSize, height: iconSize }}
          className="text-muted-foreground"
        />
      )}
    </div>
  )
}
