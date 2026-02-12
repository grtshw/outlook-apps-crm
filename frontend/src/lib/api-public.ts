// Public (unauthenticated) API functions for shared guest list access

async function publicFetch<T>(url: string, init?: RequestInit): Promise<T> {
  const res = await fetch(url, init)
  if (!res.ok) {
    const body = await res.json().catch(() => ({}))
    throw new Error(body.error || `Request failed: ${res.status}`)
  }
  return res.json()
}

function sessionFetch<T>(url: string, sessionToken: string, init?: RequestInit): Promise<T> {
  return publicFetch<T>(url, {
    ...init,
    headers: { Authorization: `Bearer ${sessionToken}`, ...init?.headers },
  })
}

export interface ShareInfo {
  list_name: string
  event_name: string
  recipient_name: string
  masked_email: string
  requires_verification: boolean
}

export async function getShareInfo(token: string): Promise<ShareInfo> {
  return publicFetch(`/api/public/guest-lists/${token}`)
}

export async function sendOTP(token: string): Promise<{ sent: boolean; email: string; expires: number }> {
  return publicFetch(`/api/public/guest-lists/${token}/send-otp`, { method: 'POST' })
}

export async function verifyOTP(
  token: string,
  code: string
): Promise<{ verified: boolean; session_token: string; expires_in: number }> {
  return publicFetch(`/api/public/guest-lists/${token}/verify`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ code }),
  })
}

export interface SharedGuestListView {
  list_name: string
  event_name: string
  items: Array<{
    id: string
    name: string
    role: string
    company: string
    invite_round: string
    linkedin: string
    city: string
    degrees: string
    relationship: number
    notes: string
    client_notes: string
    rsvp_status: string
  }>
  total_guests: number
  shared_by: string
  shared_at: string
}

export async function getSharedGuestList(token: string, sessionToken: string): Promise<SharedGuestListView> {
  return sessionFetch(`/api/public/guest-lists/${token}/view`, sessionToken)
}

// RSVP public endpoints
export interface RSVPInfo {
  type: 'personal' | 'generic'
  list_name: string
  event_name: string
  description: string
  prefilled_first_name: string
  prefilled_last_name: string
  prefilled_email: string
  prefilled_phone: string
  prefilled_dietary_requirements: string[]
  prefilled_dietary_requirements_other: string
  prefilled_accessibility_requirements: string[]
  prefilled_accessibility_requirements_other: string
  already_responded: boolean
  rsvp_status: 'accepted' | 'declined' | ''
  rsvp_plus_one: boolean
  rsvp_plus_one_name: string
  rsvp_plus_one_last_name: string
  rsvp_plus_one_job_title: string
  rsvp_plus_one_company: string
  rsvp_plus_one_email: string
  rsvp_plus_one_dietary: string
  rsvp_comments: string
}

export async function getRSVPInfo(token: string): Promise<RSVPInfo> {
  return publicFetch(`/api/public/rsvp/${token}`)
}

export interface RSVPSubmission {
  first_name: string
  last_name: string
  email: string
  phone?: string
  dietary_requirements?: string[]
  dietary_requirements_other?: string
  accessibility_requirements?: string[]
  accessibility_requirements_other?: string
  plus_one?: boolean
  plus_one_name?: string
  plus_one_last_name?: string
  plus_one_job_title?: string
  plus_one_company?: string
  plus_one_email?: string
  plus_one_dietary?: string
  response: 'accepted' | 'declined'
  invited_by?: string
  comments?: string
}

export async function submitRSVP(token: string, data: RSVPSubmission): Promise<{ message: string }> {
  return publicFetch(`/api/public/rsvp/${token}`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(data),
  })
}

export async function updateSharedGuestListItem(
  token: string,
  itemId: string,
  data: { invite_round?: string; client_notes?: string },
  sessionToken: string
): Promise<{ message: string }> {
  return sessionFetch(`/api/public/guest-lists/${token}/items/${itemId}`, sessionToken, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(data),
  })
}
