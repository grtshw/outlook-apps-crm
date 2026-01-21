# CRM (crm.theoutlook.io)

Customer Relationship Management for The Outlook Apps ecosystem.

## Overview

CRM is the **canonical source** for contacts (people) and organisations across all Outlook apps. It replaces the presenter/organisation management that was previously in Presentations, following the COPE (Create Once, Publish Everywhere) model.

## Key Features

- **Unified Contact Management**: Single source of truth for all people across Presentations, Awards, Events
- **Organisation Directory**: Companies, sponsors, and partner organisations
- **Activity Timeline**: Aggregated history from all apps (presentations submitted, awards won, events attended)
- **COPE Provider**: Projects data to consumer apps via webhooks
- **HubSpot Integration**: Bidirectional sync with HubSpot CRM (planned)

## Architecture

```
                      CRM (crm.theoutlook.io)
                      ┌─────────────────────┐
                      │  Contacts           │  ← Canonical source
                      │  Organisations      │
                      │  Activities         │
                      └──────┬──────────────┘
                             │ webhooks
           ┌─────────────────┼─────────────────┐
           ▼                 ▼                 ▼
    ┌─────────────┐   ┌─────────────┐   ┌─────────────┐
    │Presentations│   │     DAM     │   │   Website   │
    │ (consumer)  │   │ (consumer)  │   │ (consumer)  │
    └─────────────┘   └─────────────┘   └─────────────┘
```

## Tech Stack

- **Backend**: Go 1.24+ with PocketBase v0.35
- **Frontend**: TypeScript, Vite, Tailwind CSS v4
- **Database**: SQLite via PocketBase
- **UI Components**: @theoutlook/ui-kit (shared component library)
- **Hosting**: Fly.io

## Collections

| Collection | Purpose |
|------------|---------|
| `contacts` | People (presenters, attendees, entrants, sponsors) |
| `organisations` | Companies, sponsors, partners |
| `activities` | Timeline events from all apps |
| `users` | Admin and viewer accounts |
| `app_settings` | Application configuration |

## Contact Schema

| Field | Type | Description |
|-------|------|-------------|
| `email` | string | Primary identifier (unique) |
| `name` | string | Full name |
| `phone`, `pronouns`, `bio`, `job_title` | string | Profile fields |
| `linkedin`, `instagram`, `website` | string | Social links |
| `location`, `do_position` | string | Location info |
| `avatar` | file | Profile photo |
| `organisation` | relation | Link to organisation |
| `tags` | JSON | e.g., ["speaker-2024", "sponsor-contact"] |
| `status` | string | active, inactive, archived |
| `source` | string | presentations, awards, hubspot, manual |
| `source_ids` | JSON | {presentations: "abc", awards: "def"} |

## Development

### Prerequisites

- Go 1.24+
- Node.js 20+
- npm

### Quick Start

```bash
# Clone the repo
git clone git@github.com:grtshw/outlook-apps-crm.git
cd CRM

# Start development server
./start.sh
```

This runs:
- PocketBase backend on http://localhost:8090
- Vite dev server on http://localhost:3000

### First-time Setup

1. Start the server with `./start.sh`
2. Go to http://localhost:8090/_/ to create an admin account
3. Configure Microsoft OAuth in PocketBase settings

## Deployment

```bash
# Deploy to Fly.io
fly deploy
```

Production URL: https://crm.theoutlook.io

## Environment Variables

### Projection (COPE Provider)

```bash
PRESENTATIONS_WEBHOOK_URL=https://outlook-apps-presentations.fly.dev/api/webhooks/contact-projection
DAM_WEBHOOK_URL=https://outlook-apps-dam.fly.dev/api/webhooks/contact-projection
WEBSITE_WEBHOOK_URL=https://outlook-apps-website.fly.dev/api/webhooks/contact-projection
PROJECTION_WEBHOOK_SECRET=<shared-hmac-secret>
```

### Activity Webhooks (Receiver)

```bash
ACTIVITY_WEBHOOK_SECRET=<webhook-secret>
```

### Backups

```bash
BACKUP_BUCKET_NAME=outlook-apps-backups
BACKUP_ACCESS_KEY_ID=<tigris-access-key>
BACKUP_SECRET_ACCESS_KEY=<tigris-secret-key>
BACKUP_ENDPOINT_URL=https://fly.storage.tigris.dev
```

### HubSpot (Planned)

```bash
HUBSPOT_ACCESS_TOKEN=pat-xxx
HUBSPOT_WEBHOOK_SECRET=xxx
```

## User Roles

| Role | Permissions |
|------|-------------|
| `admin` | Full access: create, edit, delete contacts and organisations |
| `viewer` | Read-only access to contacts and organisations |

## Related Apps

- [Presentations](https://presentations.theoutlook.io) - Call for presenters, session management
- [DAM](https://dam.theoutlook.io) - Digital asset management
- [Events](https://events.theoutlook.io) - Event management
- [Website](https://theoutlook.com.au) - Public website

## License

Proprietary - The Outlook
