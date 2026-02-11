import PocketBase, { type RecordModel } from 'pocketbase'

export const pb = new PocketBase('/')

// Types
export interface User extends RecordModel {
  email: string
  name: string
  role: 'admin' | 'viewer'
  avatar?: string
  avatarURL?: string
}

export type ContactRole = 'presenter' | 'speaker' | 'sponsor' | 'judge' | 'attendee' | 'staff' | 'volunteer'

export interface Contact extends RecordModel {
  email: string
  name: string
  phone?: string
  pronouns?: string
  bio?: string
  job_title?: string
  linkedin?: string
  instagram?: string
  website?: string
  location?: string
  do_position?: string
  avatar?: string
  avatar_url?: string
  avatar_thumb_url?: string
  avatar_small_url?: string
  avatar_original_url?: string
  organisation?: string
  organisation_id?: string
  organisation_name?: string
  tags?: string[]
  roles?: ContactRole[]
  status: 'active' | 'inactive' | 'archived'
  source?: string
  source_ids?: Record<string, string>
  degrees?: '1st' | '2nd' | '3rd'
  relationship?: number
  notes?: string
}

export interface Organisation extends RecordModel {
  name: string
  website?: string
  linkedin?: string
  description_short?: string
  description_medium?: string
  description_long?: string
  logo_square?: string
  logo_standard?: string
  logo_inverted?: string
  logo_square_url?: string
  logo_standard_url?: string
  logo_inverted_url?: string
  contacts?: Array<{ name: string; linkedin?: string; email?: string }>
  tags?: string[]
  status: 'active' | 'archived'
  source?: string
  source_ids?: { presentations?: string; awards?: string; events?: string }
}

export interface Activity extends RecordModel {
  type: string
  title?: string
  contact?: string
  organisation?: string
  source_app: string
  source_id?: string
  source_url?: string
  metadata?: Record<string, unknown>
  occurred_at?: string
}

export interface PaginatedResult<T> {
  items: T[]
  page: number
  perPage: number
  totalItems: number
  totalPages: number
}

export interface DashboardStats {
  contacts: {
    active: number
    inactive: number
    archived: number
    total: number
  }
  organisations: {
    active: number
    archived: number
    total: number
  }
  recent_activities: number
}

export interface App {
  key: string
  name: string
  url: string
  icon: string
}
