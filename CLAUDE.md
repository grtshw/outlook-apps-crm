# CRM App

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

**Read README.md first** for product context: user roles, key entities, workflows, vocabulary, and prompt tips.

## Commit Preferences

When the user says `commit`, `push`, or `c+p` after making changes, skip verification steps (status, diff, log) and just commit with a sensible message and push. When the user says `deploy` or `d`, just run `fly deploy` without extra checks. Only do full verification for complex or ambiguous situations.

## Shared Architecture (READ FIRST)

This app is part of The Outlook Apps ecosystem. **Before making changes**, read the shared documentation:

| Document | Location | Purpose |
|----------|----------|---------||
| **ARCHITECTURE.md** | `ui-kit/ARCHITECTURE.md` | Component usage, examples, API reference |
| **PATTERNS.md** | `ui-kit/PATTERNS.md` | Mandatory patterns, code standards, review checklist |
| **DEPENDENCIES.md** | `ui-kit/DEPENDENCIES.md` | Shared dependency versions, update procedures |

### Quick Reference - Mandatory Patterns

```typescript
// Security - ALWAYS escape user content
import { escapeHtml } from '@theoutlook/ui-kit';
html = `<p>${escapeHtml(userInput)}</p>`;

// Loading - Use ui-kit skeletons
import { renderTableSkeleton } from '@theoutlook/ui-kit';

// Confirmations - Use ui-kit dialogs
import { showDeleteConfirmation } from '@theoutlook/ui-kit';

// Toasts - Use ui-kit notifications
import { showToast } from '@theoutlook/ui-kit';

// Errors - Extract user-friendly messages
import { extractErrorMessage } from '@theoutlook/ui-kit';

// Buttons - ALWAYS use .btn class (NEVER inline Tailwind button styling)
html = `<button class="btn">Primary</button>`;
html = `<button class="btn btn-danger">Delete</button>`;
html = `<button class="btn btn-secondary">Cancel</button>`;
```

### Critical Rule: ui-kit Is Mandatory

**NEVER create custom implementations when ui-kit provides the functionality.** Do not modify or move away from ui-kit patterns unless explicitly instructed by the user. When functionality is missing from ui-kit, **ASK FIRST** before adding to ui-kit or creating pattern exceptions.

### ui-kit Principles and Enforcement

**Goal:** Build universal UI components that minimise refactor effort by centralising structure, styling, accessibility, and interaction patterns in the ui-kit.

**Component ownership boundaries:**

| ui-kit components must own | Templates/app screens must own |
|---------------------------|-------------------------------|
| Semantics and accessibility (elements, aria, keyboard behaviour) | Data shaping and mapping |
| Internal layout and spacing rules | Business logic and state |
| Interaction patterns and visual states | Event wiring and side effects |
| Design tokens and styling primitives | Routing, permissions, and orchestration |
| Portable variants that work across apps | |

**Hard rule:** Templates/screens must pass **data**, not **layout decisions**.
- Allowed: `title`, `description`, `items`, `status`, `tone`, `size`, `icon`, `actions`
- Not allowed: raw layout classes, spacing rules, DOM structure overrides

## Development

Start the dev server:
```bash
./start.sh
```

This runs:
- PocketBase backend on http://localhost:8090
- Vite dev server on http://localhost:3000 (proxies API to backend)
- PocketBase admin UI at http://localhost:8090/_/

## First-time setup

1. Start the server with `./start.sh`
2. Go to http://localhost:8090/_/ to create an admin account
3. Create user accounts via PocketBase admin UI

## Build for production

```bash
cd frontend && npm run build
go build -o crm ./backend
./crm serve --http="0.0.0.0:8080"
```

## Deploy to Fly.io

```bash
fly deploy
```

## Style guide

- Headings, labels, and buttons should use sentence case
- Use Tailwind utilities only, no inline styles except for dynamic values
- Never use font weight utilities (font-bold, font-semibold, font-medium, etc.) - rely on the font file's default weight

