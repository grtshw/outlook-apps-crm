import {
  pb,
  type Contact,
  type ContactLink,
  type Organisation,
  type Activity,
  type DashboardStats,
  type PaginatedResult,
  type App,
  type EventProjection,
  type GuestList,
  type GuestListItem,
  type GuestListShare,
  type Theme,
} from './pocketbase'
import type { RecordModel } from 'pocketbase'

function getAuthHeaders(): HeadersInit {
  const token = pb.authStore.token
  return token ? { Authorization: token } : {}
}

// Standard fetch helper — use for new API functions
export async function fetchJSON<T>(url: string, init?: RequestInit): Promise<T> {
  const res = await fetch(url, {
    ...init,
    headers: { ...getAuthHeaders(), ...init?.headers },
  })
  if (!res.ok) {
    const body = await res.json().catch(() => ({}))
    throw new Error(body.error || `Request failed: ${res.status}`)
  }
  return res.json()
}

// Dashboard - compute stats from collections
async function fetchCount(collection: string, filter?: string): Promise<number> {
  const params = new URLSearchParams({ page: '1', perPage: '1' })
  if (filter) params.set('filter', filter)
  try {
    const response = await fetch(`/api/collections/${collection}/records?${params}`, {
      headers: getAuthHeaders(),
    })
    if (!response.ok) return 0
    const data = await response.json()
    return data.totalItems || 0
  } catch {
    return 0
  }
}

export async function getDashboardStats(): Promise<DashboardStats> {
  const [contactsActive, contactsInactive, contactsArchived, orgsActive, orgsArchived, recentActivities] = await Promise.all([
    fetchCount('contacts', "status='active'"),
    fetchCount('contacts', "status='inactive'"),
    fetchCount('contacts', "status='archived'"),
    fetchCount('organisations', "status='active'"),
    fetchCount('organisations', "status='archived'"),
    fetchCount('activities', `occurred_at>='${new Date(Date.now() - 30 * 24 * 60 * 60 * 1000).toISOString()}'`),
  ])

  return {
    contacts: {
      active: contactsActive,
      inactive: contactsInactive,
      archived: contactsArchived,
      total: contactsActive + contactsInactive + contactsArchived,
    },
    organisations: {
      active: orgsActive,
      archived: orgsArchived,
      total: orgsActive + orgsArchived,
    },
    recent_activities: recentActivities,
  }
}

// Contacts — use custom endpoints that handle PII decryption
export async function getContacts(params: {
  page?: number
  perPage?: number
  search?: string
  status?: string
  sort?: string
  humanitix_event?: string
} = {}): Promise<PaginatedResult<Contact>> {
  const queryParams = new URLSearchParams()
  queryParams.set('page', String(params.page || 1))
  queryParams.set('perPage', String(params.perPage || 25))
  queryParams.set('sort', params.sort || 'name')
  if (params.status && params.status !== 'all') {
    queryParams.set('status', params.status)
  }
  if (params.search) {
    queryParams.set('search', params.search)
  }
  if (params.humanitix_event) {
    queryParams.set('humanitix_event', params.humanitix_event)
  }

  return fetchJSON<PaginatedResult<Contact>>(`/api/contacts?${queryParams}`)
}

export async function getContact(id: string): Promise<Contact> {
  return fetchJSON<Contact>(`/api/contacts/${id}`)
}

export async function createContact(data: Partial<Contact>): Promise<Contact> {
  return fetchJSON<Contact>('/api/contacts', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(data),
  })
}

export async function updateContact(id: string, data: Partial<Contact>): Promise<Contact> {
  return fetchJSON<Contact>(`/api/contacts/${id}`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(data),
  })
}

export async function deleteContact(id: string): Promise<void> {
  await fetchJSON<{ message: string }>(`/api/contacts/${id}`, {
    method: 'DELETE',
  })
}

export async function getContactActivities(id: string): Promise<Activity[]> {
  try {
    const data = await fetchJSON<Activity[]>(`/api/contacts/${id}/activities`)
    return data || []
  } catch {
    return []
  }
}

// Merge contacts
export interface MergeContactsRequest {
  primary_id: string
  merged_ids: string[]
  field_selections: Record<string, string>
  merged_roles: string[]
  merged_tags: string[]
  merged_dietary_requirements: string[]
  merged_accessibility_requirements: string[]
}

export interface MergeContactsResponse {
  id: string
  activities_reassigned: number
  contacts_deleted: number
}

