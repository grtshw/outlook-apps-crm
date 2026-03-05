# List & filter patterns audit

Cross-app audit of every list page, how filtering/search/pagination/sorting/bulk actions work, and the inconsistencies between them. Goal: define a standard pattern for `outlook-apps-shadcn`.

---

## Current state by app

### CRM (React + PocketBase)

| Page | Filters | Filter scope | Pagination | Sort | View toggle | Bulk actions | Count display |
|------|---------|--------------|------------|------|-------------|--------------|---------------|
| Contacts | Search (name/email), status dropdown, humanitix event dropdown | Server-side | Offset, 25/page | Fixed: `name` asc | List/Cards (localStorage) | Merge (checkbox multi-select) | "Showing X to Y of Z" |
| Organisations | Search (name), status dropdown | Server-side | Offset, 24/page | Fixed: `name` asc | List/Cards (localStorage) | None | "Showing X to Y of Z" |
| Guest lists | Search (name), status dropdown | Server-side | None (limit 100) | Fixed: `-created` | List only | None | None |
| Guest list detail | None (all loaded) | N/A | None (load all) | Client-side, 9 sortable columns (click header) | List only | Send invites, send followups, add guests | Badge count + RSVP summary |

**Reusable component:** `EntityList<T>` - generic table/card renderer with columns config, loading skeletons, empty state. Used for contacts, organisations, guest lists.

**Pagination UI:** Previous/Next buttons + "Page X of Y" + "Showing N to M of TOTAL".

---

### DAM (React + PocketBase)

| Page | Filters | Filter scope | Pagination | Sort | View toggle | Bulk actions | Count display |
|------|---------|--------------|------------|------|-------------|--------------|---------------|
| People | Type (all/presenters/other), alphabet (A-G/H-N/O-T/U-Z) | Type: server, alphabet: client | Offset, 25/page | Fixed: `name` asc | List/Cards (localStorage) | None | Pagination "X-Y of Z" |
| Organisations | Alphabet (A-G/H-N/O-T/U-Z) | Client | None (load all) | Fixed: `name` asc | List/Cards (localStorage) | None | None |
| Collections | Search (text), type dropdown (conference/brand/campaign) | Client (on paginated results!) | Offset, 48/page | Fixed: `-created` | Cards only | Star toggle | Pagination "X-Y of Z" |
| Events | None | N/A | Offset, 48/page | Fixed: `-edition_year,name` | Cards only | Star toggle | Pagination "X-Y of Z" |
| Tags | None | N/A | Offset, 48/page | Fixed: `name` | Cards only | Star toggle | Pagination "X-Y of Z" |
| Search | Query (AND/OR/phrase/exact), file type checkboxes, orientation, tags (AND/OR), date range | Server-side | Offset, 25/page | Sort dropdown (date/modified/title/type) + asc/desc - **stored but NOT applied to API** | Grid/List + density (compact/comfortable/spacious) | Process, Tag, Add to collection, Delete (selection bar) | "N results" |
| Deleted assets | None (hardcoded `is_deleted=true`) | Server-side | Offset, 25/page | Fixed: `-created` | Cards only | Restore, Delete forever (per-item) | "N deleted assets" |
| Collection detail | Folder selector | Server-side | Offset, 25/page | Sort dropdown - **stored but NOT applied** | Grid/List + density | Same as Search | None |

**Reusable component:** `EntityList<T>` - near-identical copy of CRM's version (different grid columns: `1/2/3` vs CRM's `2/3/4/6`).

**Pagination UI:** First/Prev/numbered pages with ellipsis/Next/Last + "X-Y of Z". More sophisticated than CRM.

**Notable issues:**
- Collections page filters client-side on already-paginated results (filters only affect current page, not total dataset)
- Sort controls on Search and Collection detail are rendered but the values are never sent to the API
- Organisations loads everything with no pagination

---

### Events (React + PocketBase)