## Backend Patterns

### Response Helpers

Use helpers from `utils/helpers.go` for consistent responses:

```go
import "github.com/grtshw/outlook-apps-crm/utils"

// Error responses
return utils.NotFoundResponse(re, "Contact not found")
return utils.BadRequestResponse(re, "Invalid input")
return utils.InternalErrorResponse(re, "Failed to process")

// Success responses
return utils.SuccessResponse(re, "Contact deleted successfully")
return utils.DataResponse(re, resultData)
```

### SQL Injection Prevention

**ALWAYS use parameterized queries with PocketBase's binding syntax:**

```go
// ✅ CORRECT - parameterized query
records, err := app.FindRecordsByFilter("contacts", "email = {:email}", "", 1, 0, map[string]any{"email": userInput})

// ❌ WRONG - string interpolation (SQL injection risk)
query := fmt.Sprintf("SELECT * FROM contacts WHERE email = '%s'", userInput)
```

### Auth Middleware

```go
// Require any authenticated user
e.Router.GET("/api/contacts", handler).BindFunc(utils.RequireAuth)

// Require admin role
e.Router.POST("/api/contacts", handler).BindFunc(utils.RequireAdmin)
```

## Collections

| Collection | Purpose |
|------------|---------|
| `users` | Admin and viewer accounts (Microsoft OAuth) |
| `contacts` | People (unified from Presentations, Awards, Events) |
| `organisations` | Companies and sponsors |
| `activities` | Timeline of events from all apps |
| `app_settings` | App configuration (required for initAppShell) |

## COPE Provider Pattern

CRM is the **canonical source** for contacts and organisations. It projects data to consumers:

### Projection Flow

1. Contact/Organisation created/updated in CRM
2. Webhook hook fires (`webhooks.go`)
3. Payload sent to all configured consumers (Presentations, DAM, Website)
4. HMAC-SHA256 signature in `X-Webhook-Signature` header
5. Consumers store in `contact_projections`/`org_projections` collections

### Projected Statuses

| Status | Projected | Notes |
|--------|-----------|-------|
| `active` | Yes | Normal active record |
| `inactive` | Yes | Temporarily inactive (contacts only) |
| `archived` | No | Hidden from consumers |

### Environment Variables (Projection)

```bash
# Webhook URLs for consumers
PRESENTATIONS_WEBHOOK_URL=https://outlook-apps-presentations.fly.dev/api/webhooks/contact-projection
DAM_WEBHOOK_URL=https://outlook-apps-dam.fly.dev/api/webhooks/contact-projection
WEBSITE_WEBHOOK_URL=https://outlook-apps-website.fly.dev/api/webhooks/contact-projection

# Shared HMAC secret
PROJECTION_WEBHOOK_SECRET=<shared-secret>
```

## Activity Timeline

CRM aggregates activities from all apps via webhooks.

### Receiving Activities

`POST /api/webhooks/activity` receives activity data:

```json
{
  "type": "presentation_accepted",
  "title": "Alice's talk was accepted",
  "contact_id": "abc123",
  "source_app": "presentations",
  "source_id": "pres456",
  "source_url": "https://presentations.theoutlook.io/...",
  "metadata": {},
  "occurred_at": "2026-01-15T10:30:00Z"
}
```

### Activity Types

| Source App | Types |
|------------|-------|
| Presentations | cfp_submitted, session_accepted, session_rejected, presentation_delivered |
| Awards | entry_submitted, entry_shortlisted, entry_winner |
| Events | ticket_purchased, sponsor_committed, event_attended |
| DAM | photo_tagged, asset_featured |
| HubSpot | email_sent, email_opened, meeting_scheduled, note_added |

## Backups

Automated daily backups run at 3 AM AEST via `backend/backup.go`.

### How It Works

