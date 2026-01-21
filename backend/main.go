package main

import (
	"log"
	"net/http"
	"os"

	_ "github.com/grtshw/outlook-apps-crm/migrations"
	"github.com/grtshw/outlook-apps-crm/utils"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/plugins/migratecmd"
)

func main() {
	app := pocketbase.New()

	// Register migrations
	migratecmd.MustRegister(app, app.RootCmd, migratecmd.Config{
		Automigrate: true,
	})

	// OnServe hook - runs when the server starts
	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		// Security headers middleware
		e.Router.BindFunc(securityHeadersMiddleware)

		// Register custom routes
		registerRoutes(e, app)

		// Serve frontend SPA
		serveFrontend(e)

		// Start the backup scheduler (runs at 3 AM AEST daily)
		go scheduleBackups(app)

		return e.Next()
	})

	// Register webhook hooks for COPE sync to consumers (Presentations, DAM, Website)
	registerWebhookHooks(app)

	// Sync Microsoft profile photo on OAuth login
	app.OnRecordAuthWithOAuth2Request("users").BindFunc(func(e *core.RecordAuthWithOAuth2RequestEvent) error {
		if e.OAuth2User != nil && e.OAuth2User.AccessToken != "" {
			go syncMicrosoftProfilePhoto(app, e.Record, e.OAuth2User.AccessToken)
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
	e.Response.Header().Set("X-Content-Type-Options", "nosniff")
	e.Response.Header().Set("X-Frame-Options", "DENY")
	e.Response.Header().Set("X-XSS-Protection", "1; mode=block")
	return e.Next()
}

// registerRoutes sets up all custom API endpoints
func registerRoutes(e *core.ServeEvent, app *pocketbase.PocketBase) {
	// Public API endpoints (no auth required) - for COPE projections
	e.Router.GET("/api/public/contacts", func(re *core.RequestEvent) error {
		return handlePublicContacts(re, app)
	})

	e.Router.GET("/api/public/organisations", func(re *core.RequestEvent) error {
		return handlePublicOrganisations(re, app)
	})

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

	// Contact avatar upload
	e.Router.POST("/api/contacts/{id}/avatar", func(re *core.RequestEvent) error {
		return handleContactAvatarUpload(re, app)
	}).BindFunc(utils.RequireAdmin)

	// Contact activities
	e.Router.GET("/api/contacts/{id}/activities", func(re *core.RequestEvent) error {
		return handleContactActivities(re, app)
	}).BindFunc(utils.RequireAuth)

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
	e.Router.POST("/api/webhooks/activity", func(re *core.RequestEvent) error {
		return handleActivityWebhook(re, app)
	})

	// Project all endpoint - push all contacts and organisations to consumers
	e.Router.POST("/api/project-all", func(re *core.RequestEvent) error {
		return handleProjectAll(re, app)
	}).BindFunc(utils.RequireAdmin)

	// Import presenters from Presentations app (admin only)
	e.Router.POST("/api/import/presenters", func(re *core.RequestEvent) error {
		return handleImportPresenters(re, app)
	}).BindFunc(utils.RequireAdmin)

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

// Placeholder for utils import (will be used later)
var _ = utils.RequireAuth
