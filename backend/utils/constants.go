package utils

// Collection names
const (
	CollectionUsers              = "users"
	CollectionContacts           = "contacts"
	CollectionOrganisations      = "organisations"
	CollectionActivities         = "activities"
	CollectionAppSettings        = "app_settings"
	CollectionEventProjections   = "event_projections"
	CollectionGuestLists         = "guest_lists"
	CollectionGuestListItems     = "guest_list_items"
	CollectionGuestListShares    = "guest_list_shares"
	CollectionGuestListOTPCodes  = "guest_list_otp_codes"
)

// Field names
const (
	FieldStatus     = "status"
	FieldSource     = "source"
	FieldSourceIDs  = "source_ids"
	FieldTags       = "tags"
	FieldRole       = "role"
	FieldOrphanedAt = "orphaned_at"
)

// Status values
var (
	ContactStatuses      = []string{"active", "inactive", "pending", "archived"}
	OrganisationStatuses = []string{"active", "archived"}
	UserRoles            = []string{"admin", "viewer"}
)

// Source values (where the record originated from)
var (
	SourceValues = []string{"presentations", "awards", "events", "hubspot", "manual"}
)

// Activity types
var (
	ActivityTypes = []string{
		// Presentations
		"cfp_submitted",
		"session_accepted",
		"session_rejected",
		"presentation_delivered",
		// Awards
		"entry_submitted",
		"entry_shortlisted",
		"entry_winner",
		// Events
		"ticket_purchased",
		"sponsor_committed",
		"event_attended",
		// DAM
		"photo_tagged",
		"asset_featured",
		// HubSpot
		"email_sent",
		"email_opened",
		"meeting_scheduled",
		"note_added",
	}
)

// RSVP status values
var RSVPStatuses = []string{"accepted", "declined"}

// File size limits (in bytes)
const (
	MaxAvatarFileSize = 5242880 // 5MB
	MaxLogoFileSize   = 5242880 // 5MB
)
