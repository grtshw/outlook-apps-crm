package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	_ "github.com/grtshw/outlook-apps-crm/migrations"
	"github.com/grtshw/outlook-apps-crm/utils"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/plugins/migratecmd"
	"github.com/spf13/cobra"
	"github.com/theoutlook/projections/events/receiver"
)

func main() {
	app := pocketbase.New()

	// Register migrations
	migratecmd.MustRegister(app, app.RootCmd, migratecmd.Config{
		Automigrate: false, // Disabled for local testing with prod DB
	})

	// Register encrypt-pii command for migrating legacy unencrypted data
	app.RootCmd.AddCommand(&cobra.Command{
		Use:   "encrypt-pii",
		Short: "Encrypt existing unencrypted PII fields in contacts",
		Run: func(cmd *cobra.Command, args []string) {
			if err := app.Bootstrap(); err != nil {
				log.Fatalf("Failed to bootstrap: %v", err)
			}
			if err := runPIIEncryptionMigration(app); err != nil {
				log.Fatalf("Migration failed: %v", err)
			}
		},
	})

	// Register rotate-keys command for re-encrypting PII with a new key
	app.RootCmd.AddCommand(&cobra.Command{
		Use:   "rotate-keys",
		Short: "Re-encrypt all PII fields with the current key and recompute blind indexes",
		Run: func(cmd *cobra.Command, args []string) {
			if err := app.Bootstrap(); err != nil {
				log.Fatalf("Failed to bootstrap: %v", err)
			}
			if err := runKeyRotation(app); err != nil {
				log.Fatalf("Key rotation failed: %v", err)
			}
		},
	})

	// Register import-presenters command for syncing presenters from Presentations + DAM avatars
	app.RootCmd.AddCommand(&cobra.Command{
		Use:   "import-presenters",
		Short: "Import presenters from Presentations app and fetch DAM avatar URLs",
		Run: func(cmd *cobra.Command, args []string) {
			if err := app.Bootstrap(); err != nil {
				log.Fatalf("Failed to bootstrap: %v", err)
			}
			sourceURL := os.Getenv("PRESENTATIONS_API_URL")
			if sourceURL == "" {
				log.Fatal("PRESENTATIONS_API_URL environment variable not set")
			}
			fmt.Println("Importing presenters from", sourceURL)
			if err := runPresenterImport(app, sourceURL); err != nil {
				log.Fatalf("Import failed: %v", err)
			}
			fmt.Println("Import complete")
		},
	})

	// Register project-all command to push all contacts/orgs to COPE consumers
	app.RootCmd.AddCommand(&cobra.Command{
		Use:   "project-all",
		Short: "Project all contacts and organisations to COPE consumers (DAM, Presentations, Website)",
		Run: func(cmd *cobra.Command, args []string) {
			if err := app.Bootstrap(); err != nil {
				log.Fatalf("Failed to bootstrap: %v", err)
			}
			fmt.Println("Projecting all contacts and organisations to consumers...")
			result, err := ProjectAll(app)
			if err != nil {
				log.Fatalf("Projection failed: %v", err)
			}
			fmt.Printf("Projected %d contacts, %d organisations (projection_id: %s)\n", result.Counts["contacts"], result.Counts["organisations"], result.ProjectionID)
		},
	})

	// Register sync-avatar-urls command to pull avatar URLs from DAM
	app.RootCmd.AddCommand(&cobra.Command{
		Use:   "sync-avatar-urls",
		Short: "Sync avatar URLs from DAM for all contacts",
		Run: func(cmd *cobra.Command, args []string) {
			if err := app.Bootstrap(); err != nil {
				log.Fatalf("Failed to bootstrap: %v", err)
			}
			fmt.Println("Syncing avatar URLs from DAM...")
			result, err := syncAvatarURLsFromDAM(app)
			if err != nil {
				log.Fatalf("Sync failed: %v", err)
			}
			fmt.Printf("Updated %d contacts, skipped %d (total: %d)\n", result.Updated, result.Skipped, result.Total)
		},
	})

	// Register sync-humanitix command to sync attendees from Humanitix
	app.RootCmd.AddCommand(&cobra.Command{
		Use:   "sync-humanitix [event-id]",
		Short: "Sync attendees from Humanitix for an event",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			if err := app.Bootstrap(); err != nil {
				log.Fatalf("Failed to bootstrap: %v", err)
			}
			eventID := args[0]
			fmt.Printf("Syncing Humanitix attendees for event %s...\n", eventID)
			runHumanitixSync(app, "", eventID, nil)
			fmt.Println("Humanitix sync complete")
		},
	})

	// Register sync-mailchimp command to bulk sync contacts to Mailchimp
	app.RootCmd.AddCommand(&cobra.Command{
		Use:   "sync-mailchimp",
		Short: "Bulk sync all active contacts to Mailchimp",
		Run: func(cmd *cobra.Command, args []string) {
			if err := app.Bootstrap(); err != nil {
				log.Fatalf("Failed to bootstrap: %v", err)
			}
			listID, mappings := getMailchimpSyncConfig(app)
			if listID == "" {
				log.Fatal("Mailchimp list ID not configured in app_settings")
			}
			fmt.Printf("Syncing contacts to Mailchimp list %s...\n", listID)
			runMailchimpSync(app, listID, mappings)
			fmt.Println("Mailchimp sync complete")
		},
	})

	// OnServe hook - runs when the server starts
	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		// Configure SendGrid SMTP
		configurePocketBaseSMTP(app)

		// Security headers middleware
		e.Router.BindFunc(securityHeadersMiddleware)

		// Register custom routes
		registerRoutes(e, app)

		// Serve frontend SPA
		serveFrontend(e)

		// Start the backup scheduler (runs at 3 AM AEST daily)
		go scheduleBackups(app)

		// Load DAM avatar URL cache (refreshed on project-all and avatar webhooks)
		go RefreshDAMAvatarCache()

		return e.Next()
	})

	// Register webhook hooks for COPE sync to consumers (Presentations, DAM, Website)
	registerWebhookHooks(app)

	// Register audit logging hooks
	registerAuditHooks(app)

	// Register encryption hooks for PII fields
	registerEncryptionHooks(app)

	// Sync Microsoft profile photo on OAuth login (runs synchronously so the
	// auth response includes the updated avatar filename)
	app.OnRecordAuthWithOAuth2Request("users").BindFunc(func(e *core.RecordAuthWithOAuth2RequestEvent) error {
		if e.OAuth2User != nil && e.OAuth2User.AccessToken != "" {
			syncMicrosoftProfilePhoto(app, e.Record, e.OAuth2User.AccessToken)
			if fresh, err := app.FindRecordById("users", e.Record.Id); err == nil {
				e.Record = fresh
			}
		}
		return e.Next()
	})

	// Start the application
	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}