export async function mergeContacts(data: MergeContactsRequest): Promise<MergeContactsResponse> {
  return fetchJSON<MergeContactsResponse>('/api/contacts/merge', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(data),
  })
}

// Contact links
export async function getContactLinks(contactId: string): Promise<ContactLink[]> {
  return fetchJSON<ContactLink[]>(`/api/contacts/${contactId}/links`)
}

export async function createContactLink(contactId: string, targetContactId: string, notes?: string): Promise<{ id: string }> {
  return fetchJSON<{ id: string }>(`/api/contacts/${contactId}/links`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ target_contact_id: targetContactId, notes }),
  })
}

export async function deleteContactLink(linkId: string): Promise<void> {
  await fetchJSON<{ message: string }>(`/api/contact-links/${linkId}`, {
    method: 'DELETE',
  })
}

// Organisations
export async function getOrganisations(params: {
  page?: number
  perPage?: number
  search?: string
  status?: string
  sort?: string
} = {}): Promise<PaginatedResult<Organisation>> {
  const queryParams = new URLSearchParams()
  queryParams.set('page', String(params.page || 1))
  queryParams.set('perPage', String(params.perPage || 24))
  queryParams.set('sort', params.sort || 'name')

  const filters: string[] = []
  if (params.status && params.status !== 'all') {
    filters.push(`status='${params.status}'`)
  }
  if (params.search) {
    const search = params.search.replace(/'/g, "\\'")
    filters.push(`name~'${search}'`)
  }
  if (filters.length > 0) {
    queryParams.set('filter', filters.join('&&'))
  }

  const response = await fetch(`/api/collections/organisations/records?${queryParams}`, {
    headers: { Authorization: pb.authStore.token },
  })
  if (!response.ok) throw new Error('Failed to fetch organisations')
  const data = await response.json()

  return {
    items: data.items,
    page: data.page,
    perPage: data.perPage,
    totalItems: data.totalItems,
    totalPages: data.totalPages,
  }
}

export async function getOrganisation(id: string): Promise<Organisation> {
  return fetchJSON<Organisation>(`/api/collections/organisations/records/${id}`)
}

export async function createOrganisation(data: Partial<Organisation>): Promise<Organisation> {
  const response = await fetch('/api/collections/organisations/records', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      Authorization: pb.authStore.token,
    },
    body: JSON.stringify(data),
  })
  if (!response.ok) {
    const error = await response.json()
    throw new Error(error.message || 'Failed to create organisation')
  }
  return response.json()
}

export async function updateOrganisation(id: string, data: Partial<Organisation>): Promise<Organisation> {
  const response = await fetch(`/api/collections/organisations/records/${id}`, {
    method: 'PATCH',
    headers: {
      'Content-Type': 'application/json',
      Authorization: pb.authStore.token,
    },
    body: JSON.stringify(data),
  })
  if (!response.ok) {
    const error = await response.json()
    throw new Error(error.message || 'Failed to update organisation')
  }
  return response.json()
}

export async function deleteOrganisation(id: string): Promise<void> {
  const response = await fetch(`/api/collections/organisations/records/${id}`, {
    method: 'DELETE',
    headers: { Authorization: pb.authStore.token },
  })
  if (!response.ok) throw new Error('Failed to delete organisation')
}

// Activities
export async function getActivities(params: {
  page?: number
  perPage?: number
  source_app?: string
  type?: string
} = {}): Promise<PaginatedResult<Activity>> {
  const queryParams = new URLSearchParams()
  queryParams.set('page', String(params.page || 1))
  queryParams.set('perPage', String(params.perPage || 25))
  queryParams.set('sort', '-occurred_at')

  const filters: string[] = []
  if (params.source_app) {
    filters.push(`source_app='${params.source_app}'`)
  }
  if (params.type) {
    filters.push(`type='${params.type}'`)
  }
  if (filters.length > 0) {
    queryParams.set('filter', filters.join('&&'))
  }

  const response = await fetch(`/api/collections/activities/records?${queryParams}`, {
    headers: { Authorization: pb.authStore.token },
  })
  if (!response.ok) throw new Error('Failed to fetch activities')
  const data = await response.json()

  return {
    items: data.items,
    page: data.page,
    perPage: data.perPage,
    totalItems: data.totalItems,
    totalPages: data.totalPages,
  }
}