| Page | Filters | Filter scope | Pagination | Sort | View toggle | Bulk actions | Count display |
|------|---------|--------------|------------|------|-------------|--------------|---------------|
| Events | Type dropdown, status dropdown, year dropdown | Client (loads all, perPage: 100) | None | Client: `-date`, then name | Pinned=cards, rest=table (automatic, no toggle) | None | None |
| Invoices | Tab bar (pending/assigned/ignored/all), search (debounced 300ms) | Server-side | Offset, 25/page | Fixed: `-date,-created` | Table only | Bulk delete (checkbox) | "N invoices" + "Page X of Y" |
| Bills | Tab bar (pending/assigned/ignored/all), search (debounced 300ms) | Server-side | Offset, 50/page | Fixed: `-date` | Table only | None | "Page X of Y" |
| Transactions | Tab bar (pending/assigned/uncategorized/ignored/all), search (debounced 300ms) | Server-side | Offset, 50/page | Fixed: `-date` | Table only | None | "Page X of Y" |
| Event tickets | None | N/A | None (load all) | None | Table only (inline edit) | None | Count in heading |
| Event sponsors | None | N/A | None (load all) | None | Table only (inline edit) | Bulk delete (checkbox) | "N sponsors" |
| Event sales | None | N/A | None (load all) | None | Table only (inline edit) | None | None |
| Partners | Search (client-side) | Client | None (load all) | None | Table only | None | None |
| Seasons | None | N/A | None | None | Table only | None | None |

**Reusable component:** None - each page builds its own table.

**Pagination UI:** Previous/Next + "Page X of Y". Tab state persisted in URL (`?tab=assigned&page=2`).

**Notable:** Invoices/Bills/Transactions use a consistent tab + search + pagination pattern. Events page loads everything and filters client-side.

---

### Presentations (React + PocketBase)

| Page | Filters | Filter scope | Pagination | Sort | View toggle | Bulk actions | Count display |
|------|---------|--------------|------------|------|-------------|--------------|---------------|
| Presenters | Alphabet (A-G/H-N/O-T/U-Z), search (debounced 300ms) | Server-side | Offset, 50/page | Fixed: `name` asc | Table only | None | "Showing X to Y of Z" + count badge |
| Submissions | Status tabs (All/Missing video/Has video), search | Status: client, search: client | None (load all, limit 1000) | Client-side sortable headers (title, avg rating, like count) | Table only | Bulk delete (checkbox + toolbar) | Tab count badges "All (240)" |
| Organisations | None | N/A | Offset, 50/page | Fixed: `name` asc | Cards only | None | "Showing X to Y of Z" |
| Curated | None | N/A | None (load all) | Fixed: `-created` | Table only | None | None |
| Outcomes | Status tabs (All/Accepted/Shortlisted/Workshop/Rejected/Sent) | Client | None (load all) | None | Table only | Bulk send (checkbox + toolbar with progress bar) | Tab count badges |
| Contracts | Status tabs + event dropdown + search | All client | None (load all) | None | Table only | None | Tab count badges |
| Invitations | Status tabs + event dropdown + search | All client | None (load all) | None | Table only | None | Tab count badges |
| Invoices | Status tabs + category dropdown + search | All client | None (load all) | None | Table only | None | Tab count badges |
| Onboarding | Status dropdown + search | All client | None (load all) | None | Table only | None | None |
| Programming | Type + status + year dropdowns | Client | None (load all) | None | Pinned=cards, rest=table | None | None |

**Reusable component:** None - each page builds its own table. Has a `BulkActionsToolbar` component for bulk operations.

**Pagination UI:** Numbered page buttons + "Showing X to Y of Z".

**Notable:** Most pages load all data (limit 500-1000) and filter entirely client-side. Only Presenters and Organisations use server-side pagination.

---

### Awards (React + PocketBase)