// securityHeadersMiddleware adds security headers to all responses
func securityHeadersMiddleware(e *core.RequestEvent) error {
	h := e.Response.Header()

	// Existing security headers
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("X-Frame-Options", "DENY")
	h.Set("X-XSS-Protection", "1; mode=block")

	// HSTS - enforce HTTPS for 1 year, include subdomains
	h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")

	// Content Security Policy - restrict sources
	h.Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data: https:; connect-src 'self' https:; frame-ancestors 'none'")

	// Referrer Policy - don't leak URLs to external sites
	h.Set("Referrer-Policy", "strict-origin-when-cross-origin")

	// Permissions Policy - disable unused browser features
	h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=()")

	return e.Next()
}

// registerRoutes sets up all custom API endpoints
func registerRoutes(e *core.ServeEvent, app *pocketbase.PocketBase) {
	// Public API endpoints (no auth required) - for COPE projections
	// Rate limited to prevent scraping
	e.Router.GET("/api/public/contacts", func(re *core.RequestEvent) error {
		return handlePublicContacts(re, app)
	}).BindFunc(utils.RateLimitPublic)

	e.Router.GET("/api/public/organisations", func(re *core.RequestEvent) error {
		return handlePublicOrganisations(re, app)
	}).BindFunc(utils.RateLimitPublic)

	// External API (service-to-service with token auth)
	// Rate limited to prevent abuse
	// Used by Presentations for self-registration and profile updates
	e.Router.POST("/api/external/contacts", func(re *core.RequestEvent) error {
		return handleExternalContactCreate(re, app)
	}).BindFunc(utils.RateLimitExternalAPI)
	e.Router.PATCH("/api/external/contacts/{id}", func(re *core.RequestEvent) error {
		return handleExternalContactUpdate(re, app)
	}).BindFunc(utils.RateLimitExternalAPI)
	// Used by Presentations for organisation management
	e.Router.POST("/api/external/organisations", func(re *core.RequestEvent) error {
		return handleExternalOrganisationCreate(re, app)
	}).BindFunc(utils.RateLimitExternalAPI)
	e.Router.PATCH("/api/external/organisations/{id}", func(re *core.RequestEvent) error {
		return handleExternalOrganisationUpdate(re, app)
	}).BindFunc(utils.RateLimitExternalAPI)

	// Protected routes (require auth)
	// Dashboard stats
	e.Router.GET("/api/dashboard/stats", func(re *core.RequestEvent) error {
		return handleDashboardStats(re, app)
	}).BindFunc(utils.RequireAuth)

	// Contacts CRUD
	e.Router.GET("/api/contacts", func(re *core.RequestEvent) error {
		return handleContactsList(re, app)
	}).BindFunc(utils.RequireAuth)

	e.Router.GET("/api/contacts/{id}", func(re *core.RequestEvent) error {
		return handleContactGet(re, app)
	}).BindFunc(utils.RequireAuth)

	e.Router.POST("/api/contacts", func(re *core.RequestEvent) error {
		return handleContactCreate(re, app)
	}).BindFunc(utils.RequireAdmin)

	e.Router.PATCH("/api/contacts/{id}", func(re *core.RequestEvent) error {
		return handleContactUpdate(re, app)
	}).BindFunc(utils.RequireAdmin)

	e.Router.DELETE("/api/contacts/{id}", func(re *core.RequestEvent) error {
		return handleContactDelete(re, app)
	}).BindFunc(utils.RequireAdmin)

	// Merge contacts
	e.Router.POST("/api/contacts/merge", func(re *core.RequestEvent) error {
		return handleContactsMerge(re, app)
	}).BindFunc(utils.RateLimitAuth).BindFunc(utils.RequireAdmin)

	// Contact avatar upload
	e.Router.POST("/api/contacts/{id}/avatar", func(re *core.RequestEvent) error {
		return handleContactAvatarUpload(re, app)
	}).BindFunc(utils.RequireAdmin)

	// Contact activities
	e.Router.GET("/api/contacts/{id}/activities", func(re *core.RequestEvent) error {
		return handleContactActivities(re, app)
	}).BindFunc(utils.RequireAuth)

	// Contact links
	e.Router.GET("/api/contacts/{id}/links", func(re *core.RequestEvent) error {
		return handleContactLinksList(re, app)
	}).BindFunc(utils.RateLimitAuth).BindFunc(utils.RequireAuth)

	e.Router.POST("/api/contacts/{id}/links", func(re *core.RequestEvent) error {
		return handleContactLinkCreate(re, app)
	}).BindFunc(utils.RateLimitAuth).BindFunc(utils.RequireAdmin)

	e.Router.DELETE("/api/contact-links/{linkId}", func(re *core.RequestEvent) error {
		return handleContactLinkDelete(re, app)
	}).BindFunc(utils.RateLimitAuth).BindFunc(utils.RequireAdmin)

	// Humanitix integration (admin only)
	e.Router.GET("/api/admin/humanitix/events", func(re *core.RequestEvent) error {
		return handleHumanitixEventsList(re, app)
	}).BindFunc(utils.RateLimitAuth).BindFunc(utils.RequireAdmin)

	e.Router.POST("/api/admin/humanitix/sync", func(re *core.RequestEvent) error {
		return handleHumanitixSync(re, app)
	}).BindFunc(utils.RateLimitAuth).BindFunc(utils.RequireAdmin)

	e.Router.GET("/api/admin/humanitix/sync-logs", func(re *core.RequestEvent) error {
		return handleHumanitixSyncLogs(re, app)
	}).BindFunc(utils.RateLimitAuth).BindFunc(utils.RequireAuth)

	// Mailchimp integration (admin only + webhook)
	e.Router.GET("/api/admin/mailchimp/status", func(re *core.RequestEvent) error {
		return handleMailchimpStatus(re, app)
	}).BindFunc(utils.RateLimitAuth).BindFunc(utils.RequireAdmin)

	e.Router.GET("/api/admin/mailchimp/lists", func(re *core.RequestEvent) error {
		return handleMailchimpLists(re, app)
	}).BindFunc(utils.RateLimitAuth).BindFunc(utils.RequireAdmin)

	e.Router.GET("/api/admin/mailchimp/lists/{id}/merge-fields", func(re *core.RequestEvent) error {
		return handleMailchimpMergeFields(re, app)
	}).BindFunc(utils.RateLimitAuth).BindFunc(utils.RequireAdmin)

	e.Router.GET("/api/admin/mailchimp/settings", func(re *core.RequestEvent) error {
		return handleMailchimpSettingsGet(re, app)
	}).BindFunc(utils.RateLimitAuth).BindFunc(utils.RequireAdmin)

	e.Router.PUT("/api/admin/mailchimp/settings", func(re *core.RequestEvent) error {
		return handleMailchimpSettingsSave(re, app)
	}).BindFunc(utils.RateLimitAuth).BindFunc(utils.RequireAdmin)

	e.Router.POST("/api/admin/mailchimp/sync", func(re *core.RequestEvent) error {
		return handleMailchimpSync(re, app)
	}).BindFunc(utils.RateLimitAuth).BindFunc(utils.RequireAdmin)

	e.Router.POST("/api/admin/mailchimp/sync/{id}", func(re *core.RequestEvent) error {
		return handleMailchimpSyncContact(re, app)
	}).BindFunc(utils.RateLimitAuth).BindFunc(utils.RequireAdmin)

	e.Router.POST("/api/webhooks/mailchimp", func(re *core.RequestEvent) error {
		return handleMailchimpWebhook(re, app)
	}).BindFunc(utils.RateLimitExternalAPI)

	// Also handle Mailchimp webhook validation GET
	e.Router.GET("/api/webhooks/mailchimp", func(re *core.RequestEvent) error {
		return handleMailchimpWebhook(re, app)
	}).BindFunc(utils.RateLimitExternalAPI)

	// Attendee API (public, OTP-authenticated)
	e.Router.POST("/api/attendee/send-otp", func(re *core.RequestEvent) error {
		return handleAttendeeSendOTP(re, app)
	}).BindFunc(utils.RateLimitPublic)

	e.Router.POST("/api/attendee/verify", func(re *core.RequestEvent) error {
		return handleAttendeeVerifyOTP(re, app)
	}).BindFunc(utils.RateLimitPublic)

	e.Router.GET("/api/attendee/profile", func(re *core.RequestEvent) error {
		return handleAttendeeProfile(re, app)
	}).BindFunc(utils.RateLimitPublic)

	e.Router.PATCH("/api/attendee/profile", func(re *core.RequestEvent) error {
		return handleAttendeeProfileUpdate(re, app)
	}).BindFunc(utils.RateLimitPublic)

	e.Router.GET("/api/attendee/activities", func(re *core.RequestEvent) error {
		return handleAttendeeActivities(re, app)
	}).BindFunc(utils.RateLimitPublic)

	e.Router.POST("/api/attendee/link-email", func(re *core.RequestEvent) error {
		return handleAttendeeEmailLink(re, app)
	}).BindFunc(utils.RateLimitPublic)

	// Organisations CRUD
	e.Router.GET("/api/organisations", func(re *core.RequestEvent) error {
		return handleOrganisationsList(re, app)
	}).BindFunc(utils.RequireAuth)

	e.Router.GET("/api/organisations/{id}", func(re *core.RequestEvent) error {
		return handleOrganisationGet(re, app)
	}).BindFunc(utils.RequireAuth)

	e.Router.POST("/api/organisations", func(re *core.RequestEvent) error {
		return handleOrganisationCreate(re, app)
	}).BindFunc(utils.RequireAdmin)

	e.Router.PATCH("/api/organisations/{id}", func(re *core.RequestEvent) error {
		return handleOrganisationUpdate(re, app)
	}).BindFunc(utils.RequireAdmin)

	e.Router.DELETE("/api/organisations/{id}", func(re *core.RequestEvent) error {
		return handleOrganisationDelete(re, app)
	}).BindFunc(utils.RequireAdmin)

	// Organisation logo upload token (for DAM uploads)
	// Frontend requests a signed token, then uploads directly to DAM
	e.Router.POST("/api/organisations/{id}/logo/{type}/token", func(re *core.RequestEvent) error {
		return handleOrganisationLogoUploadToken(re, app)
	}).BindFunc(utils.RequireAdmin)

	// Activities list
	e.Router.GET("/api/activities", func(re *core.RequestEvent) error {
		return handleActivitiesList(re, app)
	}).BindFunc(utils.RequireAuth)

	// Activity webhook receiver (from other apps)
	// Rate limited to prevent abuse
	e.Router.POST("/api/webhooks/activity", func(re *core.RequestEvent) error {
		return handleActivityWebhook(re, app)
	}).BindFunc(utils.RateLimitExternalAPI)

	// Avatar URL webhook receiver (from DAM - avatar variant URLs after processing)
	e.Router.POST("/api/webhooks/avatar-urls", func(re *core.RequestEvent) error {
		return handleAvatarURLWebhook(re, app)
	}).BindFunc(utils.RateLimitExternalAPI)

	// Sync avatar URLs from DAM (admin only - one-time pull)
	e.Router.POST("/api/admin/sync-avatar-urls", func(re *core.RequestEvent) error {
		return handleSyncAvatarURLs(re, app)
	}).BindFunc(utils.RateLimitAuth).BindFunc(utils.RequireAdmin)

	// Project all endpoint - push all contacts and organisations to consumers
	e.Router.POST("/api/project-all", func(re *core.RequestEvent) error {
		return handleProjectAll(re, app)
	}).BindFunc(utils.RateLimitAuth).BindFunc(utils.RequireAdmin)

	// Projection management endpoints
	e.Router.GET("/api/projections/logs", func(re *core.RequestEvent) error {
		return handleProjectionLogs(app, re)
	}).BindFunc(utils.RequireAuth)

	e.Router.GET("/api/projections/{id}/progress", func(re *core.RequestEvent) error {
		return handleProjectionProgress(app, re)
	}).BindFunc(utils.RequireAuth)

	e.Router.GET("/api/projection-consumers", func(re *core.RequestEvent) error {
		return handleListProjectionConsumers(app, re)
	}).BindFunc(utils.RequireAuth)

	e.Router.PATCH("/api/projection-consumers/{id}/toggle", func(re *core.RequestEvent) error {
		return handleToggleProjectionConsumer(app, re)
	}).BindFunc(utils.RequireAdmin)

	// Projection callback endpoint (public - consumers report status)
	e.Router.POST("/api/projections/callback", func(re *core.RequestEvent) error {
		return handleProjectionCallback(app, re)
	}).BindFunc(utils.RateLimitExternalAPI)

	// Import presenters from Presentations app (admin only)
	e.Router.POST("/api/import/presenters", func(re *core.RequestEvent) error {
		return handleImportPresenters(re, app)
	}).BindFunc(utils.RequireAdmin)

	// Event projection webhook (COPE - receive events from Events app)
	eventReceiver := receiver.NewReceiver(receiver.Config{
		WebhookSecret: os.Getenv("PROJECTION_WEBHOOK_SECRET"),
		ConsumerName:  "crm",
	}, app)
	e.Router.POST("/api/webhooks/event-projection", eventReceiver.HandleWebhook).BindFunc(utils.RateLimitExternalAPI)

	// Event projections list (for guest list event dropdown)
	e.Router.GET("/api/event-projections", func(re *core.RequestEvent) error {
		return handleListEventProjections(re, app)
	}).BindFunc(utils.RequireAuth)

	// Guest lists CRUD (admin only)
	e.Router.GET("/api/guest-lists", func(re *core.RequestEvent) error {
		return handleGuestListsList(re, app)
	}).BindFunc(utils.RateLimitAuth).BindFunc(utils.RequireAdmin)

	e.Router.GET("/api/guest-lists/{id}", func(re *core.RequestEvent) error {
		return handleGuestListGet(re, app)
	}).BindFunc(utils.RateLimitAuth).BindFunc(utils.RequireAdmin)

	e.Router.POST("/api/guest-lists", func(re *core.RequestEvent) error {
		return handleGuestListCreate(re, app)
	}).BindFunc(utils.RateLimitAuth).BindFunc(utils.RequireAdmin)

	e.Router.PATCH("/api/guest-lists/{id}", func(re *core.RequestEvent) error {
		return handleGuestListUpdate(re, app)
	}).BindFunc(utils.RateLimitAuth).BindFunc(utils.RequireAdmin)

	e.Router.DELETE("/api/guest-lists/{id}", func(re *core.RequestEvent) error {
		return handleGuestListDelete(re, app)
	}).BindFunc(utils.RateLimitAuth).BindFunc(utils.RequireAdmin)

	e.Router.POST("/api/guest-lists/{id}/clone", func(re *core.RequestEvent) error {
		return handleGuestListClone(re, app)
	}).BindFunc(utils.RateLimitAuth).BindFunc(utils.RequireAdmin)

	// Guest list items (admin only)
	e.Router.GET("/api/guest-lists/{id}/items", func(re *core.RequestEvent) error {
		return handleGuestListItemsList(re, app)
	}).BindFunc(utils.RateLimitAuth).BindFunc(utils.RequireAdmin)

	e.Router.POST("/api/guest-lists/{id}/items", func(re *core.RequestEvent) error {
		return handleGuestListItemCreate(re, app)
	}).BindFunc(utils.RateLimitAuth).BindFunc(utils.RequireAdmin)

	e.Router.POST("/api/guest-lists/{id}/items/bulk", func(re *core.RequestEvent) error {
		return handleGuestListItemBulkAdd(re, app)
	}).BindFunc(utils.RateLimitAuth).BindFunc(utils.RequireAdmin)

	e.Router.PATCH("/api/guest-list-items/{itemId}", func(re *core.RequestEvent) error {
		return handleGuestListItemUpdate(re, app)
	}).BindFunc(utils.RateLimitAuth).BindFunc(utils.RequireAdmin)

	e.Router.DELETE("/api/guest-list-items/{itemId}", func(re *core.RequestEvent) error {
		return handleGuestListItemDelete(re, app)
	}).BindFunc(utils.RateLimitAuth).BindFunc(utils.RequireAdmin)

	// Guest list shares (admin only)
	e.Router.GET("/api/guest-lists/{id}/shares", func(re *core.RequestEvent) error {
		return handleGuestListSharesList(re, app)
	}).BindFunc(utils.RateLimitAuth).BindFunc(utils.RequireAdmin)

	e.Router.POST("/api/guest-lists/{id}/shares", func(re *core.RequestEvent) error {
		return handleGuestListShareCreate(re, app)
	}).BindFunc(utils.RateLimitAuth).BindFunc(utils.RequireAdmin)

	e.Router.DELETE("/api/guest-list-shares/{shareId}", func(re *core.RequestEvent) error {
		return handleGuestListShareRevoke(re, app)
	}).BindFunc(utils.RateLimitAuth).BindFunc(utils.RequireAdmin)

	// Public share endpoints (no CRM auth, rate limited)
	e.Router.GET("/api/public/guest-lists/{token}", func(re *core.RequestEvent) error {
		return handlePublicGuestListInfo(re, app)
	}).BindFunc(utils.RateLimitPublic)

	e.Router.POST("/api/public/guest-lists/{token}/send-otp", func(re *core.RequestEvent) error {
		return handlePublicGuestListSendOTP(re, app)
	}).BindFunc(utils.RateLimitPublic)

	e.Router.POST("/api/public/guest-lists/{token}/verify", func(re *core.RequestEvent) error {
		return handlePublicGuestListVerify(re, app)
	}).BindFunc(utils.RateLimitPublic)

	e.Router.GET("/api/public/guest-lists/{token}/view", func(re *core.RequestEvent) error {
		return handlePublicGuestListView(re, app)
	}).BindFunc(utils.RateLimitPublic)

	e.Router.PATCH("/api/public/guest-lists/{token}/items/{itemId}", func(re *core.RequestEvent) error {
		return handlePublicGuestListItemUpdate(re, app)
	}).BindFunc(utils.RateLimitPublic)

	e.Router.PATCH("/api/public/guest-lists/{token}/landing", func(re *core.RequestEvent) error {
		return handlePublicGuestListLandingUpdate(re, app)
	}).BindFunc(utils.RateLimitPublic)

	// Public RSVP endpoints (no CRM auth, rate limited)
	e.Router.GET("/api/public/rsvp/{token}", func(re *core.RequestEvent) error {
		return handlePublicRSVPInfo(re, app)
	}).BindFunc(utils.RateLimitPublic)

	e.Router.POST("/api/public/rsvp/{token}", func(re *core.RequestEvent) error {
		return handlePublicRSVPSubmit(re, app)
	}).BindFunc(utils.RateLimitPublic)

	e.Router.POST("/api/public/rsvp/{token}/forward", func(re *core.RequestEvent) error {
		return handlePublicRSVPForward(re, app)
	}).BindFunc(utils.RateLimitPublic)

	e.Router.GET("/api/public/rsvp/{token}/email-preview", func(re *core.RequestEvent) error {
		return handlePublicRSVPEmailPreview(re, app)
	}).BindFunc(utils.RateLimitPublic)

	// Admin RSVP management
	e.Router.POST("/api/guest-lists/{id}/rsvp/enable", func(re *core.RequestEvent) error {
		return handleGuestListRSVPToggle(re, app)
	}).BindFunc(utils.RateLimitAuth).BindFunc(utils.RequireAdmin)

	e.Router.POST("/api/guest-lists/{id}/rsvp/send-invites", func(re *core.RequestEvent) error {
		return handleGuestListRSVPSendInvites(re, app)
	}).BindFunc(utils.RateLimitAuth).BindFunc(utils.RequireAdmin)

	log.Printf("[Routes] Registered API endpoints")
}