// Apps
export async function loadApps(): Promise<App[]> {
  try {
    const settings = await pb.collection('app_settings').getFullList<RecordModel>()
    const appsString = settings.find((s) => s.key === 'apps')?.value
    if (appsString) {
      return JSON.parse(appsString as string)
    }
  } catch {
    // Ignore errors loading apps
  }
  return []
}

// ── Projections ──

export async function projectAll(): Promise<{
  status: string
  projection_id: string
  total: number
  counts: Record<string, number>
  consumers: string[]
}> {
  return fetchJSON('/api/project-all', { method: 'POST' })
}

export async function getProjectionLogs(): Promise<{
  logs: Array<{
    id: string
    created_at: string
    record_count: number
    consumers: Array<{
      name: string
      status: 'pending' | 'ok' | 'error' | 'partial'
      message?: string
      records_processed?: number
      received_at?: string
    }>
  }>
}> {
  return fetchJSON('/api/projections/logs')
}

export async function getProjectionConsumers(): Promise<{
  consumers: Array<{
    id: string
    name: string
    app_id: string
    enabled: boolean
    last_consumption?: string
    last_status?: string
    last_message?: string
  }>
}> {
  return fetchJSON('/api/projection-consumers')
}

export async function toggleProjectionConsumer(id: string): Promise<{ message: string; enabled: boolean }> {
  return fetchJSON(`/api/projection-consumers/${id}/toggle`, { method: 'PATCH' })
}

export async function getProjectionProgress(projectionId: string): Promise<{
  projection_id: string
  total: number
  completed: number
  consumers: Array<{
    name: string
    status: 'pending' | 'ok' | 'error' | 'partial'
    message?: string
    records_processed?: number
    received_at?: string
  }>
}> {
  return fetchJSON(`/api/projections/${projectionId}/progress`)
}

// ── Event Projections ──

export async function getEventProjections(params?: { search?: string }): Promise<{ items: EventProjection[] }> {
  const queryParams = new URLSearchParams()
  if (params?.search) queryParams.set('search', params.search)
  return fetchJSON(`/api/event-projections?${queryParams}`)
}

// ── Guest Lists ──

export async function getGuestLists(params?: {
  status?: string
  search?: string
}): Promise<{ items: GuestList[] }> {
  const queryParams = new URLSearchParams()
  if (params?.status && params.status !== 'all') queryParams.set('status', params.status)
  if (params?.search) queryParams.set('search', params.search)
  return fetchJSON(`/api/guest-lists?${queryParams}`)
}

export async function getGuestList(id: string): Promise<GuestList> {
  return fetchJSON(`/api/guest-lists/${id}`)
}

export async function createGuestList(data: {
  name: string
  description?: string
  event_projection?: string
  status?: string
}): Promise<{ id: string; name: string; status: string }> {
  return fetchJSON('/api/guest-lists', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(data),
  })
}

export async function updateGuestList(
  id: string,
  data: Partial<{
    name: string; description: string; event_projection: string; status: string;
    landing_enabled: boolean; landing_headline: string; landing_description: string;
    landing_image_url: string; landing_program: import('./pocketbase').ProgramItem[];
    landing_content: string; program_description: string; program_title: string;
    event_date: string; event_time: string; event_location: string; event_location_address: string;
    organisation: string;
    rsvp_bcc_contacts: string[];
    theme: string;
  }>
): Promise<{ message: string }> {
  return fetchJSON(`/api/guest-lists/${id}`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(data),
  })
}

export async function deleteGuestList(id: string): Promise<{ message: string }> {
  return fetchJSON(`/api/guest-lists/${id}`, { method: 'DELETE' })
}

export async function deleteGuestListImage(id: string): Promise<{ message: string }> {
  return fetchJSON(`/api/guest-lists/${id}/image`, { method: 'DELETE' })
}

export async function cloneGuestList(
  id: string,
  data?: { name?: string; description?: string; event_projection?: string; status?: string }
): Promise<{ id: string; name: string; items_cloned: number }> {
  return fetchJSON(`/api/guest-lists/${id}/clone`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(data || {}),
  })
}

// ── Guest List Items ──

export async function getGuestListItems(listId: string): Promise<{ items: GuestListItem[] }> {
  return fetchJSON(`/api/guest-lists/${listId}/items`)
}

export async function addGuestListItem(
  listId: string,
  data: { contact_id: string; invite_round?: string }
): Promise<{ id: string; contact_id: string; contact_name: string }> {
  return fetchJSON(`/api/guest-lists/${listId}/items`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(data),
  })
}

