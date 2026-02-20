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
export type DietaryRequirement = 'vegetarian' | 'vegan' | 'gluten_free' | 'dairy_free' | 'nut_allergy' | 'halal' | 'kosher'
export type AccessibilityRequirement = 'wheelchair_access' | 'hearing_assistance' | 'vision_assistance' | 'sign_language_interpreter' | 'mobility_assistance'

export interface Contact extends RecordModel {
  email: string
  first_name: string
  last_name: string
  name: string
  personal_email?: string
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
  status: 'active' | 'inactive' | 'pending' | 'archived'
  source?: string
  source_ids?: Record<string, string>
  degrees?: '1st' | '2nd' | '3rd'
  relationship?: number
  notes?: string
  dietary_requirements?: DietaryRequirement[]
  dietary_requirements_other?: string
  accessibility_requirements?: AccessibilityRequirement[]
  accessibility_requirements_other?: string
  linked_contacts?: ContactLink[]
}

export interface ContactLink {
  link_id: string
  contact_id: string
  name: string
  email: string
  avatar_thumb_url?: string
  organisation?: string
  verified: boolean
  source: 'manual' | 'attendee' | 'system'
  notes?: string
  created: string
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
  industry?: string
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

export interface EventProjection extends RecordModel {
  event_id: string
  slug: string
  name: string
  edition_year: number
  date: string
  venue: string
  venue_city: string
  format: string
  event_type: string
  status: string
  capacity: number
  description: string
}

export interface ProgramItem {
  time: string
  title: string
  description?: string
  speaker_contact_id?: string
  speaker_name?: string
  speaker_org?: string
  speaker_image_url?: string
}

export interface GuestList extends RecordModel {
  name: string
  description: string
  event_projection: string
  event_name: string
  created_by: string
  status: 'draft' | 'active' | 'archived'
  item_count: number
  share_count: number
  rsvp_enabled: boolean
  rsvp_generic_token: string
  rsvp_generic_url: string
  landing_enabled: boolean
  landing_headline: string
  landing_description: string
  landing_image_url: string
  landing_program: ProgramItem[]
  landing_content: string
  event_date: string
  event_time: string
  event_location: string
  event_location_address: string
  organisation: string
  organisation_name: string
  organisation_logo_url: string
  rsvp_bcc_contacts: Array<{ id: string; name: string; email: string }>
}

export interface GuestListItem extends RecordModel {
  contact_id: string
  contact_name: string
  contact_email: string
  contact_job_title: string
  contact_organisation_name: string
  contact_linkedin: string
  contact_location: string
  contact_degrees: '1st' | '2nd' | '3rd' | ''
  contact_relationship: number
  contact_status?: string
  contact_avatar_url?: string
  contact_avatar_small_url?: string
  contact_avatar_thumb_url?: string
  invite_round: '1st' | '2nd' | '3rd' | 'maybe' | ''
  invite_status: 'to_invite' | 'invited' | 'accepted' | 'declined' | 'no_show' | ''
  notes: string
  client_notes: string
  sort_order: number
  rsvp_token: string
  rsvp_status: 'accepted' | 'declined' | ''
  rsvp_dietary: string
  rsvp_plus_one: boolean
  rsvp_plus_one_name: string
  rsvp_plus_one_dietary: string
  rsvp_responded_at: string
  rsvp_invited_by: string
  rsvp_comments: string
  invite_opened: boolean
  invite_clicked: boolean
}

export interface GuestListShare extends RecordModel {
  token: string
  recipient_email: string
  recipient_name: string
  expires_at: string
  revoked: boolean
  verified_at: string
  last_accessed_at: string
  access_count: number
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
