import { api, Organisation } from '../services/api';
import { damApi, type UploadToken } from '../services/dam-api';
import { attachRouterLinks, registerPageCleanup } from '../router';
import {
  escapeHtml,
  showToast,
  renderCardSkeleton,
  showDeleteConfirmation,
  setDocumentTitle,
  icon,
  renderBadge,
  renderPageTemplate,
  registerAction,
  extractErrorMessage,
  showDrawer,
  renderDrawerSection,
  attachDrawerSectionHandlers,
  type DrawerController,
} from '@theoutlook/ui-kit';
import { preparePageTemplate } from '../components/template';
import { createPageContext } from '../utils/page-context';

/**
 * Get HMAC upload token from CRM backend for DAM operations
 */
async function getLogoUploadToken(orgId: string, type: string, action: string): Promise<UploadToken> {
  const response = await fetch(`/api/organisations/${orgId}/logo/${type}/token`, {
    method: 'POST',
    headers: {
      Authorization: api.pb.authStore.token,
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({ action }),
  });

  if (!response.ok) {
    const error = await response.json().catch(() => ({}));
    throw new Error(error.error || 'Failed to get upload token');
  }

  return response.json();
}

type AlphabetFilter = 'all' | 'a-g' | 'h-n' | 'o-t' | 'u-z';

const ALPHABET_RANGES: Record<AlphabetFilter, string[]> = {
  'all': [],
  'a-g': ['a', 'b', 'c', 'd', 'e', 'f', 'g'],
  'h-n': ['h', 'i', 'j', 'k', 'l', 'm', 'n'],
  'o-t': ['o', 'p', 'q', 'r', 's', 't'],
  'u-z': ['u', 'v', 'w', 'x', 'y', 'z'],
};

function getAlphabetFilterFromUrl(): AlphabetFilter {
  const params = new URLSearchParams(window.location.search);
  const filter = params.get('alpha');
  if (filter && filter in ALPHABET_RANGES) return filter as AlphabetFilter;
  return 'all';
}

function updateUrlAlphabetFilter(filter: AlphabetFilter) {
  const url = new URL(window.location.href);
  if (filter === 'all') {
    url.searchParams.delete('alpha');
  } else {
    url.searchParams.set('alpha', filter);
  }
  window.history.pushState({}, '', url.toString());
}

function filterByAlphabet<T extends { name: string }>(items: T[], filter: AlphabetFilter): T[] {
  if (filter === 'all') return items;
  const range = ALPHABET_RANGES[filter];
  return items.filter(item => {
    const firstChar = item.name.charAt(0).toLowerCase();
    return range.includes(firstChar);
  });
}

interface OrganisationsState {
  organisations: Organisation[];
  totalItems: number;
  searchQuery: string;
  alphabetFilter: AlphabetFilter;
}

let state: OrganisationsState = {
  organisations: [],
  totalItems: 0,
  searchQuery: '',
  alphabetFilter: 'all',
};

export async function renderOrganisationsPage(): Promise<void> {
  setDocumentTitle('Organisations');

  preparePageTemplate();

  const context = createPageContext();

  // Extract org ID from URL for deep linking (e.g., /organisations/abc123)
  const pathMatch = window.location.pathname.match(/^\/organisations\/([^/]+)$/);
  const deepLinkOrgId = pathMatch?.[1];

  state.alphabetFilter = getAlphabetFilterFromUrl();

  // Reset state
  state = {
    organisations: [],
    totalItems: 0,
    searchQuery: '',
    alphabetFilter: getAlphabetFilterFromUrl(),
  };

  // Build alphabet filter tabs for header
  const alphabetTabs = [
    { id: 'all', label: 'All', active: state.alphabetFilter === 'all' },
    { id: 'a-g', label: 'A–G', active: state.alphabetFilter === 'a-g' },
    { id: 'h-n', label: 'H–N', active: state.alphabetFilter === 'h-n' },
    { id: 'o-t', label: 'O–T', active: state.alphabetFilter === 'o-t' },
    { id: 'u-z', label: 'U–Z', active: state.alphabetFilter === 'u-z' },
  ];
  const headerContent = `
    <div class="flex items-center gap-1 border-l border-gray-300 pl-4 ml-1">
      ${alphabetTabs.map(f => `
        <button
          data-alpha-filter="${f.id}"
          class="px-3 py-1 text-sm rounded-md transition-colors ${f.active ? 'bg-gray-900 text-white' : 'text-gray-600 hover:bg-gray-100'}"
        >${f.label}</button>
      `).join('')}
    </div>
  `;

  await renderPageTemplate(
    {
      title: 'Organisations',
      headerContent,
      actions: api.isAdmin()
        ? [
            {
              id: 'add-org',
              label: 'Add organisation',
              icon: 'plus-lg',
              variant: 'primary',
              type: 'action',
              action: 'showAddOrganisation',
            },
          ]
        : [],
      render: async (container) => {
        // Show loading state
        container.innerHTML = `
          <div class="mb-4">
            <input
              type="text"
              id="search-input"
              class="input w-full"
              placeholder="Search organisations..."
            />
          </div>
          <div id="organisations-grid">
            ${renderCardSkeleton({ count: 6, columns: 3 })}
          </div>
        `;

        // Register action for header button
        registerAction('showAddOrganisation', showCreateDrawer);

        // Load data
        await loadOrganisations();

        // Render grid
        renderGrid();

        // Attach handlers
        attachHandlers();

        // Handle deep link - open drawer for specific organisation
        if (deepLinkOrgId) {
          const org = state.organisations.find((o) => o.id === deepLinkOrgId);
          if (org) {
            setTimeout(() => showEditDrawer(org), 100);
          }
        }
      },
    },
    context
  );

  // Register cleanup
  registerPageCleanup(() => {
    // Cleanup any listeners
  });
}

async function loadOrganisations(): Promise<void> {
  try {
    const result = await api.getOrganisations({
      page: 1,
      perPage: 500,
      search: state.searchQuery,
    });

    state.organisations = result.items;
    state.totalItems = result.totalItems;

    // Fetch logo URLs from DAM in background (batch of 10 concurrent requests)
    fetchLogoUrlsInBackground();
  } catch (error) {
    showToast('Failed to load organisations', 'error');
  }
}

/**
 * Fetch logo URLs from DAM for all organisations in batches
 * Updates the grid progressively as URLs are fetched
 */
async function fetchLogoUrlsInBackground(): Promise<void> {
  const BATCH_SIZE = 10;
  const orgs = state.organisations;

  for (let i = 0; i < orgs.length; i += BATCH_SIZE) {
    const batch = orgs.slice(i, i + BATCH_SIZE);
    const promises = batch.map(async (org) => {
      // Skip if already has logo URLs
      if (org.logo_square_url || org.logo_standard_url || org.logo_inverted_url) {
        return;
      }

      // Use Presentations org ID for DAM lookup (DAM stores orgs by Presentations ID)
      const presentationsId = org.source_ids?.presentations;
      if (!presentationsId) {
        return;
      }

      const logoUrls = await damApi.getOrganisationLogoUrls(presentationsId);
      if (logoUrls) {
        org.logo_square_url = logoUrls.square || undefined;
        org.logo_standard_url = logoUrls.standard || undefined;
        org.logo_inverted_url = logoUrls.inverted || undefined;
      }
    });

    await Promise.all(promises);

    // Update grid after each batch
    renderGrid();
  }
}

function getFilteredOrganisations(): Organisation[] {
  let filtered = state.organisations;

  // Apply alphabet filter
  filtered = filterByAlphabet(filtered, state.alphabetFilter);

  // Apply search filter (client-side for instant feedback)
  if (state.searchQuery) {
    const query = state.searchQuery.toLowerCase();
    filtered = filtered.filter((o) => o.name.toLowerCase().includes(query));
  }

  return filtered;
}

function getPreviewLogo(org: Organisation): string | null {
  if (org.logo_square_url) return org.logo_square_url;
  if (org.logo_standard_url) return org.logo_standard_url;
  if (org.logo_inverted_url) return org.logo_inverted_url;
  return null;
}

function countLogos(org: Organisation): number {
  let count = 0;
  if (org.logo_square_url) count++;
  if (org.logo_standard_url) count++;
  if (org.logo_inverted_url) count++;
  return count;
}

function getInitial(name: string): string {
  return name.charAt(0).toUpperCase();
}

function getHostname(url: string): string {
  try {
    return new URL(url).hostname;
  } catch {
    return url;
  }
}

function renderGrid(): void {
  const container = document.getElementById('organisations-grid');
  if (!container) return;

  const filtered = getFilteredOrganisations();
  const isAdmin = api.isAdmin();

  if (filtered.length === 0) {
    container.innerHTML = `
      <div class="bg-white rounded-lg border border-gray-200 p-12 text-center">
        <div class="text-gray-400 mb-4">
          ${icon('building', { class: 'w-16 h-16 mx-auto' })}
        </div>
        <h3 class="text-lg text-gray-900 mb-2">${state.searchQuery ? 'No organisations found' : 'No organisations yet'}</h3>
        <p class="text-gray-500 mb-6">${state.searchQuery ? 'Try a different search term.' : 'Add your first organisation to get started.'}</p>
        ${!state.searchQuery && isAdmin ? '<button id="empty-add-btn" class="btn">Add your first organisation</button>' : ''}
      </div>
    `;

    container.querySelector('#empty-add-btn')?.addEventListener('click', showCreateDrawer);
    return;
  }

  container.innerHTML = `
    <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
      ${filtered
        .map((org) => {
          const logoUrl = getPreviewLogo(org);
          const logoCount = countLogos(org);

          return `
            <div class="bg-white rounded-lg border border-gray-200 hover:shadow-md transition-shadow p-6 cursor-pointer org-card" data-id="${org.id}">
              <div class="flex items-center gap-4">
                ${
                  logoUrl
                    ? `
                  <div class="w-16 h-16 flex-shrink-0 flex items-center justify-center bg-gray-50 rounded-lg p-2">
                    <img src="${escapeHtml(logoUrl)}" alt="${escapeHtml(org.name)}" class="max-w-full max-h-full object-contain" />
                  </div>
                `
                    : `
                  <div class="w-16 h-16 rounded-lg bg-brand-purple flex items-center justify-center flex-shrink-0">
                    <span class="text-white text-xl">${escapeHtml(getInitial(org.name))}</span>
                  </div>
                `
                }
                <div class="flex-1 min-w-0">
                  <h3 class="text-lg text-gray-900 truncate">${escapeHtml(org.name)}</h3>
                  <div class="flex items-center gap-2 mt-1">
                    ${renderBadge({ label: org.status, variant: org.status === 'active' ? 'success' : 'secondary' })}
                    ${logoCount > 0 ? `<span class="text-xs text-gray-500">${logoCount} logo${logoCount !== 1 ? 's' : ''}</span>` : ''}
                  </div>
                  ${
                    org.website
                      ? `
                    <a href="${escapeHtml(org.website)}" target="_blank" rel="noopener"
                       class="text-xs text-brand-green hover:underline truncate block mt-1"
                       onclick="event.stopPropagation()">
                      ${escapeHtml(getHostname(org.website))}
                    </a>
                  `
                      : ''
                  }
                </div>
              </div>
            </div>
          `;
        })
        .join('')}
    </div>
  `;

  // Card click handlers
  container.querySelectorAll('.org-card').forEach((card) => {
    card.addEventListener('click', () => {
      const id = card.getAttribute('data-id');
      if (id) {
        const org = state.organisations.find((o) => o.id === id);
        if (org) showEditDrawer(org);
      }
    });
  });

  attachRouterLinks(container);
}

function attachHandlers(): void {
  const searchInput = document.getElementById('search-input') as HTMLInputElement;
  if (searchInput) {
    let debounceTimer: ReturnType<typeof setTimeout>;
    searchInput.addEventListener('input', () => {
      clearTimeout(debounceTimer);
      debounceTimer = setTimeout(() => {
        state.searchQuery = searchInput.value;
        renderGrid();
      }, 150);
    });
  }

  // Attach alphabet filter handlers
  document.querySelectorAll('[data-alpha-filter]').forEach(btn => {
    btn.addEventListener('click', () => {
      const filterId = (btn as HTMLElement).dataset.alphaFilter as AlphabetFilter;
      state.alphabetFilter = filterId;
      updateUrlAlphabetFilter(filterId);
      renderGrid();

      // Update button styles
      document.querySelectorAll('[data-alpha-filter]').forEach(b => {
        const isActive = (b as HTMLElement).dataset.alphaFilter === filterId;
        b.className = `px-3 py-1 text-sm rounded-md transition-colors ${isActive ? 'bg-gray-900 text-white' : 'text-gray-600 hover:bg-gray-100'}`;
      });
    });
  });
}

function showCreateDrawer(): void {
  const drawerRef: { current: DrawerController | null } = { current: null };

  // Section: Details
  const detailsContent = `
    <div class="space-y-4">
      <div>
        <label class="block text-sm text-gray-700 mb-1">Organisation name *</label>
        <input type="text" name="name" class="input w-full" placeholder="e.g., Acme Corporation" required />
      </div>
      <div class="grid grid-cols-2 gap-4">
        <div>
          <label class="block text-sm text-gray-700 mb-1">Website</label>
          <input type="url" name="website" class="input w-full" placeholder="https://example.com" />
        </div>
        <div>
          <label class="block text-sm text-gray-700 mb-1">LinkedIn</label>
          <input type="url" name="linkedin" class="input w-full" placeholder="https://linkedin.com/company/..." />
        </div>
      </div>
    </div>
  `;

  // Section: Descriptions
  const descriptionsContent = `
    <div class="space-y-4">
      <div>
        <label class="block text-sm text-gray-700 mb-1">Short description <span class="text-gray-400">(max 50)</span></label>
        <input type="text" name="description_short" class="input w-full" maxlength="50" placeholder="Brief tagline" />
      </div>
      <div>
        <label class="block text-sm text-gray-700 mb-1">Medium description <span class="text-gray-400">(max 150)</span></label>
        <textarea name="description_medium" class="input w-full" rows="2" maxlength="150" placeholder="One paragraph summary"></textarea>
      </div>
      <div>
        <label class="block text-sm text-gray-700 mb-1">Long description <span class="text-gray-400">(max 500)</span></label>
        <textarea name="description_long" class="input w-full" rows="4" maxlength="500" placeholder="Full description"></textarea>
      </div>
    </div>
  `;

  const content = `
    <form id="create-form">
      ${renderDrawerSection({
        id: 'details',
        title: 'Details',
        content: detailsContent,
        collapsible: false,
      })}
      ${renderDrawerSection({
        id: 'descriptions',
        title: 'Descriptions',
        content: descriptionsContent,
        collapsible: true,
        collapsed: true,
      })}
      <p class="text-xs text-gray-500 mt-4">Logos can be added after creation.</p>
    </form>
  `;

  const handleCreate = async () => {
    const panel = drawerRef.current?.getPanel();
    if (!panel) return;

    const form = panel.querySelector('#create-form') as HTMLFormElement;
    const formData = new FormData(form);
    const name = formData.get('name') as string;

    if (!name) {
      showToast('Please enter organisation name', 'error');
      return;
    }

    try {
      await api.createOrganisation({
        name,
        website: formData.get('website') as string || undefined,
        linkedin: formData.get('linkedin') as string || undefined,
        description_short: formData.get('description_short') as string || undefined,
        description_medium: formData.get('description_medium') as string || undefined,
        description_long: formData.get('description_long') as string || undefined,
        status: 'active',
      });
      await loadOrganisations();
      renderGrid();
      drawerRef.current?.close();
      showToast('Organisation created', 'success');
    } catch (err) {
      showToast(extractErrorMessage(err, 'Failed to create organisation'), 'error');
      console.error(err);
    }
  };

  drawerRef.current = showDrawer({
    title: 'Add organisation',
    content,
    width: 'lg',
    actions: [
      { label: 'Cancel', variant: 'secondary', onClick: () => drawerRef.current?.close() },
      { label: 'Create', variant: 'primary', onClick: handleCreate },
    ],
    onOpen: () => {
      const panel = drawerRef.current?.getPanel();
      if (panel) {
        attachDrawerSectionHandlers(panel);
      }
    },
  });
}

async function showEditDrawer(org: Organisation): Promise<void> {
  // Track contacts
  type OrgContact = { name: string; linkedin?: string; email?: string };
  let currentContacts: OrgContact[] = org.contacts ? [...org.contacts] : [];

  const drawerRef: { current: DrawerController | null } = { current: null };
  const isAdmin = api.isAdmin();

  // Section: Details
  const detailsContent = `
    <div class="space-y-4">
      <div>
        <label class="block text-sm text-gray-700 mb-1">Organisation name *</label>
        <input type="text" name="name" class="input w-full" value="${escapeHtml(org.name)}" ${!isAdmin ? 'disabled' : ''} required />
      </div>
      <div class="grid grid-cols-2 gap-4">
        <div>
          <label class="block text-sm text-gray-700 mb-1">Website</label>
          <input type="url" name="website" class="input w-full" value="${escapeHtml(org.website || '')}" ${!isAdmin ? 'disabled' : ''} placeholder="https://example.com" />
        </div>
        <div>
          <label class="block text-sm text-gray-700 mb-1">LinkedIn</label>
          <input type="url" name="linkedin" class="input w-full" value="${escapeHtml(org.linkedin || '')}" ${!isAdmin ? 'disabled' : ''} placeholder="https://linkedin.com/company/..." />
        </div>
      </div>
      <div>
        <label class="block text-sm text-gray-700 mb-1">Status</label>
        <select name="status" class="input w-full" ${!isAdmin ? 'disabled' : ''}>
          <option value="active" ${org.status === 'active' ? 'selected' : ''}>Active</option>
          <option value="archived" ${org.status === 'archived' ? 'selected' : ''}>Archived</option>
        </select>
      </div>
    </div>
  `;

  // Section: Descriptions
  const descriptionsContent = `
    <div class="space-y-4">
      <div>
        <label class="block text-sm text-gray-700 mb-1">Short description <span class="text-gray-400">(max 50)</span></label>
        <input type="text" name="description_short" class="input w-full" maxlength="50" value="${escapeHtml(org.description_short || '')}" ${!isAdmin ? 'disabled' : ''} placeholder="Brief tagline" />
        <div class="text-xs text-gray-400 text-right mt-1"><span id="short-count">${(org.description_short || '').length}</span>/50</div>
      </div>
      <div>
        <label class="block text-sm text-gray-700 mb-1">Medium description <span class="text-gray-400">(max 150)</span></label>
        <textarea name="description_medium" class="input w-full" rows="2" maxlength="150" ${!isAdmin ? 'disabled' : ''} placeholder="One paragraph summary">${escapeHtml(org.description_medium || '')}</textarea>
        <div class="text-xs text-gray-400 text-right mt-1"><span id="medium-count">${(org.description_medium || '').length}</span>/150</div>
      </div>
      <div>
        <label class="block text-sm text-gray-700 mb-1">Long description <span class="text-gray-400">(max 500)</span></label>
        <textarea name="description_long" class="input w-full" rows="4" maxlength="500" ${!isAdmin ? 'disabled' : ''} placeholder="Full description">${escapeHtml(org.description_long || '')}</textarea>
        <div class="text-xs text-gray-400 text-right mt-1"><span id="long-count">${(org.description_long || '').length}</span>/500</div>
      </div>
    </div>
  `;

  // Section: Logo variants
  const renderLogoVariantsContent = () => `
    <div class="space-y-4">
      <div>
        <label class="block text-sm text-gray-700 mb-2">Square</label>
        <div id="logo-square-container" class="flex items-center gap-3">
          ${org.logo_square_url ? `
            <div class="w-16 h-16 rounded bg-gray-100 overflow-hidden flex items-center justify-center">
              <img src="${escapeHtml(org.logo_square_url)}" alt="" class="max-w-full max-h-full object-contain" />
            </div>
            ${isAdmin ? `<button type="button" class="delete-logo-btn btn btn-sm btn-secondary text-red-600" data-type="logo_square">Remove</button>` : ''}
          ` : isAdmin ? `
            <button type="button" class="upload-logo-btn btn btn-sm btn-secondary" data-type="logo_square">Upload Square</button>
            <span class="text-xs text-gray-500">PNG, JPG or SVG</span>
          ` : `<span class="text-xs text-gray-500">No square logo</span>`}
        </div>
      </div>
      <div>
        <label class="block text-sm text-gray-700 mb-2">Standard</label>
        <div id="logo-standard-container" class="flex items-center gap-3">
          ${org.logo_standard_url ? `
            <div class="w-16 h-16 rounded bg-gray-100 overflow-hidden flex items-center justify-center">
              <img src="${escapeHtml(org.logo_standard_url)}" alt="" class="max-w-full max-h-full object-contain" />
            </div>
            ${isAdmin ? `<button type="button" class="delete-logo-btn btn btn-sm btn-secondary text-red-600" data-type="logo_standard">Remove</button>` : ''}
          ` : isAdmin ? `
            <button type="button" class="upload-logo-btn btn btn-sm btn-secondary" data-type="logo_standard">Upload Standard</button>
            <span class="text-xs text-gray-500">PNG, JPG or SVG</span>
          ` : `<span class="text-xs text-gray-500">No standard logo</span>`}
        </div>
      </div>
      <div>
        <label class="block text-sm text-gray-700 mb-2">Inverted</label>
        <div id="logo-inverted-container" class="flex items-center gap-3">
          ${org.logo_inverted_url ? `
            <div class="w-16 h-16 rounded bg-gray-800 overflow-hidden flex items-center justify-center">
              <img src="${escapeHtml(org.logo_inverted_url)}" alt="" class="max-w-full max-h-full object-contain" />
            </div>
            ${isAdmin ? `<button type="button" class="delete-logo-btn btn btn-sm btn-secondary text-red-600" data-type="logo_inverted">Remove</button>` : ''}
          ` : isAdmin ? `
            <button type="button" class="upload-logo-btn btn btn-sm btn-secondary" data-type="logo_inverted">Upload Inverted</button>
            <span class="text-xs text-gray-500">PNG, JPG or SVG</span>
          ` : `<span class="text-xs text-gray-500">No inverted logo</span>`}
        </div>
      </div>
      <input type="file" id="logo-file-input" accept="image/*,.svg" class="hidden" />
    </div>
  `;

  // Section: Contacts
  const renderContactsContent = () => `
    <div id="contacts-list" class="space-y-3">
      ${currentContacts.length === 0 ? `
        <p class="text-gray-500 text-sm">No contacts added yet.</p>
      ` : currentContacts.map((contact, idx) => `
        <div class="flex items-start gap-2 p-3 bg-gray-50 rounded-lg" data-idx="${idx}">
          <div class="flex-1 space-y-2">
            <input type="text" class="contact-name input w-full" value="${escapeHtml(contact.name)}" placeholder="Contact name" ${!isAdmin ? 'disabled' : ''} />
            <input type="email" class="contact-email input w-full" value="${escapeHtml(contact.email || '')}" placeholder="Email" ${!isAdmin ? 'disabled' : ''} />
            <input type="url" class="contact-linkedin input w-full" value="${escapeHtml(contact.linkedin || '')}" placeholder="LinkedIn URL" ${!isAdmin ? 'disabled' : ''} />
          </div>
          ${isAdmin ? `
            <button type="button" class="remove-contact-btn p-2 text-red-500 hover:text-red-600" data-idx="${idx}">
              ${icon('x-lg', { class: 'w-4 h-4' })}
            </button>
          ` : ''}
        </div>
      `).join('')}
    </div>
    ${isAdmin ? `
      <button type="button" id="add-contact-btn" class="mt-3 text-sm text-brand-green hover:text-brand-green-hover flex items-center gap-1">
        ${icon('plus-lg', { class: 'w-4 h-4' })}
        Add contact
      </button>
    ` : ''}
  `;

  // Section: Source info (read-only)
  const sourceContent = org.source ? `
    <div class="space-y-2 text-sm text-gray-500">
      <p>Source: <span class="text-gray-900">${escapeHtml(org.source)}</span></p>
      <p>Created: <span class="text-gray-900">${new Date(org.created).toLocaleDateString()}</span></p>
      <p>Updated: <span class="text-gray-900">${new Date(org.updated).toLocaleDateString()}</span></p>
    </div>
  ` : `
    <div class="space-y-2 text-sm text-gray-500">
      <p>Created: <span class="text-gray-900">${new Date(org.created).toLocaleDateString()}</span></p>
      <p>Updated: <span class="text-gray-900">${new Date(org.updated).toLocaleDateString()}</span></p>
    </div>
  `;

  const logoCount = countLogos(org);

  // Form content
  const formContent = `
    <form id="edit-form">
      ${renderDrawerSection({
        id: 'details',
        title: 'Details',
        content: detailsContent,
        collapsible: false,
      })}
      ${renderDrawerSection({
        id: 'logos',
        title: 'Logos',
        content: renderLogoVariantsContent(),
        collapsible: true,
        collapsed: logoCount === 0,
        badge: logoCount > 0 ? logoCount : undefined,
      })}
      ${renderDrawerSection({
        id: 'contacts',
        title: 'Contacts',
        content: renderContactsContent(),
        collapsible: true,
        collapsed: currentContacts.length === 0,
        badge: currentContacts.length > 0 ? currentContacts.length : undefined,
      })}
      ${renderDrawerSection({
        id: 'descriptions',
        title: 'Descriptions',
        content: descriptionsContent,
        collapsible: true,
        collapsed: true,
      })}
      ${renderDrawerSection({
        id: 'source',
        title: 'Created/updated',
        content: sourceContent,
        collapsible: true,
        collapsed: true,
      })}
    </form>
  `;

  // Handle save
  const handleSave = async () => {
    const panel = drawerRef.current?.getPanel();
    if (!panel) return;

    const editForm = panel.querySelector('#edit-form') as HTMLFormElement;
    if (!editForm) return;

    const formData = new FormData(editForm);
    const name = formData.get('name') as string;

    if (!name) {
      showToast('Organisation name is required', 'error');
      return;
    }

    try {
      // Filter out empty contacts
      const validContacts = currentContacts.filter(c => c.name.trim() || c.email?.trim() || c.linkedin?.trim());

      await api.updateOrganisation(org.id, {
        name,
        website: formData.get('website') as string || undefined,
        linkedin: formData.get('linkedin') as string || undefined,
        status: formData.get('status') as 'active' | 'archived',
        description_short: formData.get('description_short') as string || undefined,
        description_medium: formData.get('description_medium') as string || undefined,
        description_long: formData.get('description_long') as string || undefined,
        contacts: validContacts,
      });
      await loadOrganisations();
      renderGrid();
      drawerRef.current?.close();
      showToast('Organisation updated', 'success');
    } catch (err) {
      showToast(extractErrorMessage(err, 'Failed to update organisation'), 'error');
      console.error(err);
    }
  };

  // Handle delete
  const handleDelete = () => {
    showDeleteConfirmation({
      itemName: 'organisation',
      itemTitle: org.name,
      onConfirm: async () => {
        try {
          await api.deleteOrganisation(org.id);
          await loadOrganisations();
          renderGrid();
          drawerRef.current?.close();
          showToast('Organisation deleted', 'success');
        } catch (err) {
          showToast(extractErrorMessage(err, 'Failed to delete organisation'), 'error');
        }
      },
    });
  };

  // Build actions
  const actions = isAdmin
    ? [
        { label: 'Delete', variant: 'danger' as const, onClick: handleDelete },
        { label: 'Cancel', variant: 'secondary' as const, onClick: () => drawerRef.current?.close() },
        { label: 'Save changes', variant: 'primary' as const, onClick: handleSave },
      ]
    : [{ label: 'Close', variant: 'secondary' as const, onClick: () => drawerRef.current?.close() }];

  // Create drawer
  drawerRef.current = showDrawer({
    title: isAdmin ? `Edit ${org.name}` : org.name,
    content: formContent,
    width: 'lg',
    actions,
    onOpen: () => {
      const panel = drawerRef.current?.getPanel();
      if (!panel) return;

      // Update URL to deep link
      window.history.replaceState({}, '', `/organisations/${org.id}`);

      // Attach section handlers
      attachDrawerSectionHandlers(panel);
      attachAllHandlers(panel);
    },
    onClose: () => {
      // Reset URL back to list
      window.history.replaceState({}, '', '/organisations');
    },
  });

  // Helper function to attach all handlers
  const attachAllHandlers = (panel: HTMLElement) => {
    // Character counter handlers
    const shortInput = panel.querySelector('input[name="description_short"]') as HTMLInputElement;
    const mediumInput = panel.querySelector('textarea[name="description_medium"]') as HTMLTextAreaElement;
    const longInput = panel.querySelector('textarea[name="description_long"]') as HTMLTextAreaElement;

    shortInput?.addEventListener('input', () => {
      const count = panel.querySelector('#short-count');
      if (count) count.textContent = String(shortInput.value.length);
    });
    mediumInput?.addEventListener('input', () => {
      const count = panel.querySelector('#medium-count');
      if (count) count.textContent = String(mediumInput.value.length);
    });
    longInput?.addEventListener('input', () => {
      const count = panel.querySelector('#long-count');
      if (count) count.textContent = String(longInput.value.length);
    });

    // Logo upload handlers (only for admin)
    if (isAdmin) {
      let currentLogoType: string | null = null;
      const fileInput = panel.querySelector('#logo-file-input') as HTMLInputElement;

      panel.querySelectorAll('.upload-logo-btn').forEach((btn) => {
        btn.addEventListener('click', () => {
          currentLogoType = (btn as HTMLElement).dataset.type || null;
          fileInput?.click();
        });
      });

      panel.querySelectorAll('.delete-logo-btn').forEach((btn) => {
        btn.addEventListener('click', async () => {
          const logoType = (btn as HTMLElement).dataset.type;
          if (!logoType) return;

          try {
            // Delete logo via DAM API with HMAC token
            const typeValue = logoType.replace('logo_', ''); // square, standard, inverted
            await damApi.deleteOrganisationLogo(org.id, typeValue, getLogoUploadToken);

            // Update local org state
            (org as Record<string, unknown>)[logoType] = null;
            (org as Record<string, unknown>)[`${logoType}_url`] = null;
            // Re-render just this section
            const container = panel.querySelector(`#logo-${typeValue}-container`);
            if (container) {
              container.innerHTML = `
                <button type="button" class="upload-logo-btn btn btn-sm btn-secondary" data-type="${logoType}">Upload ${typeValue.charAt(0).toUpperCase() + typeValue.slice(1)}</button>
                <span class="text-xs text-gray-500">PNG, JPG or SVG</span>
              `;
              // Re-attach handler
              container.querySelector('.upload-logo-btn')?.addEventListener('click', () => {
                currentLogoType = logoType;
                fileInput?.click();
              });
            }
            showToast('Logo removed', 'success');
          } catch (err) {
            showToast('Failed to remove logo', 'error');
          }
        });
      });

      fileInput?.addEventListener('change', async () => {
        const file = fileInput.files?.[0];
        if (!file || !currentLogoType) return;

        try {
          // Upload via DAM API with HMAC token
          const logoType = currentLogoType.replace('logo_', ''); // square, standard, inverted
          const logoUrls = await damApi.uploadOrganisationLogo(org.id, logoType, file, getLogoUploadToken);

          // Update local org state with returned URLs
          const logoUrlKey = `logo_${logoType}_url`;
          (org as Record<string, unknown>)[currentLogoType] = 'dam'; // Mark as stored in DAM
          (org as Record<string, unknown>)[logoUrlKey] = logoUrls[logoType as keyof typeof logoUrls];

          // Re-render just this section
          const container = panel.querySelector(`#logo-${logoType}-container`);
          const newUrl = logoUrls[logoType as keyof typeof logoUrls];
          if (container && newUrl) {
            const bgClass = logoType === 'inverted' ? 'bg-gray-800' : 'bg-gray-100';
            container.innerHTML = `
              <div class="w-16 h-16 rounded ${bgClass} overflow-hidden flex items-center justify-center">
                <img src="${escapeHtml(newUrl)}" alt="" class="max-w-full max-h-full object-contain" />
              </div>
              <button type="button" class="delete-logo-btn btn btn-sm btn-secondary text-red-600" data-type="${currentLogoType}">Remove</button>
            `;
            // Re-attach delete handler
            const savedLogoType = currentLogoType;
            container.querySelector('.delete-logo-btn')?.addEventListener('click', async () => {
              try {
                await damApi.deleteOrganisationLogo(org.id, logoType, getLogoUploadToken);

                (org as Record<string, unknown>)[savedLogoType] = null;
                (org as Record<string, unknown>)[`${savedLogoType}_url`] = null;
                container.innerHTML = `
                  <button type="button" class="upload-logo-btn btn btn-sm btn-secondary" data-type="${savedLogoType}">Upload ${logoType.charAt(0).toUpperCase() + logoType.slice(1)}</button>
                  <span class="text-xs text-gray-500">PNG, JPG or SVG</span>
                `;
                container.querySelector('.upload-logo-btn')?.addEventListener('click', () => {
                  currentLogoType = savedLogoType;
                  fileInput?.click();
                });
                showToast('Logo removed', 'success');
              } catch (err) {
                showToast('Failed to remove logo', 'error');
              }
            });
          }

          showToast('Logo uploaded', 'success');
          fileInput.value = '';
        } catch (err) {
          showToast('Failed to upload logo', 'error');
          console.error(err);
        }
      });
    }

    // Contacts handling (only for admin)
    if (isAdmin) {
      const refreshContactsSection = () => {
        const container = panel.querySelector('#contacts-list');
        if (!container) return;

        if (currentContacts.length === 0) {
          container.innerHTML = `<p class="text-gray-500 text-sm">No contacts added yet.</p>`;
        } else {
          container.innerHTML = currentContacts.map((contact, idx) => `
            <div class="flex items-start gap-2 p-3 bg-gray-50 rounded-lg" data-idx="${idx}">
              <div class="flex-1 space-y-2">
                <input type="text" class="contact-name input w-full" value="${escapeHtml(contact.name)}" placeholder="Contact name" />
                <input type="email" class="contact-email input w-full" value="${escapeHtml(contact.email || '')}" placeholder="Email" />
                <input type="url" class="contact-linkedin input w-full" value="${escapeHtml(contact.linkedin || '')}" placeholder="LinkedIn URL" />
              </div>
              <button type="button" class="remove-contact-btn p-2 text-red-500 hover:text-red-600" data-idx="${idx}">
                ${icon('x-lg', { class: 'w-4 h-4' })}
              </button>
            </div>
          `).join('');
        }
        attachContactsHandlers();
      };

      const attachContactsHandlers = () => {
        // Update currentContacts when inputs change
        panel.querySelectorAll('#contacts-list .contact-name').forEach((input, idx) => {
          input.addEventListener('input', () => {
            currentContacts[idx].name = (input as HTMLInputElement).value;
          });
        });
        panel.querySelectorAll('#contacts-list .contact-email').forEach((input, idx) => {
          input.addEventListener('input', () => {
            currentContacts[idx].email = (input as HTMLInputElement).value;
          });
        });
        panel.querySelectorAll('#contacts-list .contact-linkedin').forEach((input, idx) => {
          input.addEventListener('input', () => {
            currentContacts[idx].linkedin = (input as HTMLInputElement).value;
          });
        });

        // Remove contact buttons
        panel.querySelectorAll('.remove-contact-btn').forEach((btn) => {
          btn.addEventListener('click', () => {
            const idx = parseInt((btn as HTMLElement).dataset.idx || '0', 10);
            currentContacts.splice(idx, 1);
            refreshContactsSection();
          });
        });
      };

      // Add contact button
      panel.querySelector('#add-contact-btn')?.addEventListener('click', () => {
        currentContacts.push({ name: '', linkedin: '', email: '' });
        refreshContactsSection();
      });

      // Initial contacts handlers
      attachContactsHandlers();
    }
  };
}