export async function bulkAddGuestListItems(
  listId: string,
  data: { contact_ids: string[]; invite_round?: string }
): Promise<{ added: number }> {
  return fetchJSON(`/api/guest-lists/${listId}/items/bulk`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(data),
  })
}

export async function updateGuestListItem(
  itemId: string,
  data: Partial<{ invite_round: string; invite_status: string; notes: string; sort_order: number }>
): Promise<{ message: string }> {
  return fetchJSON(`/api/guest-list-items/${itemId}`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(data),
  })
}

export async function deleteGuestListItem(itemId: string): Promise<{ message: string }> {
  return fetchJSON(`/api/guest-list-items/${itemId}`, { method: 'DELETE' })
}

// ── Guest List Shares ──

export async function getGuestListShares(listId: string): Promise<{ items: GuestListShare[] }> {
  return fetchJSON(`/api/guest-lists/${listId}/shares`)
}

export async function createGuestListShare(
  listId: string,
  data: { recipient_email: string; recipient_name?: string }
): Promise<{ id: string; token: string; share_url: string; expires_at: string }> {
  return fetchJSON(`/api/guest-lists/${listId}/shares`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(data),
  })
}

export async function revokeGuestListShare(shareId: string): Promise<{ message: string }> {
  return fetchJSON(`/api/guest-list-shares/${shareId}`, { method: 'DELETE' })
}

// RSVP admin endpoints
export async function toggleGuestListRSVP(
  listId: string,
  enabled: boolean
): Promise<{ rsvp_enabled: boolean; rsvp_generic_url: string }> {
  return fetchJSON(`/api/guest-lists/${listId}/rsvp/enable`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ enabled }),
  })
}

export async function sendRSVPInvites(
  listId: string,
  itemIds?: string[]
): Promise<{ sent: number; skipped: number }> {
  return fetchJSON(`/api/guest-lists/${listId}/rsvp/send-invites`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ item_ids: itemIds || [] }),
  })
}

// ── Mailchimp Settings ──

export interface MailchimpStatus {
  configured: boolean
  has_list: boolean
  list_id: string
}

export interface MailchimpList {
  id: string
  name: string
  member_count: number
}

export interface MailchimpMergeField {
  tag: string
  name: string
  type: string
}

export interface MergeFieldMapping {
  mailchimp_tag: string
  crm_field: string
}

export interface MailchimpSettings {
  id?: string
  list_id: string
  list_name: string
  merge_field_mappings: MergeFieldMapping[]
}

export async function getMailchimpStatus(): Promise<MailchimpStatus> {
  return fetchJSON('/api/admin/mailchimp/status')
}

export async function getMailchimpLists(): Promise<{ lists: MailchimpList[] }> {
  return fetchJSON('/api/admin/mailchimp/lists')
}

export async function getMailchimpMergeFields(listId: string): Promise<{ merge_fields: MailchimpMergeField[] }> {
  return fetchJSON(`/api/admin/mailchimp/lists/${listId}/merge-fields`)
}

export async function getMailchimpSettings(): Promise<MailchimpSettings> {
  return fetchJSON('/api/admin/mailchimp/settings')
}

export async function saveMailchimpSettings(data: {
  list_id: string
  list_name: string
  merge_field_mappings: MergeFieldMapping[]
}): Promise<MailchimpSettings> {
  return fetchJSON('/api/admin/mailchimp/settings', {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(data),
  })
}

// Themes CRUD
export async function getThemes(): Promise<{ items: Theme[] }> {
  return fetchJSON('/api/themes')
}

export async function getTheme(id: string): Promise<Theme> {
  return fetchJSON(`/api/themes/${id}`)
}

export async function createTheme(data: Partial<Theme>): Promise<Theme> {
  return fetchJSON('/api/themes', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(data),
  })
}

export async function updateTheme(id: string, data: Partial<Theme>): Promise<Theme> {
  return fetchJSON(`/api/themes/${id}`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(data),
  })
}

export async function deleteTheme(id: string): Promise<{ message: string }> {
  return fetchJSON(`/api/themes/${id}`, { method: 'DELETE' })
}

// File URLs
export function getFileUrl(collectionId: string, recordId: string, filename: string): string {
  return `${pb.baseURL}/api/files/${collectionId}/${recordId}/${filename}`
}
