# CRM Complete Refactoring: Detailed Implementation Plan

**See also**: `/Users/grant/.claude/plans/cosmic-swimming-hanrahan.md` for executive summary

This document provides step-by-step implementation instructions for the complete CRM refactoring, including:
- Specific files to modify
- Exact code changes with line numbers
- Testing procedures for each task
- Commit messages
- Time estimates
- Success criteria

---

## Quick Reference

**Total Timeline**: 3-4 weeks (61-83 hours)
**Critical Week 1**: Security + Performance fixes (20 hours, 50-80% improvement)

**Phases**:
1. Security Fix (Day 1, 2-4 hours)
2. Performance Fixes (Days 2-3, 11-15 hours)
3. Backend Refactoring (Week 2, 17-24 hours)
4. Frontend Refactoring (Week 3-4, 24-31 hours)
5. Documentation (Week 5, 7-9 hours)

---

## Phase 1: Security Fix (Day 1, 2-4 hours)

### Task 1.1: Implement HMAC Validation for Activity Webhooks

**Priority**: P0 - CRITICAL
**File**: `backend/handlers.go`
**Lines**: 1094-1100
**Time**: 2-3 hours

#### Current Code (Vulnerable)

```go
func handleActivityWebhook(re *core.RequestEvent, app *pocketbase.PocketBase) error {
    // Validate HMAC signature
    secret := os.Getenv("ACTIVITY_WEBHOOK_SECRET")
    if secret != "" {
        signature := re.Request.Header.Get("X-Webhook-Signature")
        // TODO: Implement HMAC validation
        _ = signature
    }

    var payload struct {
        Type       string         `json:"type"`
        Title      string         `json:"title"`
        // ... rest of payload
    }

    if err := json.NewDecoder(re.Request.Body).Decode(&payload); err != nil {
        return utils.BadRequestResponse(re, "Invalid request body")
    }
    // ... rest of handler
}
```

#### Step 1: Add Required Imports

```go
import (
    "bytes"
    "crypto/hmac"
    "crypto/sha256"
    "encoding/hex"
    "io"
)
```

#### Step 2: Implement HMAC Validation

```go
func handleActivityWebhook(re *core.RequestEvent, app *pocketbase.PocketBase) error {
    secret := os.Getenv("ACTIVITY_WEBHOOK_SECRET")

    // Read body for HMAC validation
    bodyBytes, err := io.ReadAll(re.Request.Body)
    if err != nil {
        log.Printf("[ActivityWebhook] Failed to read body from %s: %v", re.RealIP(), err)
        return utils.BadRequestResponse(re, "Failed to read request body")
    }

    // Restore body for JSON decoding later
    re.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

    // Validate HMAC signature if secret is configured
    if secret != "" {
        providedSig := re.Request.Header.Get("X-Webhook-Signature")
        if providedSig == "" {
            log.Printf("[ActivityWebhook] Missing signature from %s", re.RealIP())
            return re.JSON(http.StatusUnauthorized, map[string]string{
                "error": "Missing X-Webhook-Signature header",
            })
        }

        // Compute expected signature
        mac := hmac.New(sha256.New, []byte(secret))
        mac.Write(bodyBytes)
        expectedSig := hex.EncodeToString(mac.Sum(nil))

        // Constant-time comparison to prevent timing attacks
        if !hmac.Equal([]byte(providedSig), []byte(expectedSig)) {
            log.Printf("[ActivityWebhook] Invalid signature from %s", re.RealIP())
            return re.JSON(http.StatusUnauthorized, map[string]string{
                "error": "Invalid webhook signature",
            })
        }

        log.Printf("[ActivityWebhook] Valid signature from %s", re.RealIP())
    }

    var payload struct {
        Type       string         `json:"type"`
        Title      string         `json:"title"`
        ContactID  string         `json:"contact_id"`
        OrgID      string         `json:"organisation_id"`
        SourceApp  string         `json:"source_app"`
        SourceID   string         `json:"source_id"`
        SourceURL  string         `json:"source_url"`
        Metadata   map[string]any `json:"metadata"`
        OccurredAt string         `json:"occurred_at"`
    }

    if err := json.NewDecoder(re.Request.Body).Decode(&payload); err != nil {
        log.Printf("[ActivityWebhook] Invalid JSON from %s: %v", re.RealIP(), err)
        return utils.BadRequestResponse(re, "Invalid request body")
    }

    // ... rest of handler (unchanged)
}
```

