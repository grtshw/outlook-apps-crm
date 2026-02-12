# CRM App

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

**Read README.md first** for product context: user roles, key entities, workflows, vocabulary, and prompt tips.

## Commit Preferences

When the user says `commit`, `push`, or `c+p` after making changes, skip verification steps (status, diff, log) and just commit with a sensible message and push. When the user says `deploy` or `d`, just run `fly deploy` without extra checks. Only do full verification for complex or ambiguous situations.

## Shared UI Rules (STRICTLY ENFORCE)

This app uses shared shadcn/ui components symlinked from `outlook-apps-shadcn`. **Read and strictly follow** the rules in `../outlook-apps-shadcn/RULES.md` before making any frontend changes.

Key rules:
- `components/ui/` is a symlink — never edit UI components in this repo, edit them in `outlook-apps-shadcn`
- **Never pass dimension classes to `<SheetContent>`** — the shared component owns all sizing
- Pages pass data and behaviour, not layout decisions

## Development

Start the dev server:
```bash
./start.sh
```

This runs:
- PocketBase backend on http://localhost:8090
- Vite dev server on http://localhost:3000 (proxies API to backend)
- PocketBase admin UI at http://localhost:8090/_/

### Database location

The database lives at **`pb_data/data.db`** (project root), NOT `backend/pb_data/data.db`. The `crm` binary runs from the project root so PocketBase uses `./pb_data/`. The `backend/` directory only contains Go source code.

### Syncing production database

```bash
./scripts/sync-prod.sh --download   # Download prod DB to local
./scripts/sync-prod.sh --upload     # Upload local DB to prod (DANGEROUS)
```

After syncing, set a local password:
```bash
./crm superuser upsert admin@local.dev LocalPassword123
# Then start server, auth as superuser, PATCH the user record's password via API
```

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

## Security (CRITICAL - READ BEFORE ANY CHANGES)

This app handles **sensitive personal information** (PII) including emails, phone numbers, biographical data, and location information. Security is non-negotiable.

### Security Architecture Overview

The CRM implements defense-in-depth security:

| Layer | Implementation | Files |
|-------|----------------|-------|
| **Transport** | HTTPS enforced, HSTS header | `fly.toml`, `main.go` |
| **Headers** | CSP, X-Frame-Options, Referrer-Policy, Permissions-Policy | `main.go:securityHeadersMiddleware` |
| **Rate Limiting** | Per-IP/user sliding window limits | `utils/ratelimit.go` |
| **Authentication** | Microsoft OAuth, session tokens | PocketBase built-in |
| **Authorization** | Role-based (admin/viewer), middleware guards | `utils/auth.go` |
| **Webhook Security** | HMAC-SHA256 signatures | `handlers.go`, `webhooks.go` |
| **Data at Rest** | AES-256-GCM field-level encryption | `utils/crypto.go` |
| **Audit Trail** | All CRUD operations logged | `utils/audit.go` |

### PII Field Encryption

**Encrypted fields in `contacts` collection:**
- `email` - encrypted with AES-256-GCM, has blind index for lookups
- `phone` - encrypted
- `bio` - encrypted
- `location` - encrypted

**How it works:**
1. Data enters via API with plaintext values
2. `OnRecordCreateExecute` / `OnRecordUpdateExecute` hooks encrypt before DB write
3. Database stores `enc:` prefixed base64-encoded ciphertext
4. Response builders (`buildContactResponse`, `buildContactProjection`, etc.) decrypt before returning
5. Blind index (`email_index`) enables email lookups without decrypting all records

**Environment variable:** `ENCRYPTION_KEY` (required in production)

### MANDATORY Security Rules

#### 1. NEVER Store PII in Plaintext
```go
// ❌ WRONG - storing plaintext PII
record.Set("email", userEmail)
record.Set("phone", userPhone)

// ✅ CORRECT - encryption hooks handle this automatically
// Just set the value, hooks encrypt before DB write
record.Set("email", userEmail)  // Hook encrypts this
```

#### 2. ALWAYS Decrypt PII Before Returning to Client
```go
// ❌ WRONG - returning encrypted value
data["email"] = record.GetString("email")

// ✅ CORRECT - decrypt before returning
data["email"] = utils.DecryptField(record.GetString("email"))
```

#### 3. ALWAYS Use Blind Index for Email Lookups
```go
// ❌ WRONG - searching by encrypted email won't work
records, _ := app.FindRecordsByFilter("contacts", "email = {:email}", ...)

// ✅ CORRECT - use blind index
blindIndex := utils.BlindIndex(email)
records, _ := app.FindRecordsByFilter("contacts", "email_index = {:idx}", "", 1, 0, map[string]any{"idx": blindIndex})
```