1. Go scheduler (`scheduleBackups`) waits until 3 AM AEST
2. Calls PocketBase's `CreateBackup()` API to create a consistent `.zip` snapshot
3. Uploads to Tigris S3 bucket `outlook-apps-backups` under `crm/database/`
4. Deletes local backup file to save volume space
5. Cleans up backups older than 30 days from S3

### Environment Variables (Backup)

| Variable | Description |
|----------|-------------|
| `BACKUP_BUCKET_NAME` | `outlook-apps-backups` |
| `BACKUP_ACCESS_KEY_ID` | Tigris access key |
| `BACKUP_SECRET_ACCESS_KEY` | Tigris secret key |
| `BACKUP_ENDPOINT_URL` | `https://fly.storage.tigris.dev` |

### Verify Backups

```bash
# Check logs for backup status
fly logs -a outlook-apps-crm | grep "\[Backup\]"

# List backups in S3
aws s3 ls s3://outlook-apps-backups/crm/database/ --endpoint-url https://fly.storage.tigris.dev
```

### Recovery

See `../BACKUP-RECOVERY.md` for full recovery procedures. Quick option:
1. Go to https://crm.theoutlook.io/_/
2. Settings > Backups > Upload backup > Restore

## Drupal/Presentations Import

Standalone import tools in `backend/import/organisations/` for migrating data into CRM.

### Import organisations from Presentations

Fetches organisations from Presentations projections API and creates/updates them in CRM:

```bash
go run ./backend/import/organisations \
  -presentations-url http://localhost:8091 \
  -crm-url http://localhost:8090 \
  -crm-email admin@example.com \
  -crm-password yourpassword \
  -update-existing  # optional: update existing orgs with logos/source_ids
```

### Import logos from Drupal

Fetches logos from Drupal's export API and matches them to existing CRM organisations by name:

```bash
go run ./backend/import/organisations logos \
  -drupal-url https://the-outlook.ddev.site \
  -drupal-token YOUR_DRUPAL_TOKEN \
  -crm-url http://localhost:8090 \
  -crm-email admin@example.com \
  -crm-password yourpassword
```

### Import organisation contacts from Drupal

Fetches organisation contacts (sponsor contacts) from Drupal and saves them to the organisation's `contacts` JSON field:

```bash
go run ./backend/import/organisations contacts \
  -drupal-url https://the-outlook.ddev.site \
  -drupal-token YOUR_DRUPAL_TOKEN \
  -crm-url http://localhost:8090 \
  -crm-email admin@example.com \
  -crm-password yourpassword
```

**Drupal field mappings (logos):**
| Drupal field | CRM field |
|--------------|-----------|
| `field_organisation_logo` (image) | `logo_square` |
| `field_organisation_logo_svg` (media ref) | `logo_standard` |
| `field_partner_logo` (image) | `logo_inverted` |

**Drupal field mappings (org contacts):**
| Drupal field | CRM field |
|--------------|-----------|
| `field_contacts[].title` | `contacts[].name` |
| `field_contacts[].uri` | `contacts[].linkedin` |
| `field_contacts[].email` | `contacts[].email` |

**Notes:**
- Drupal export API endpoint: `/api/export/organizations?token=...`
- Organisations are matched by exact name
- Logos are downloaded and re-uploaded to CRM (not linked)
- Organisation contacts are stored in the `contacts` JSON field on organisations (not the core `contacts` collection)

## Page Cleanup Lifecycle

Pages that add event listeners to `document` or `window` MUST register cleanup:

```typescript
import { registerPageCleanup } from '../router';

export async function renderSomePage() {
  const cleanupFns: (() => void)[] = [];

  const handler = () => { /* ... */ };
  document.addEventListener('click', handler);
  cleanupFns.push(() => document.removeEventListener('click', handler));

  registerPageCleanup(() => {
    cleanupFns.forEach(fn => fn());
  });
}
```