| Page | Filters | Filter scope | Pagination | Sort | View toggle | Bulk actions | Count display |
|------|---------|--------------|------------|------|-------------|--------------|---------------|
| Admin entries | Search, status dropdown, category dropdown | All client (backend supports server-side but unused) | None (limit 500) | Fixed: `-created` | Table only | None | "X of Y entries" |
| Admin codes | Gala dropdown | Client | None (limit 500) | Fixed: `-created` | Table only | Move to gala, activate/deactivate, delete (checkbox + bottom bar) | "X of Y" + "N selected" |
| Admin categories | None | N/A | None | Fixed: sort_order/name | Tracks=cards, categories=table | None | None |
| Admin galas | None | N/A | None | None | Cards only | None | None |
| Juror invitations | Gala dropdown | Server-side | None (no limit) | Fixed: `-expires_at` | Table only | Bulk revoke | Count shown |
| Juror applications | Status dropdown | Client | None | Fixed: `-id` | Table only | None | None |
| Juror accounts | None | N/A | None | None | Table only | None | None |
| Juror entries | Category dropdown, search | Server-side | None (limit 500) | Fixed: `category,company_name` | Table only | None | Entry count |

**Reusable component:** None - each page builds its own table.

**Pagination UI:** None - no pages are paginated.

**Notable:** Zero pagination anywhere. Everything loads with hard limits (500). Backend has filter support that the frontend doesn't use.

---

## Inconsistency summary

### 1. Filtering scope (the biggest problem)

| Pattern | Apps using it | Problem |
|---------|---------------|---------|
| Server-side filter + server-side pagination | CRM contacts/orgs, DAM people/search, Events invoices/bills/txns, Presentations presenters | Correct approach |
| Client-side filter on fully-loaded data | Events events, Presentations most pages, Awards all pages | Works for small datasets but won't scale |
| **Client-side filter on paginated server data** | **DAM collections (search + type dropdown)** | **Broken - only filters current page, not total dataset** |
| Load all with hard limit, no pagination | Awards (500), Presentations submissions (1000), CRM guest lists (100) | Silent data loss if dataset exceeds limit |

### 2. Pagination

| Pattern | Where used |
|---------|------------|
| Offset + Previous/Next + "Page X of Y" + "Showing X to Y of Z" | CRM |
| Offset + First/Prev/numbered/Next/Last + "X-Y of Z" | DAM |
| Offset + Previous/Next + "Page X of Y" | Events |
| Offset + numbered pages + "Showing X to Y of Z" | Presentations (2 pages only) |
| No pagination at all | Awards (every page), Presentations (most pages), Events (events/tickets/sponsors) |

### 3. Search

| Pattern | Where used |
|---------|------------|
| Server-side via API `search` param | CRM, DAM search, Events invoices/bills/txns, Presentations presenters |
| Client-side `.filter()` on loaded data | Events partners, Presentations submissions/contracts/invitations/invoices/onboarding, Awards entries |
| Debounce timing | 300ms (Events, Presentations), immediate (CRM via React Query key change) |

### 4. Quick filters (tabs vs dropdowns)

| Pattern | Where used |
|---------|------------|
| Status as `<Select>` dropdown | CRM contacts/orgs, Awards entries, Presentations onboarding |
| Status as tab bar with count badges | Events invoices/bills/txns, Presentations submissions/outcomes/contracts/invitations/invoices |
| Alphabet buttons (All/A-G/H-N/O-T/U-Z) | DAM people/orgs, Presentations presenters |
| Type/category as `<Select>` dropdown | CRM humanitix, Events events, Awards entries/codes, Presentations contracts/invoices |

### 5. View toggle

| Pattern | Where used |
|---------|------------|
| List/Cards toggle (localStorage) | CRM contacts/orgs, DAM people/orgs |
| Grid/List + density control (localStorage + URL) | DAM search/collection detail |
| Auto layout (pinned=cards, rest=table) | Events events, Presentations programming |
| No toggle (table only) | Everything else |

### 6. Sort

| Pattern | Where used |
|---------|------------|
| Fixed server-side sort (no UI) | CRM, DAM most pages, Events, Presentations presenters/orgs, Awards |
| Client-side clickable column headers | CRM guest list detail, Presentations submissions |
| Sort dropdown (stored but not applied!) | DAM search/collection detail |

### 7. Bulk actions