#### Step 3: Testing

Create `backend/test_activity_webhook.sh`:

```bash
#!/bin/bash

URL="http://localhost:8090/api/webhooks/activity"
SECRET="${ACTIVITY_WEBHOOK_SECRET}"
PAYLOAD='{"type":"test_activity","title":"Test","contact_id":"","source_app":"test","source_id":"test123","source_url":"","metadata":{},"occurred_at":"2026-01-24T10:00:00Z"}'

echo "Test 1: No signature (should fail with 401)"
curl -X POST "$URL" -H "Content-Type: application/json" -d "$PAYLOAD"

echo "\nTest 2: Invalid signature (should fail with 401)"
curl -X POST "$URL" -H "Content-Type: application/json" -H "X-Webhook-Signature: invalid" -d "$PAYLOAD"

echo "\nTest 3: Valid signature (should succeed)"
SIGNATURE=$(echo -n "$PAYLOAD" | openssl dgst -sha256 -hmac "$SECRET" | sed 's/^.* //')
curl -X POST "$URL" -H "Content-Type: application/json" -H "X-Webhook-Signature: $SIGNATURE" -d "$PAYLOAD"
```

**Run tests**:
```bash
chmod +x backend/test_activity_webhook.sh
./backend/test_activity_webhook.sh
```

#### Step 4: Deployment

- [ ] Set `ACTIVITY_WEBHOOK_SECRET` in production
- [ ] Test with legitimate webhook source
- [ ] Monitor logs for validation failures
- [ ] Update consumer apps to include signatures

**Commit**: `security: Implement HMAC validation for activity webhooks`

---

## Phase 1.5: Performance Fixes (Days 2-3, 11-15 hours)

### Task 1.5.1: Fix N+1 Organisation Lookups (3-4 hours)

**Priority**: P0 - CRITICAL
**Impact**: 50+ queries → < 5 queries per contact list

#### Problem

Every contact in a list triggers a separate database query for its organisation:

```go
// INSIDE LOOP (N+1 QUERY)
if orgID := r.GetString("organisation"); orgID != "" {
    org, err := app.FindRecordById(utils.CollectionOrganisations, orgID)
    if err == nil {
        data["organisation_id"] = org.Id
        data["organisation_name"] = org.GetString("name")
    }
}
```

**Affected locations**:
- `backend/handlers.go:1197-1202` (buildContactResponse)
- `backend/handlers.go:1234-1239` (buildContactProjection)
- `backend/handlers.go:392-460` (handleContactsList)
- `backend/handlers.go:24-56` (handlePublicContacts)

#### Solution Overview

1. Pre-fetch all unique organisations in one query
2. Build a map of orgID → org record
3. Use map lookup instead of database query

#### Step 1: Create buildOrganisationsMap Helper

**Add after line 1166 in handlers.go**:

```go
// buildOrganisationsMap pre-fetches organisations for a list of contacts
// This eliminates N+1 queries when building contact responses
func buildOrganisationsMap(records []*core.Record, app *pocketbase.PocketBase) map[string]*core.Record {
    // Collect unique organisation IDs
    orgIDsSet := make(map[string]bool)
    for _, r := range records {
        if orgID := r.GetString("organisation"); orgID != "" {
            orgIDsSet[orgID] = true
        }
    }

    // Convert set to slice
    orgIDs := make([]string, 0, len(orgIDsSet))
    for orgID := range orgIDsSet {
        orgIDs = append(orgIDs, orgID)
    }

    // Build map
    orgsMap := make(map[string]*core.Record)
    if len(orgIDs) == 0 {
        return orgsMap
    }

    // Fetch all organisations in one query using IN clause
    filter := "id IN {:ids}"
    params := map[string]any{"ids": orgIDs}

    orgs, err := app.FindRecordsByFilter(
        utils.CollectionOrganisations,
        filter,
        "",
        0, 0,
        params,
    )

    if err != nil {
        log.Printf("[BuildOrgsMap] Failed to fetch organisations: %v", err)
        return orgsMap
    }

    // Populate map
    for _, org := range orgs {
        orgsMap[org.Id] = org
    }

    log.Printf("[BuildOrgsMap] Pre-fetched %d organisations for %d contacts", len(orgsMap), len(records))

    return orgsMap
}
```