#### 4. ALWAYS Validate Webhook Signatures
```go
// ❌ WRONG - accepting webhooks without validation
func handleWebhook(re *core.RequestEvent) error {
    var payload MyPayload
    json.NewDecoder(re.Request.Body).Decode(&payload)
    // Process payload...
}

// ✅ CORRECT - validate HMAC signature first
func handleWebhook(re *core.RequestEvent) error {
    bodyBytes, _ := io.ReadAll(re.Request.Body)

    secret := os.Getenv("WEBHOOK_SECRET")
    if secret != "" {
        signature := re.Request.Header.Get("X-Webhook-Signature")
        if signature == "" {
            return re.JSON(http.StatusUnauthorized, map[string]string{"error": "Missing signature"})
        }

        mac := hmac.New(sha256.New, []byte(secret))
        mac.Write(bodyBytes)
        expectedSig := hex.EncodeToString(mac.Sum(nil))

        if !hmac.Equal([]byte(signature), []byte(expectedSig)) {
            return re.JSON(http.StatusUnauthorized, map[string]string{"error": "Invalid signature"})
        }
    }

    // Now safe to process
    var payload MyPayload
    json.Unmarshal(bodyBytes, &payload)
}
```

#### 5. ALWAYS Apply Rate Limiting to New Endpoints
```go
// ❌ WRONG - unprotected endpoint
e.Router.GET("/api/new-endpoint", handler)

// ✅ CORRECT - rate limited
e.Router.GET("/api/new-endpoint", handler).BindFunc(utils.RateLimitAuth)

// For public endpoints:
e.Router.GET("/api/public/new-endpoint", handler).BindFunc(utils.RateLimitPublic)

// For external/webhook endpoints:
e.Router.POST("/api/external/new-endpoint", handler).BindFunc(utils.RateLimitExternalAPI)
```

#### 6. ALWAYS Log Security-Relevant Actions
```go
// ✅ CORRECT - audit log for data changes
utils.LogFromRequest(app, re, "create", "contacts", record.Id, "success", nil, "")

// For failures:
utils.LogFromRequest(app, re, "create", "contacts", "", "failure", nil, err.Error())
```

#### 7. NEVER Expose Encryption Keys or Secrets
```go
// ❌ WRONG - hardcoded secrets
const webhookSecret = "my-secret-key"

// ❌ WRONG - logging secrets
log.Printf("Using key: %s", os.Getenv("ENCRYPTION_KEY"))

// ✅ CORRECT - always from environment, never logged
secret := os.Getenv("WEBHOOK_SECRET")
```

#### 8. NEVER Disable Security Features
```go
// ❌ FORBIDDEN - bypassing rate limiting
// e.Router.GET("/api/contacts", handler)  // No rate limit

// ❌ FORBIDDEN - skipping auth
// e.Router.POST("/api/admin/action", handler)  // No RequireAdmin

// ❌ FORBIDDEN - disabling encryption
// utils.IsEncryptionEnabled = func() bool { return false }
```

### Adding New PII Fields

If you need to add a new field that contains personal information:

1. **Add to encryption list** in `main.go:registerEncryptionHooks`:
```go
piiFields := []string{"email", "phone", "bio", "location", "NEW_FIELD"}
```

2. **Add decryption** in ALL response builders:
   - `handlers.go:buildContactResponse`
   - `handlers.go:buildContactProjection`
   - `webhooks.go:buildContactWebhookPayload`
   - `webhooks.go:buildDAMContactPayload`

3. **Update field max length** if needed (encrypted values are longer than plaintext)

4. **Run migration** to encrypt existing data

### Security Environment Variables

| Variable | Purpose | Required |
|----------|---------|----------|
| `ENCRYPTION_KEY` | AES-256 key derivation (32+ chars) | **YES** in prod |
| `PROJECTION_WEBHOOK_SECRET` | HMAC signing for outbound projections (shared with all apps) | YES |

See `/SECRETS.md` in the parent directory for full secrets management and rotation documentation.

### Migrating Legacy Unencrypted Data

To encrypt existing unencrypted contacts in production:

```bash
# 1. Wake up the machine
curl https://outlook-apps-crm.fly.dev/

# 2. SSH and run migration
fly ssh console -a outlook-apps-crm

# Inside the container:
./crm migrate-encryption
```

The migration script:
- Finds all contacts without `enc:` prefix
- Encrypts PII fields (email, phone, bio, location)
- Sets `email_index` blind index
- Reports progress and results

### Security Incident Response

If you suspect a security issue:

1. **Do NOT** push changes that disable security features
2. **Do** check audit logs: `sqlite3 pb_data/data.db "SELECT * FROM audit_logs ORDER BY created DESC LIMIT 50"`
3. **Do** check rate limit logs: `fly logs -a outlook-apps-crm | grep RateLimit`
4. **Do** rotate secrets if compromised: `fly secrets set ENCRYPTION_KEY="$(openssl rand -base64 32)"`

### Security Review Checklist

Before merging ANY backend changes, verify:

- [ ] No new endpoints without rate limiting
- [ ] No new endpoints without appropriate auth middleware
- [ ] All PII fields encrypted before storage
- [ ] All PII fields decrypted before API response
- [ ] All inbound webhooks validate HMAC signatures
- [ ] All outbound webhooks include HMAC signatures
- [ ] No secrets hardcoded or logged
- [ ] Audit logging added for new data operations
- [ ] SQL injection prevented (parameterized queries only)

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
| `contacts` | People (unified from Presentations, Awards, Events) - **PII fields encrypted** |
| `organisations` | Companies and sponsors |
| `activities` | Timeline of events from all apps |
| `app_settings` | App configuration (required for initAppShell) |
| `audit_logs` | Security audit trail (admin read-only, no API write/delete) |
| `projection_consumers` | Webhook endpoint registry for COPE consumers |

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