| Pattern | Where used |
|---------|------------|
| Checkbox + bottom toolbar | Awards codes, Presentations submissions/outcomes |
| Checkbox + inline bar above table | Events sponsors/invoices |
| Checkbox + merge dialog | CRM contacts |
| Selection bar (fixed bottom) | DAM search/collection detail |
| No bulk actions | Most pages |

### 8. Results count

| Pattern | Where used |
|---------|------------|
| "Showing X to Y of Z items" | CRM, Presentations presenters |
| "X-Y of Z" (in pagination) | DAM |
| "N items" (simple count) | Events sponsors, DAM search/deleted |
| "N invoices" + "Page X of Y" | Events invoices |
| Tab badges "All (240)" | Presentations, Events |
| "X of Y" (filtered of total) | Awards entries/codes |
| No count shown | Many pages |

### 9. Empty states

| Pattern | Where used |
|---------|------------|
| Centered message + CTA button | Events events, DAM |
| Table row spanning all columns | Events invoices/bills/txns |
| Simple centered text | CRM (via EntityList) |
| Icon + heading + description | Presentations |
| No dedicated empty state | Awards |

### 10. URL state

| What's in URL | Where |
|---------------|-------|
| Tab + page | Events invoices/bills/txns |
| Filters + page + view + density + sort | DAM search/collection detail |
| Selected item ID (drawer) | DAM people, Events transactions, Presentations presenters |
| Nothing (all in React state) | CRM, Awards, Presentations most pages |

---

## Duplicate code

Both CRM and DAM have near-identical `EntityList<T>` components that should be in `outlook-apps-shadcn`. The only differences:
- CRM card grid: `grid-cols-2 md:grid-cols-3 lg:grid-cols-4 xl:grid-cols-6`
- DAM card grid: `grid-cols-1 md:grid-cols-2 lg:grid-cols-3`
- DAM's `onItemClick` is optional

Presentations has a `BulkActionsToolbar` that could be shared.

The shared library already has `FilterButton`, `PageHeader`, `Table`, `Select`, `Tabs`, `Card`, `Badge` - the primitives exist but there's no composed list pattern.

---

## Proposed standard

### Principles

1. **All filters that affect the dataset must be server-side.** Never filter client-side on paginated data. If a page is paginated, every filter must hit the API and reset to page 1.
2. **Paginate by default.** Any list that could exceed ~50 items should be paginated. Hard limits without pagination are a bug.
3. **URL is the source of truth for list state.** Filters, search, page, sort, and view should all be in URL search params so pages are shareable and back-button works.
4. **Consistent UI vocabulary.** Same filter type = same component across all apps.

### Standard list anatomy

```
+--PageHeader (title + action buttons)-------------------+
|                                                         |
+--FilterBar---------------------------------------------+
| [Search input]  [Quick filter tabs/pills]  [Dropdowns] |
|                              [View toggle]  [Sort]      |
+--BulkActionBar (conditional)----------------------------+
| [N selected]  [Action] [Action] [Clear]                 |
+---------------------------------------------------------+
|                                                         |
|  EntityList (table or card grid)                        |
|  - Loading: skeleton rows/cards                         |
|  - Empty: centered message + optional CTA               |
|  - Data: table rows or responsive card grid             |
|                                                         |
+---------------------------------------------------------+
|                                                         |
+--Pagination---------------------------------------------+
| Showing X to Y of Z          [<] [1] [2] ... [>]       |
+---------------------------------------------------------+
```

### Component hierarchy (for `outlook-apps-shadcn`)