// serveFrontend serves the SPA frontend
func serveFrontend(e *core.ServeEvent) {
	// Check if frontend dist exists
	staticDir := "./pb_public"
	if _, err := os.Stat(staticDir); os.IsNotExist(err) {
		staticDir = "../frontend/dist"
	}

	// Serve static files
	e.Router.GET("/{path...}", func(re *core.RequestEvent) error {
		path := re.Request.PathValue("path")

		// Don't handle API routes - let them 404 if not matched
		if len(path) >= 4 && path[:4] == "api/" {
			return re.JSON(http.StatusNotFound, map[string]string{"error": "Not found"})
		}

		// Root path or empty - serve index.html
		if path == "" || path == "/" {
			return re.FileFS(os.DirFS(staticDir), "index.html")
		}

		filePath := staticDir + "/" + path

		// Check if file exists (and is not a directory)
		if info, err := os.Stat(filePath); err == nil && !info.IsDir() {
			return re.FileFS(os.DirFS(staticDir), path)
		}

		// SPA fallback - serve index.html for client-side routing
		return re.FileFS(os.DirFS(staticDir), "index.html")
	})
}

// registerEncryptionHooks sets up PII field encryption for contacts
func registerEncryptionHooks(app *pocketbase.PocketBase) {
	// Only contacts collection has PII fields to encrypt
	piiFields := []string{"email", "personal_email", "phone", "bio", "location"}

	// Encrypt PII fields after validation, before database insert
	// OnRecordCreateExecute fires after validation passes
	app.OnRecordCreateExecute("contacts").BindFunc(func(e *core.RecordEvent) error {
		if !utils.IsEncryptionEnabled() {
			return e.Next()
		}

		for _, field := range piiFields {
			val := e.Record.GetString(field)
			if val == "" {
				continue
			}
			// Skip if already encrypted
			if len(val) > 4 && val[:4] == "enc:" {
				continue
			}
			encrypted, err := utils.Encrypt(val)
			if err == nil {
				e.Record.Set(field, encrypted)
			}
		}

		// Set blind index for email lookups
		if email := e.Record.GetString("email"); email != "" {
			originalEmail := utils.DecryptField(email)
			e.Record.Set("email_index", utils.BlindIndex(originalEmail))
		}

		// Set blind index for personal_email lookups
		if personalEmail := e.Record.GetString("personal_email"); personalEmail != "" {
			originalEmail := utils.DecryptField(personalEmail)
			e.Record.Set("personal_email_index", utils.BlindIndex(originalEmail))
		}

		return e.Next()
	})

	// Encrypt PII fields after validation, before database update
	app.OnRecordUpdateExecute("contacts").BindFunc(func(e *core.RecordEvent) error {
		if !utils.IsEncryptionEnabled() {
			return e.Next()
		}

		for _, field := range piiFields {
			val := e.Record.GetString(field)
			if val == "" {
				continue
			}
			// Skip if already encrypted
			if len(val) > 4 && val[:4] == "enc:" {
				continue
			}
			encrypted, err := utils.Encrypt(val)
			if err == nil {
				e.Record.Set(field, encrypted)
			}
		}

		// Update blind index for email lookups
		if email := e.Record.GetString("email"); email != "" {
			originalEmail := utils.DecryptField(email)
			e.Record.Set("email_index", utils.BlindIndex(originalEmail))
		}

		// Update blind index for personal_email lookups
		if personalEmail := e.Record.GetString("personal_email"); personalEmail != "" {
			originalEmail := utils.DecryptField(personalEmail)
			e.Record.Set("personal_email_index", utils.BlindIndex(originalEmail))
		}

		return e.Next()
	})
}

