const COMPANY_SUFFIXES = [
  'corporation', 'corp', 'incorporated', 'inc',
  'limited', 'ltd', 'pty ltd', 'pty',
  'llc', 'llp', 'lp',
  'company', 'co',
  'group', 'holdings',
  'australia', 'aus',
]

export function extractDomain(url: string): string | null {
  if (!url) return null
  try {
    const withProtocol = url.match(/^https?:\/\//) ? url : `https://${url}`
    const hostname = new URL(withProtocol).hostname
    return hostname.replace(/^www\./, '')
  } catch {
    return null
  }
}

export function getClearbitLogoUrl(websiteUrl: string): string | null {
  const domain = extractDomain(websiteUrl)
  if (!domain) return null
  return `https://logo.clearbit.com/${domain}`
}

export function guessWebsiteUrls(companyName: string): string[] {
  if (!companyName.trim()) return []

  let cleaned = companyName.toLowerCase().trim()

  for (const suffix of COMPANY_SUFFIXES) {
    const regex = new RegExp(`\\b${suffix}\\.?\\s*$`, 'i')
    cleaned = cleaned.replace(regex, '').trim()
  }

  cleaned = cleaned.replace(/[^a-z0-9\s]/g, '').replace(/\s+/g, '')

  if (!cleaned) return []

  return [`${cleaned}.com`, `${cleaned}.com.au`]
}