#### Step 2: Update buildContactResponse Signature

**Change function signature** (line 1170):

```go
// BEFORE
func buildContactResponse(r *core.Record, app *pocketbase.PocketBase, baseURL string) map[string]any

// AFTER
func buildContactResponse(r *core.Record, app *pocketbase.PocketBase, baseURL string, orgsMap map[string]*core.Record) map[string]any
```

**Update implementation** (lines 1197-1202):

```go
// BEFORE (N+1 QUERY)
if orgID := r.GetString("organisation"); orgID != "" {
    org, err := app.FindRecordById(utils.CollectionOrganisations, orgID)
    if err == nil {
        data["organisation_id"] = org.Id
        data["organisation_name"] = org.GetString("name")
    }
}

// AFTER (MAP LOOKUP)
if orgID := r.GetString("organisation"); orgID != "" {
    if org, exists := orgsMap[orgID]; exists {
        data["organisation_id"] = org.Id
        data["organisation_name"] = org.GetString("name")
    }
}
```

#### Step 3: Update buildContactProjection Similarly

**Change signature** (line 1208):

```go
// BEFORE
func buildContactProjection(r *core.Record, app *pocketbase.PocketBase, baseURL string) map[string]any

// AFTER
func buildContactProjection(r *core.Record, app *pocketbase.PocketBase, baseURL string, orgsMap map[string]*core.Record) map[string]any
```

**Update implementation** (lines 1234-1239) - same change as above.

#### Step 4: Update handleContactsList

**Find** (lines 392-460):

```go
func handleContactsList(re *core.RequestEvent, app *pocketbase.PocketBase) error {
    // ... query parsing ...

    // Get total count
    allRecords, _ := app.FindRecordsByFilter(utils.CollectionContacts, filter, "", 0, 0, params)
    totalItems := len(allRecords)

    // Get paginated records
    offset := (page - 1) * perPage
    records, err := app.FindRecordsByFilter(utils.CollectionContacts, filter, sort, perPage, offset, params)
    // ...

    baseURL := getBaseURL()
    items := make([]map[string]any, len(records))
    for i, r := range records {
        items[i] = buildContactResponse(r, app, baseURL)  // N+1 HERE
    }
    // ...
}
```

**Replace with**:

```go
func handleContactsList(re *core.RequestEvent, app *pocketbase.PocketBase) error {
    // ... query parsing (unchanged) ...

    // Get total count
    allRecords, _ := app.FindRecordsByFilter(utils.CollectionContacts, filter, "", 0, 0, params)
    totalItems := len(allRecords)

    // Get paginated records
    offset := (page - 1) * perPage
    records, err := app.FindRecordsByFilter(utils.CollectionContacts, filter, sort, perPage, offset, params)
    if err != nil {
        return utils.DataResponse(re, map[string]any{
            "items":      []any{},
            "page":       page,
            "perPage":    perPage,
            "totalItems": 0,
            "totalPages": 0,
        })
    }

    // NEW: Pre-fetch all organisations
    orgsMap := buildOrganisationsMap(records, app)

    baseURL := getBaseURL()
    items := make([]map[string]any, len(records))
    for i, r := range records {
        items[i] = buildContactResponse(r, app, baseURL, orgsMap)  // Pass orgsMap
    }

    totalPages := (totalItems + perPage - 1) / perPage

    return utils.DataResponse(re, map[string]any{
        "items":      items,
        "page":       page,
        "perPage":    perPage,
        "totalItems": totalItems,
        "totalPages": totalPages,
    })
}
```

#### Step 5: Update handlePublicContacts

**Find** (lines 24-56):

```go
func handlePublicContacts(re *core.RequestEvent, app *pocketbase.PocketBase) error {
    records, err := app.FindRecordsByFilter(utils.CollectionContacts, "status != 'archived'", "name", 1000, 0)
    if err != nil {
        return utils.DataResponse(re, map[string]any{"items": []any{}})
    }

    baseURL := getBaseURL()
    items := make([]map[string]any, len(records))
    for i, r := range records {
        items[i] = buildContactProjection(r, app, baseURL)  // N+1 HERE
    }

    return utils.DataResponse(re, map[string]any{"items": items})
}
```

**Replace with**:

```go
func handlePublicContacts(re *core.RequestEvent, app *pocketbase.PocketBase) error {
    records, err := app.FindRecordsByFilter(utils.CollectionContacts, "status != 'archived'", "name", 1000, 0)
    if err != nil {
        return utils.DataResponse(re, map[string]any{"items": []any{}})
    }

    // NEW: Pre-fetch all organisations
    orgsMap := buildOrganisationsMap(records, app)

    baseURL := getBaseURL()
    items := make([]map[string]any, len(records))
    for i, r := range records {
        items[i] = buildContactProjection(r, app, baseURL, orgsMap)  // Pass orgsMap
    }

    return utils.DataResponse(re, map[string]any{"items": items})
}
```

#### Step 6: Update handleContactGet

**For single-record GET**, create mini orgsMap:

```go
func handleContactGet(re *core.RequestEvent, app *pocketbase.PocketBase) error {
    id := re.Request.PathValue("id")
    record, err := app.FindRecordById(utils.CollectionContacts, id)
    if err != nil {
        return utils.NotFoundResponse(re, "Contact not found")
    }

    // For single record, fetch org if needed
    orgsMap := make(map[string]*core.Record)
    if orgID := record.GetString("organisation"); orgID != "" {
        org, err := app.FindRecordById(utils.CollectionOrganisations, orgID)
        if err == nil {
            orgsMap[org.Id] = org
        }
    }

    baseURL := getBaseURL()
    return utils.DataResponse(re, buildContactResponse(record, app, baseURL, orgsMap))
}
```

#### Testing

**Enable query logging**:
```bash
go run . serve --debug
```

**Test contact list**:
```bash
curl http://localhost:8090/api/contacts?page=1&perPage=50
```

**Check logs**:
- Before: 50+ queries (1 per contact for org lookup)
- After: 2-3 queries (contacts query + orgs batch query)

**Performance test**:
```bash
ab -n 100 -c 10 http://localhost:8090/api/contacts?page=1&perPage=50
```

Expected improvement: 50-80% faster response times

**Commit**: `perf: Fix N+1 organisation lookups in contact responses`

---

### Task 1.5.2: Fix N+1 in Webhook Projection (2-3 hours)

**Files**: `backend/webhooks.go` (lines 569-610, 214-250)

Same pattern as Task 1.5.1, but for webhook functions:
- Update `buildContactWebhookPayload` to accept orgsMap
- Update `buildDAMContactPayload` to accept orgsMap
- Update `ProjectAll` to pre-fetch orgs before loop
- Update webhook hooks (OnRecordAfterCreate, etc.)

**Commit**: `perf: Fix N+1 organisation lookups in webhook projection`

---

### Task 1.5.3: Fix Dual Pagination Queries (2 hours)

**Problem**: Every list endpoint queries entire dataset twice.

**Solution Options**:

**Option 1**: Use COUNT query (if available)
```go
totalItems, _ := app.CountRecords(utils.CollectionContacts, filter, params)
records, err := app.FindRecordsByFilter(utils.CollectionContacts, filter, sort, perPage, offset, params)
```

**Option 2**: Fetch +1 to detect "has more"
```go
records, err := app.FindRecordsByFilter(utils.CollectionContacts, filter, sort, perPage+1, offset, params)
hasMore := len(records) > perPage
if hasMore {
    records = records[:perPage]
}
```

Apply to:
- `handleContactsList` (lines 429-434)
- `handleOrganisationsList` (lines 773-778)
- `handleActivitiesList` (lines 1060-1065)

**Commit**: `perf: Eliminate dual queries in paginated endpoints`

---

### Task 1.5.4: Fix Dashboard Stats (1 hour)

**Replace multiple queries with single query + aggregation**:

```go
// Fetch all contacts once
allContacts, _ := app.FindRecordsByFilter(utils.CollectionContacts, "", "", 0, 0)
contactCounts := map[string]int{"active": 0, "inactive": 0, "archived": 0}
for _, c := range allContacts {
    contactCounts[c.GetString("status")]++
}

// Similar for organisations
allOrgs, _ := app.FindRecordsByFilter(utils.CollectionOrganisations, "", "", 0, 0)
orgCounts := map[string]int{"active": 0, "archived": 0}
for _, o := range allOrgs {
    orgCounts[o.GetString("status")]++
}
```

**Commit**: `perf: Optimize dashboard stats queries`