// registerAuditHooks sets up audit logging for CRUD operations and auth events
func registerAuditHooks(app *pocketbase.PocketBase) {
	// Collections to audit
	collections := []string{"contacts", "organisations", "activities", "guest_lists", "guest_list_items", "guest_list_shares", "contact_links", "humanitix_sync_log", "attendee_otp_codes"}

	for _, coll := range collections {
		collName := coll // capture for closure

		// Log after successful create
		app.OnRecordAfterCreateSuccess(collName).BindFunc(func(e *core.RecordEvent) error {
			utils.LogRecordChange(app, "create", collName, e.Record.Id, map[string]any{
				"data": e.Record.FieldsData(),
			})
			return e.Next()
		})

		// Log after successful update
		app.OnRecordAfterUpdateSuccess(collName).BindFunc(func(e *core.RecordEvent) error {
			utils.LogRecordChange(app, "update", collName, e.Record.Id, map[string]any{
				"data": e.Record.FieldsData(),
			})
			return e.Next()
		})

		// Log after successful delete
		app.OnRecordAfterDeleteSuccess(collName).BindFunc(func(e *core.RecordEvent) error {
			utils.LogRecordChange(app, "delete", collName, e.Record.Id, nil)
			return e.Next()
		})
	}

	// Log successful authentication
	app.OnRecordAuthRequest("users").BindFunc(func(e *core.RecordAuthRequestEvent) error {
		utils.LogAuthEvent(app, "login", e.Record.Id, e.Record.GetString("email"),
			"", "", "success", "")
		return e.Next()
	})
}

