import {
  pb,
  type Contact,
  type Organisation,
  type Activity,
  type DashboardStats,
  type PaginatedResult,
  type App,
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

// Contacts
export async function getContacts(params: {
  page?: number
  perPage?: number
  search?: string
  status?: string
  sort?: string
} = {}): Promise<PaginatedResult<Contact>> {
  const queryParams = new URLSearchParams()
  queryParams.set('page', String(params.page || 1))
  queryParams.set('perPage', String(params.perPage || 25))
  queryParams.set('sort', params.sort || 'name')

  const filters: string[] = []
  if (params.status && params.status !== 'all') {
    filters.push(`status='${params.status}'`)
  }
  if (params.search) {
    const search = params.search.replace(/'/g, "\\'")
    filters.push(`(name~'${search}'||email~'${search}')`)
  }
  if (filters.length > 0) {
    queryParams.set('filter', filters.join('&&'))
  }

  const response = await fetch(`/api/collections/contacts/records?${queryParams}`, {
    headers: getAuthHeaders(),
  })
  if (!response.ok) throw new Error('Failed to fetch contacts')
  const data = await response.json()

  return {
    items: data.items,
    page: data.page,
    perPage: data.perPage,
    totalItems: data.totalItems,
    totalPages: data.totalPages,
  }
}

export async function getContact(id: string): Promise<Contact> {
  return fetchJSON<Contact>(`/api/collections/contacts/records/${id}`)
}

export async function createContact(data: Partial<Contact>): Promise<Contact> {
  const response = await fetch('/api/collections/contacts/records', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      Authorization: pb.authStore.token,
    },
    body: JSON.stringify(data),
  })
  if (!response.ok) {
    const error = await response.json()
    throw new Error(error.message || 'Failed to create contact')
  }
  return response.json()
}

export async function updateContact(id: string, data: Partial<Contact>): Promise<Contact> {
  const response = await fetch(`/api/collections/contacts/records/${id}`, {
    method: 'PATCH',
    headers: {
      'Content-Type': 'application/json',
      Authorization: pb.authStore.token,
    },
    body: JSON.stringify(data),
  })
  if (!response.ok) {
    const error = await response.json()
    throw new Error(error.message || 'Failed to update contact')
  }
  return response.json()
}

export async function deleteContact(id: string): Promise<void> {
  const response = await fetch(`/api/collections/contacts/records/${id}`, {
    method: 'DELETE',
    headers: { Authorization: pb.authStore.token },
  })
  if (!response.ok) throw new Error('Failed to delete contact')
}

export async function getContactActivities(id: string): Promise<Activity[]> {
  const params = new URLSearchParams({
    filter: `contact='${id}'`,
    sort: '-occurred_at',
    perPage: '50',
  })
  try {
    const data = await fetchJSON<{ items: Activity[] }>(`/api/collections/activities/records?${params}`)
    return data.items || []
  } catch {
    return []
  }
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

// Projection - not available with standalone PocketBase
export async function projectAll(): Promise<{ total: number; contacts: number; organisations: number }> {
  throw new Error('Project all requires the full Go backend')
}

// File URLs
export function getFileUrl(collectionId: string, recordId: string, filename: string): string {
  return `${pb.baseURL}/api/files/${collectionId}/${recordId}/${filename}`
}