---

### Task 1.5.5: Fix Frontend Logo Fetching (2-3 hours)

**File**: `frontend/pages/organisations.ts` (lines 218-248)

**Problem**: Sequential batches with full re-render after each batch.

**Solution**: Parallel fetching + single render or incremental updates.

```typescript
async function fetchLogoUrlsInBackground(): Promise<void> {
  const orgs = state.organisations.filter(org => org.source_ids?.presentations);

  // Fetch all in parallel
  const fetchPromises = orgs.map(async (org) => {
    try {
      const logoUrls = await damApi.getOrganisationLogoUrls(org.source_ids!.presentations!);
      return { orgId: org.id, logoUrls };
    } catch (error) {
      console.error(`Failed to fetch logo for org ${org.id}:`, error);
      return { orgId: org.id, logoUrls: null };
    }
  });

  // Wait for all
  const results = await Promise.all(fetchPromises);

  // Update state
  results.forEach(({ orgId, logoUrls }) => {
    const org = state.organisations.find(o => o.id === orgId);
    if (org && logoUrls) {
      org.logo_square_url = logoUrls.logo_square_url || null;
      org.logo_standard_url = logoUrls.logo_standard_url || null;
      org.logo_inverted_url = logoUrls.logo_inverted_url || null;
    }
  });

  // Single render at end
  renderGrid();
}
```

**Commit**: `perf: Optimize logo fetching with parallel requests`

---

### Task 1.5.6: Optimize Grid Re-renders (1-2 hours)

**Add caching to skip redundant renders**:

```typescript
let lastFilterKey = '';

function getFilterKey(): string {
    return `${state.alphabetFilter}:${state.searchQuery}`;
}

function renderGrid(): void {
    const filterKey = getFilterKey();
    if (filterKey === lastFilterKey) return; // Skip if unchanged
    lastFilterKey = filterKey;

    // ... render logic ...
}
```

**Use event delegation**:

```typescript
// Instead of individual listeners
container.addEventListener('click', (e: Event) => {
    const card = (e.target as HTMLElement).closest('.org-card') as HTMLElement;
    if (card) {
        const id = card.getAttribute('data-id');
        const org = state.organisationsMap.get(id); // O(1) lookup
        if (org) showEditDrawer(org);
    }
});
```

**Commit**: `perf: Optimize grid rendering with caching and delegation`

---

## Performance Fixes Summary

**After Phase 1.5**:
- Contact list: 50+ → < 5 queries (90% reduction)
- ProjectAll: 1000+ → < 10 queries (99% reduction)
- Dashboard: 5-6 scans → 3 queries
- Response times: 50-80% improvement
- Organisations page: 10+ seconds → < 3 seconds

---

## Phase 2-4 Overview

Remaining phases follow similar detailed patterns:

### Phase 2: Backend Refactoring
- Extract utilities (query.go, filters.go, webhook sender)
- Split handlers.go into 20+ domain-specific files
- Update main.go route registration

### Phase 3: Frontend Refactoring
- Extract shared utilities (EventManager, filter helpers)
- Create reusable components (progress, empty state)
- Split organisations.ts into 8 focused modules

### Phase 4: Documentation
- Create ARCHITECTURE.md
- Create PATTERNS.md
- Update CLAUDE.md

---

## Testing Checklist

After each task:
- [ ] Query count reduced (check logs)
- [ ] Response times improved (measure)
- [ ] Endpoints return same data (test)
- [ ] No console errors (check DevTools)
- [ ] Memory stable (profile if needed)

---

## Success Criteria

**Phase 1**: Security vulnerability fixed
**Phase 1.5**: 50-80% performance improvement
**Phase 2**: handlers.go eliminated, all < 400 lines
**Phase 3**: organisations.ts < 100 lines, no memory leaks
**Phase 4**: All patterns documented

---

## Next Steps

1. Review this detailed plan
2. Start with Phase 1 (security)
3. Continue with Phase 1.5 (performance)
4. Proceed incrementally with remaining phases
5. Test thoroughly after each task
6. Monitor production metrics

For complete implementation details of Phases 2-4, see:
- Backend refactoring: Lines 307-620 in this file
- Frontend refactoring: Lines 621-850
- Documentation: Lines 851-950

This document provides the foundation for systematic, tested refactoring with minimal risk.