// Placeholder for utils import (will be used later)
var _ = utils.RequireAuth

// runPIIEncryptionMigration encrypts all unencrypted PII fields in contacts
func runPIIEncryptionMigration(app *pocketbase.PocketBase) error {
	if !utils.IsEncryptionEnabled() {
		return fmt.Errorf("ENCRYPTION_KEY not set - cannot encrypt data")
	}

	log.Println("[EncryptPII] Starting PII encryption migration...")

	records, err := app.FindAllRecords("contacts")
	if err != nil {
		return fmt.Errorf("failed to fetch contacts: %w", err)
	}

	log.Printf("[EncryptPII] Found %d contacts to process", len(records))

	piiFields := []string{"email", "personal_email", "phone", "bio", "location"}
	migrated := 0
	skipped := 0

	for _, record := range records {
		needsUpdate := false

		for _, field := range piiFields {
			val := record.GetString(field)
			if val == "" {
				continue
			}

			// Check if already encrypted (has "enc:" prefix)
			if strings.HasPrefix(val, "enc:") {
				continue
			}

			// Encrypt the field
			encrypted, err := utils.Encrypt(val)
			if err != nil {
				log.Printf("[EncryptPII] Warning: failed to encrypt %s for contact %s: %v", field, record.Id, err)
				continue
			}

			record.Set(field, encrypted)
			needsUpdate = true
		}

		// Update email_index for blind search
		email := record.GetString("email")
		if email != "" {
			originalEmail := utils.DecryptField(email)
			blindIndex := utils.BlindIndex(originalEmail)
			if record.GetString("email_index") != blindIndex {
				record.Set("email_index", blindIndex)
				needsUpdate = true
			}
		}

		// Update personal_email_index for blind search
		personalEmail := record.GetString("personal_email")
		if personalEmail != "" {
			originalEmail := utils.DecryptField(personalEmail)
			blindIndex := utils.BlindIndex(originalEmail)
			if record.GetString("personal_email_index") != blindIndex {
				record.Set("personal_email_index", blindIndex)
				needsUpdate = true
			}
		}

		if needsUpdate {
			// Use SaveNoValidate to bypass email validation (encrypted value isn't valid email format)
			if err := app.SaveNoValidate(record); err != nil {
				log.Printf("[EncryptPII] Error: failed to save contact %s: %v", record.Id, err)
				continue
			}
			migrated++
			log.Printf("[EncryptPII] Encrypted contact: %s", record.Id)
		} else {
			skipped++
		}
	}

	log.Printf("[EncryptPII] Migration complete: %d encrypted, %d already encrypted/empty", migrated, skipped)
	return nil
}