| Component | Purpose | New or existing |
|-----------|---------|-----------------|
| `PageHeader` | Title + action buttons | Existing |
| `FilterBar` | Container for search + filters + view toggle | **New** |
| `SearchInput` | Debounced search input (300ms) | **New** |
| `QuickFilterTabs` | Horizontal tab-style pills with optional count badges | **New** (wraps existing `Tabs`) |
| `AlphabetFilter` | All / A-G / H-N / O-T / U-Z buttons | **New** |
| `FilterButton` | Dropdown trigger showing label + value | Existing |
| `ViewToggle` | List/Cards icon toggle | **New** |
| `SortControl` | Sort field + direction dropdown | **New** |
| `EntityList<T>` | Generic table/card renderer with columns, loading, empty | **New** (merge CRM + DAM versions) |
| `ListPagination` | "Showing X to Y of Z" + numbered page buttons | **New** |
| `BulkActionBar` | Fixed/sticky bar with selected count + action buttons | **New** |
| `EmptyState` | Icon + heading + description + optional CTA | **New** |

### Standard behaviors

**Search:**
- Always 300ms debounce
- Always server-side (sent as `search` query param)
- Always resets page to 1
- Placeholder: "Search {entity}..."

**Quick filters (status, type, etc.):**
- Use `QuickFilterTabs` when there are 2-6 mutually exclusive options (status is the canonical case)
- Show count badges when the count is cheap to compute
- Use `Select` dropdown when there are 7+ options or the options are dynamic
- Always server-side, always reset page to 1
- "All" option is always first and is the default

**Alphabet filter:**
- Use only for name-sorted entity lists (people, organisations)
- Groups: All, A-G, H-N, O-T, U-Z
- Always server-side (backend filters by first character range)
- Resets page to 1

**Pagination:**
- Default 25 items/page (50 for dense lists like presenters)
- Show "Showing X to Y of Z {entity}" on the left
- Show page buttons: First / Prev / numbered with ellipsis / Next / Last
- Hide pagination entirely when totalPages <= 1
- URL param: `page=N`

**Sort:**
- Only show sort UI if the user needs to change sort order
- Default sort should be sensible (name asc for entities, -created for feeds/timelines)
- When sort UI is present, it must actually be sent to the API
- URL params: `sort=field&order=asc`

**View toggle:**
- Only offer when both views are meaningfully different (list shows tabular data, cards show visual previews)
- Persist to localStorage (keyed per page) AND URL param `view=list|cards`
- Default: list for data-heavy pages, cards for visual pages

**Bulk actions:**
- Checkbox in first column (header checkbox for select all visible)
- Sticky bar appears at bottom when selection is non-empty
- Shows "N selected" + action buttons + clear button
- Selection clears on navigation or filter change

**Empty state:**
- Centered in the list area
- Optional icon (muted, large)
- Heading: "No {entity} found" (when filters active) or "No {entity} yet" (when no filters)
- Optional description
- Optional CTA button (only when no filters)

**URL state:**
- All list state in URL search params: `?search=foo&status=active&page=2&sort=name&order=asc&view=cards`
- Use `useSearchParams` with `replace: true` for filter changes
- Changing any filter resets `page` to 1

### Per-page sizes

| Entity type | Per page | Rationale |
|-------------|----------|-----------|
| People/contacts | 25 | Names + details need vertical space |
| Organisations | 24 | Cards: divisible by 3 and 4 columns |
| Assets/media | 25 | Visual grid needs breathing room |
| Entries/submissions | 50 | Dense tabular data, scan quickly |
| Financial records | 50 | Dense tabular data |
| Small collections (tags, categories) | 48 | Cards: divisible by 3, 4, 6 |

---

## Migration priority

### High priority (broken behavior)
1. **DAM collections**: Client-side filter on paginated data - filters only affect current page
2. **DAM sort controls**: Sort dropdown rendered but never sent to API
3. **Awards**: Zero pagination anywhere - will break as data grows

### Medium priority (scaling risk)
4. **Presentations**: Most pages load all data (500-1000 limit) with client-side filtering
5. **Events events page**: Loads all events with client-side filtering
6. **CRM guest lists**: Hard limit 100, no pagination

### Low priority (consistency)
7. Unify `EntityList` from CRM and DAM into shared component
8. Extract `BulkActionBar`, `SearchInput`, `QuickFilterTabs`, `AlphabetFilter`, `ListPagination`, `ViewToggle`, `EmptyState` into shared components
9. Move all list state into URL params across all apps
10. Standardize count display and empty states
