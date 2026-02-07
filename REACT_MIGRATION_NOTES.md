# CRM React Migration Notes

## Goal
Minimal refactor: replace vanilla TS frontend with React + shadcn + shadcnblocks.com

## Key Decisions Made

1. **No custom react-kit library** — shadcnblocks covers all UI needs
2. **Shared across apps = just Tailwind config** (fonts, colors) + small PocketBase hooks
3. **Any custom components** go in app's `components/` folder, nothing fancy
4. **Backend stays unchanged** — same Go/PocketBase, same API endpoints

## Progress

### Completed
- [x] Backup old frontend to `frontend-old/`
- [x] Create new React + Vite frontend
- [x] Install dependencies (react-router@7, @tanstack/react-query, pocketbase, sonner, lucide-react, tailwindcss@4)
- [x] Initialize shadcn with Tailwind v4
- [x] Configure shadcnblocks registry in components.json
- [x] Set up vite.config.ts with proxy and path aliases
- [x] Configure fonts (Monument Grotesk) and brand colors
- [x] Create PocketBase context and auth hook (`src/hooks/use-pocketbase.tsx`)
- [x] Create API service (`src/lib/api.ts`)
- [x] Create app layout with sidebar navigation (`src/components/app-layout.tsx`)
- [x] Create login page with Microsoft OAuth + password form
- [x] Create dashboard page with stats cards
- [x] Create contacts page with table, search, pagination
- [x] Create contact drawer for view/edit/create
- [x] Create organisations page with card grid
- [x] Create organisation drawer for view/edit/create
- [x] Copy fonts and images
- [x] Build passes with no errors

### Remaining
- [ ] Add search functionality to app header
- [ ] Add "project all" admin action
- [ ] Import presenters from Presentations action
- [ ] Activity timeline on contact drawer
- [ ] Logo upload on organisation drawer (DAM integration)
- [ ] Contact roles editing
- [ ] Tags editing
- [ ] App switcher in sidebar footer (from app_settings)

## shadcnblocks.com Setup

API Key: `sk_live_Ywrxmfn-JCyjsU-KWngd0R23zguJwi6l`

Configured in `.env` and `components.json` registries.

Install blocks with:
```bash
npx shadcn add @shadcnblocks/sidebar-X
npx shadcn add @shadcnblocks/application-shell-X
```

## Architecture

```
frontend/
├── src/
│   ├── components/
│   │   ├── ui/           # shadcn components
│   │   ├── app-layout.tsx
│   │   ├── contact-drawer.tsx
│   │   └── organisation-drawer.tsx
│   ├── hooks/
│   │   ├── use-mobile.ts # shadcn hook
│   │   └── use-pocketbase.tsx
│   ├── lib/
│   │   ├── api.ts        # API functions
│   │   ├── pocketbase.ts # PocketBase client + types
│   │   └── utils.ts      # shadcn cn() helper
│   ├── pages/
│   │   ├── login.tsx
│   │   ├── dashboard.tsx
│   │   ├── contacts.tsx
│   │   └── organisations.tsx
│   ├── App.tsx           # Routes
│   ├── main.tsx          # Providers
│   └── index.css         # Tailwind + fonts + brand colors
└── public/
    ├── fonts/
    └── images/
```

## Running

```bash
./start.sh  # Starts PocketBase + Vite dev server
```

Or manually:
```bash
cd frontend && npm run dev
```

## Files to Reference from frontend-old/

- `services/dam-api.ts` — DAM integration for logos
- `components/template.ts` — app shell/search logic (for remaining features)