// runKeyRotation re-encrypts all PII fields with the current key and recomputes blind indexes.
// Used after rotating ENCRYPTION_KEY to migrate data from the previous key version.
func runKeyRotation(app *pocketbase.PocketBase) error {
	if !utils.IsEncryptionEnabled() {
		return fmt.Errorf("ENCRYPTION_KEY not set - cannot rotate keys")
	}

	log.Println("[RotateKeys] Starting key rotation for contacts...")

	records, err := app.FindAllRecords("contacts")
	if err != nil {
		return fmt.Errorf("failed to fetch contacts: %w", err)
	}

	log.Printf("[RotateKeys] Found %d contacts to process", len(records))

	piiFields := []string{"email", "personal_email", "phone", "bio", "location"}
	currentPrefix := fmt.Sprintf("enc:v%d:", utils.CurrentKeyVersion())
	rotated := 0
	skipped := 0
	errCount := 0

	for _, record := range records {
		needsUpdate := false

		for _, field := range piiFields {
			val := record.GetString(field)
			if val == "" {
				continue
			}

			// Already using current key version — skip
			if strings.HasPrefix(val, currentPrefix) {
				continue
			}

			if !strings.HasPrefix(val, "enc:") {
				// Unencrypted legacy data — encrypt it
				encrypted, err := utils.Encrypt(val)
				if err != nil {
					log.Printf("[RotateKeys] Warning: failed to encrypt %s for %s: %v", field, record.Id, err)
					continue
				}
				record.Set(field, encrypted)
				needsUpdate = true
				continue
			}

			// Decrypt with old key, re-encrypt with current key
			plaintext, err := utils.Decrypt(val)
			if err != nil {
				log.Printf("[RotateKeys] Warning: failed to decrypt %s for %s: %v", field, record.Id, err)
				continue
			}
			encrypted, err := utils.Encrypt(plaintext)
			if err != nil {
				log.Printf("[RotateKeys] Warning: failed to re-encrypt %s for %s: %v", field, record.Id, err)
				continue
			}
			record.Set(field, encrypted)
			needsUpdate = true
		}

		// Recompute blind indexes with the current key
		if email := record.GetString("email"); email != "" {
			originalEmail := utils.DecryptField(email)
			newIndex := utils.BlindIndex(originalEmail)
			if record.GetString("email_index") != newIndex {
				record.Set("email_index", newIndex)
				needsUpdate = true
			}
		}
		if personalEmail := record.GetString("personal_email"); personalEmail != "" {
			originalEmail := utils.DecryptField(personalEmail)
			newIndex := utils.BlindIndex(originalEmail)
			if record.GetString("personal_email_index") != newIndex {
				record.Set("personal_email_index", newIndex)
				needsUpdate = true
			}
		}

		if needsUpdate {
			if err := app.SaveNoValidate(record); err != nil {
				log.Printf("[RotateKeys] Error: failed to save contact %s: %v", record.Id, err)
				errCount++
				continue
			}
			rotated++
		} else {
			skipped++
		}
	}

	log.Printf("[RotateKeys] Complete: %d rotated, %d already current, %d errors", rotated, skipped, errCount)
	if errCount > 0 {
		return fmt.Errorf("%d records failed to rotate", errCount)
	}
	return nil
}
