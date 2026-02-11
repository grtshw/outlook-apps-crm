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
  }>
  total_guests: number
  shared_by: string
  shared_at: string
}

export async function getSharedGuestList(token: string, sessionToken: string): Promise<SharedGuestListView> {
  return sessionFetch(`/api/public/guest-lists/${token}/view`, sessionToken)
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
