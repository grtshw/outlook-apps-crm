import { api, Contact, ContactRole, Activity } from '../services/api';
import { attachRouterLinks, registerPageCleanup } from '../router';
import {
  escapeHtml,
  showToast,
  renderTableSkeleton,
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

// Role display configuration
const ROLE_CONFIG: Record<ContactRole, { label: string; variant: 'info' | 'success' | 'warning' | 'secondary' }> = {
  presenter: { label: 'Presenter', variant: 'info' },
  speaker: { label: 'Speaker', variant: 'info' },
  sponsor: { label: 'Sponsor', variant: 'success' },
  judge: { label: 'Judge', variant: 'warning' },
  attendee: { label: 'Attendee', variant: 'secondary' },
  staff: { label: 'Staff', variant: 'secondary' },
  volunteer: { label: 'Volunteer', variant: 'secondary' },
};

interface ContactsState {
  contacts: Contact[];
  totalItems: number;
  totalPages: number;
  currentPage: number;
  searchQuery: string;
  statusFilter: string;
}

let state: ContactsState = {
  contacts: [],
  totalItems: 0,
  totalPages: 1,
  currentPage: 1,
  searchQuery: '',
  statusFilter: '',
};

export async function renderContactsPage(): Promise<void> {
  setDocumentTitle('Contacts');

  preparePageTemplate();

  const context = createPageContext();

  // Reset state
  state = {
    contacts: [],
    totalItems: 0,
    totalPages: 1,
    currentPage: 1,
    searchQuery: '',
    statusFilter: '',
  };

  await renderPageTemplate(
    {
      title: 'Contacts',
      actions: api.isAdmin()
        ? [
            {
              id: 'import-presenters',
              label: 'Import presenters',
              icon: 'download',
              variant: 'secondary',
              type: 'action',
              action: 'importPresenters',
            },
            {
              id: 'add-contact',
              label: 'Add contact',
              icon: 'plus-lg',
              variant: 'primary',
              type: 'action',
              action: 'showAddContact',
            },
          ]
        : [],
      render: async (container) => {
        // Show loading state
        container.innerHTML = `
          <div id="contacts-table">
            ${renderTableSkeleton({ rows: 10, columns: 5 })}
          </div>
        `;

        // Register action for header buttons
        registerAction('showAddContact', showCreateContactDrawer);

        registerAction('importPresenters', async () => {
          showToast('Importing presenters from Presentations...', 'info');
          try {
            const response = await fetch('/api/import/presenters', {
              method: 'POST',
              headers: {
                Authorization: api.pb.authStore.token,
              },
            });
            if (!response.ok) {
              const error = await response.json().catch(() => ({}));
              throw new Error(error.error || 'Import failed');
            }
            const result = await response.json();
            showToast(
              `Import complete: ${result.created} created, ${result.updated} updated${result.errors > 0 ? `, ${result.errors} errors` : ''}`,
              result.errors > 0 ? 'warning' : 'success'
            );
            await loadContacts();
            renderContactsTable();
          } catch (err) {
            showToast(extractErrorMessage(err, 'Failed to import presenters'), 'error');
          }
        });

        // Load data
        await loadContacts();

        // Render table
        renderContactsTable();

        // Attach handlers
        attachHandlers(container);

        // Deep link: open edit drawer if URL matches /contacts/:id
        const pathMatch = window.location.pathname.match(/^\/contacts\/([^/]+)$/);
        if (pathMatch) {
          const deepLinkId = pathMatch[1];
          try {
            const contact = await api.getContact(deepLinkId);
            setTimeout(() => showEditContactDrawer(contact), 100);
          } catch {
            showToast('Contact not found', 'error');
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

async function loadContacts(): Promise<void> {
  try {
    const result = await api.getContacts({
      page: state.currentPage,
      perPage: 50,
      search: state.searchQuery,
      status: state.statusFilter,
    });

    state.contacts = result.items;
    state.totalItems = result.totalItems;
    state.totalPages = result.totalPages;
  } catch (error) {
    showToast('Failed to load contacts', 'error');
  }
}

function getInitial(name: string): string {
  return name.charAt(0).toUpperCase();
}

function getStatusVariant(status: string): 'success' | 'warning' | 'secondary' | 'error' | 'info' {
  switch (status) {
    case 'active':
      return 'success';
    case 'inactive':
      return 'warning';
    case 'archived':
      return 'secondary';
    default:
      return 'secondary';
  }
}

function renderRoleBadges(roles?: ContactRole[]): string {
  if (!roles || roles.length === 0) return '-';
  return roles
    .map((role) => {
      const config = ROLE_CONFIG[role] || { label: role, variant: 'secondary' as const };
      return renderBadge({ label: config.label, variant: config.variant });
    })
    .join(' ');
}

// Get the best avatar URL - prefer DAM thumbnail, then small, then original URL
function getAvatarUrl(contact: Contact): string | null {
  return contact.avatar_thumb_url || contact.avatar_small_url || contact.avatar_url || null;
}

function renderContactsTable(): void {
  const container = document.getElementById('contacts-table');
  if (!container) return;

  const { contacts, totalItems, totalPages, currentPage } = state;
  const isAdmin = api.isAdmin();

  if (contacts.length === 0) {
    container.innerHTML = `
      <div class="bg-white rounded-lg border border-gray-200 p-12 text-center">
        <div class="text-gray-400 mb-4">
          ${icon('people', { class: 'w-16 h-16 mx-auto' })}
        </div>
        <h3 class="text-lg text-gray-900 mb-2">${state.searchQuery ? 'No contacts found' : 'No contacts yet'}</h3>
        <p class="text-gray-500 mb-6">${state.searchQuery ? 'Try a different search term.' : 'Add your first contact to get started.'}</p>
        ${!state.searchQuery && isAdmin ? '<button id="empty-add-btn" class="btn">Add your first contact</button>' : ''}
      </div>
    `;

    container.querySelector('#empty-add-btn')?.addEventListener('click', showCreateContactDrawer);
    return;
  }

  container.innerHTML = `
    <p class="text-gray-500 text-sm mb-4">${totalItems} contact${totalItems === 1 ? '' : 's'}</p>

    <div class="bg-white rounded-lg border border-gray-200 overflow-hidden">
      <table class="w-full">
        <thead>
          <tr class="bg-brand-black text-white text-left">
            <th class="px-4 py-3">Name</th>
            <th class="px-4 py-3">Email</th>
            <th class="px-4 py-3">Organisation</th>
            <th class="px-4 py-3">Roles</th>
            <th class="px-4 py-3">Status</th>
            ${isAdmin ? '<th class="px-4 py-3 w-32">Actions</th>' : ''}
          </tr>
        </thead>
        <tbody>
          ${contacts
            .map(
              (contact) => `
            <tr class="border-b border-gray-100 hover:bg-gray-50">
              <td class="px-4 py-3">
                <div class="flex items-center gap-3">
                  ${
                    getAvatarUrl(contact)
                      ? `<img src="${escapeHtml(getAvatarUrl(contact)!)}" alt="" class="w-8 h-8 rounded-full object-cover" />`
                      : `<div class="w-8 h-8 rounded-full bg-brand-green flex items-center justify-center text-white text-sm">${getInitial(contact.name)}</div>`
                  }
                  <div>
                    <div class="text-gray-900">${escapeHtml(contact.name)}</div>
                    ${contact.job_title ? `<div class="text-sm text-gray-500">${escapeHtml(contact.job_title)}</div>` : ''}
                  </div>
                </div>
              </td>
              <td class="px-4 py-3 text-gray-600">${escapeHtml(contact.email)}</td>
              <td class="px-4 py-3 text-gray-600">${contact.organisation_name ? escapeHtml(contact.organisation_name) : '-'}</td>
              <td class="px-4 py-3">
                ${renderRoleBadges(contact.roles)}
              </td>
              <td class="px-4 py-3">
                ${renderBadge({ label: contact.status, variant: getStatusVariant(contact.status) })}
              </td>
              ${
                isAdmin
                  ? `
                <td class="px-4 py-3">
                  <div class="flex gap-2">
                    <button class="btn btn-sm btn-secondary" data-edit="${contact.id}">Edit</button>
                    <button class="btn btn-sm btn-danger" data-delete="${contact.id}" data-name="${escapeHtml(contact.name)}">Delete</button>
                  </div>
                </td>
              `
                  : ''
              }
            </tr>
          `
            )
            .join('')}
        </tbody>
      </table>
    </div>

    ${totalPages > 1 ? renderPagination(currentPage, totalPages) : ''}
  `;

  // Edit buttons
  container.querySelectorAll<HTMLButtonElement>('[data-edit]').forEach((btn) => {
    btn.addEventListener('click', async () => {
      const id = btn.dataset.edit;
      if (!id) return;
      try {
        const contact = await api.getContact(id);
        showEditContactDrawer(contact);
      } catch {
        showToast('Failed to load contact', 'error');
      }
    });
  });

  // Delete buttons
  container.querySelectorAll<HTMLButtonElement>('[data-delete]').forEach((btn) => {
    btn.addEventListener('click', () => {
      const id = btn.dataset.delete;
      const name = btn.dataset.name || 'this contact';
      if (!id) return;

      showDeleteConfirmation({
        itemName: 'contact',
        itemTitle: name,
        onConfirm: async () => {
          try {
            await api.deleteContact(id);
            showToast('Contact deleted', 'success');
            await loadContacts();
            renderContactsTable();
          } catch (error) {
            showToast('Failed to delete contact', 'error');
          }
        },
      });
    });
  });

  // Pagination
  container.querySelectorAll<HTMLButtonElement>('[data-page]').forEach((btn) => {
    btn.addEventListener('click', async () => {
      const page = parseInt(btn.dataset.page || '1', 10);
      state.currentPage = page;
      await loadContacts();
      renderContactsTable();
    });
  });

  attachRouterLinks(container);
}

function renderPagination(page: number, totalPages: number): string {
  return `
    <div class="flex justify-center gap-2 mt-6">
      <button class="btn btn-sm btn-secondary" ${page <= 1 ? 'disabled' : ''} data-page="${page - 1}">
        Previous
      </button>
      <span class="px-4 py-2 text-sm text-gray-600">Page ${page} of ${totalPages}</span>
      <button class="btn btn-sm btn-secondary" ${page >= totalPages ? 'disabled' : ''} data-page="${page + 1}">
        Next
      </button>
    </div>
  `;
}

function attachHandlers(_container: HTMLElement): void {
  // Handlers attached inline in renderContactsTable
}

// --- Organisation autocomplete helper ---

function attachOrgAutocomplete(panel: HTMLElement): void {
  const searchInput = panel.querySelector('#org-search') as HTMLInputElement;
  const hiddenInput = panel.querySelector('#org-id') as HTMLInputElement;
  const resultsDiv = panel.querySelector('#org-results') as HTMLElement;
  if (!searchInput || !hiddenInput || !resultsDiv) return;

  let debounceTimer: ReturnType<typeof setTimeout>;

  searchInput.addEventListener('input', () => {
    clearTimeout(debounceTimer);
    const query = searchInput.value.trim();

    if (query.length < 2) {
      resultsDiv.classList.add('hidden');
      return;
    }

    debounceTimer = setTimeout(async () => {
      try {
        const result = await api.getOrganisations({ page: 1, perPage: 10, search: query });
        if (result.items.length === 0) {
          resultsDiv.innerHTML = '<div class="px-3 py-2 text-sm text-gray-500">No organisations found</div>';
        } else {
          resultsDiv.innerHTML = result.items
            .map(
              (org) => `
            <button type="button" class="org-option w-full text-left px-3 py-2 text-sm hover:bg-gray-50" data-id="${org.id}" data-name="${escapeHtml(org.name)}">
              ${escapeHtml(org.name)}
            </button>
          `
            )
            .join('');

          resultsDiv.querySelectorAll('.org-option').forEach((btn) => {
            btn.addEventListener('click', () => {
              hiddenInput.value = (btn as HTMLElement).dataset.id || '';
              searchInput.value = (btn as HTMLElement).dataset.name || '';
              resultsDiv.classList.add('hidden');
            });
          });
        }
        resultsDiv.classList.remove('hidden');
      } catch {
        resultsDiv.classList.add('hidden');
      }
    }, 200);
  });

  // Close dropdown on blur (with delay for click to register)
  searchInput.addEventListener('blur', () => {
    setTimeout(() => resultsDiv.classList.add('hidden'), 200);
  });

  // Clear hidden input if user manually changes text after selecting
  searchInput.addEventListener('input', () => {
    // If text no longer matches a selection, clear the hidden ID
    if (hiddenInput.value && searchInput.value.trim() === '') {
      hiddenInput.value = '';
    }
  });
}

// --- Avatar upload handlers ---

function attachAvatarHandlers(panel: HTMLElement, contact: Contact): void {
  const fileInput = panel.querySelector('#avatar-file-input') as HTMLInputElement;
  const changeBtn = panel.querySelector('#change-avatar-btn');
  const removeBtn = panel.querySelector('#remove-avatar-btn');

  changeBtn?.addEventListener('click', () => fileInput?.click());

  fileInput?.addEventListener('change', async () => {
    const file = fileInput.files?.[0];
    if (!file) return;

    try {
      const formData = new FormData();
      formData.append('avatar', file);

      const response = await fetch(`/api/contacts/${contact.id}/avatar`, {
        method: 'POST',
        headers: { Authorization: api.pb.authStore.token },
        body: formData,
      });

      if (!response.ok) {
        const err = await response.json().catch(() => ({}));
        throw new Error(err.error || 'Upload failed');
      }

      const updated = await response.json();
      // Update avatar display
      const container = panel.querySelector('#avatar-container');
      if (container && updated.avatar_url) {
        container.innerHTML = `<img id="avatar-img" src="${escapeHtml(updated.avatar_url)}" alt="" class="w-16 h-16 rounded-full object-cover" />`;
      }
      showToast('Avatar updated', 'success');
    } catch (err) {
      showToast(extractErrorMessage(err, 'Failed to upload avatar'), 'error');
    }
    fileInput.value = '';
  });

  removeBtn?.addEventListener('click', async () => {
    try {
      await api.updateContact(contact.id, { avatar: '' } as Partial<Contact>);
      const container = panel.querySelector('#avatar-container');
      if (container) {
        container.innerHTML = `<div id="avatar-img" class="w-16 h-16 rounded-full bg-brand-green flex items-center justify-center text-white text-xl">${getInitial(contact.name)}</div>`;
      }
      // Hide remove button
      removeBtn.classList.add('hidden');
      showToast('Avatar removed', 'success');
    } catch (err) {
      showToast(extractErrorMessage(err, 'Failed to remove avatar'), 'error');
    }
  });
}

// --- Activity timeline loader ---

function getActivityIcon(sourceApp: string): string {
  switch (sourceApp) {
    case 'presentations':
      return 'easel';
    case 'awards':
      return 'trophy';
    case 'events':
      return 'calendar-event';
    case 'dam':
      return 'image';
    case 'hubspot':
      return 'envelope';
    default:
      return 'clock';
  }
}

async function loadActivitiesTimeline(panel: HTMLElement, contactId: string): Promise<void> {
  const container = panel.querySelector('#activity-timeline');
  if (!container) return;

  try {
    const activities: Activity[] = await api.getContactActivities(contactId);

    if (activities.length === 0) {
      container.innerHTML = '<p class="text-sm text-gray-500">No activities recorded yet.</p>';
      return;
    }

    container.innerHTML = `
      <div class="space-y-3">
        ${activities
          .map(
            (activity) => `
          <div class="flex gap-3 text-sm">
            <div class="flex-shrink-0 w-8 h-8 rounded-full bg-gray-100 flex items-center justify-center">
              ${icon(getActivityIcon(activity.source_app), { class: 'w-4 h-4 text-gray-500' })}
            </div>
            <div class="flex-1 min-w-0">
              <p class="text-gray-900">${escapeHtml(activity.title || activity.type)}</p>
              <div class="flex items-center gap-2 mt-0.5">
                <span class="text-gray-500">${escapeHtml(activity.source_app)}</span>
                ${activity.occurred_at ? `<span class="text-gray-400">${new Date(activity.occurred_at).toLocaleDateString()}</span>` : ''}
                ${activity.source_url ? `<a href="${escapeHtml(activity.source_url)}" target="_blank" rel="noopener" class="text-brand-green hover:underline">View</a>` : ''}
              </div>
            </div>
          </div>
        `
          )
          .join('')}
      </div>
    `;
  } catch {
    container.innerHTML = '<p class="text-sm text-red-500">Failed to load activities.</p>';
  }
}

// --- Create contact drawer ---

const ALL_ROLES: ContactRole[] = ['presenter', 'speaker', 'sponsor', 'judge', 'attendee', 'staff', 'volunteer'];

function showCreateContactDrawer(): void {
  const drawerRef: { current: DrawerController | null } = { current: null };

  // Section: Details
  const detailsContent = `
    <div class="space-y-4">
      <div class="grid grid-cols-2 gap-4">
        <div>
          <label class="block text-sm text-gray-700 mb-1">Name *</label>
          <input type="text" name="name" class="input w-full" placeholder="e.g., Jane Smith" required />
        </div>
        <div>
          <label class="block text-sm text-gray-700 mb-1">Email *</label>
          <input type="email" name="email" class="input w-full" placeholder="jane@example.com" required />
        </div>
      </div>
      <div class="grid grid-cols-2 gap-4">
        <div>
          <label class="block text-sm text-gray-700 mb-1">Phone</label>
          <input type="tel" name="phone" class="input w-full" placeholder="+61 400 000 000" />
        </div>
        <div>
          <label class="block text-sm text-gray-700 mb-1">Pronouns</label>
          <input type="text" name="pronouns" class="input w-full" placeholder="e.g., she/her" />
        </div>
      </div>
      <div class="grid grid-cols-2 gap-4">
        <div>
          <label class="block text-sm text-gray-700 mb-1">Job title</label>
          <input type="text" name="job_title" class="input w-full" placeholder="e.g., Senior developer" />
        </div>
        <div>
          <label class="block text-sm text-gray-700 mb-1">Organisation</label>
          <div class="relative">
            <input type="text" id="org-search" class="input w-full" placeholder="Search organisations..." autocomplete="off" />
            <input type="hidden" name="organisation" id="org-id" />
            <div id="org-results" class="absolute z-10 w-full bg-white border border-gray-200 rounded-lg shadow-lg mt-1 hidden max-h-48 overflow-y-auto"></div>
          </div>
        </div>
      </div>
    </div>
  `;

  // Section: Social and web
  const socialContent = `
    <div class="space-y-4">
      <div>
        <label class="block text-sm text-gray-700 mb-1">LinkedIn</label>
        <input type="url" name="linkedin" class="input w-full" placeholder="https://linkedin.com/in/..." />
      </div>
      <div>
        <label class="block text-sm text-gray-700 mb-1">Instagram</label>
        <input type="url" name="instagram" class="input w-full" placeholder="https://instagram.com/..." />
      </div>
      <div>
        <label class="block text-sm text-gray-700 mb-1">Website</label>
        <input type="url" name="website" class="input w-full" placeholder="https://example.com" />
      </div>
    </div>
  `;

  const content = `
    <form id="create-contact-form">
      ${renderDrawerSection({
        id: 'details',
        title: 'Details',
        content: detailsContent,
        collapsible: false,
      })}
      ${renderDrawerSection({
        id: 'social',
        title: 'Social and web',
        content: socialContent,
        collapsible: true,
        collapsed: true,
      })}
      <p class="text-xs text-gray-500 mt-4">Avatar, bio, roles, and tags can be added after creation.</p>
    </form>
  `;

  const handleCreate = async () => {
    const panel = drawerRef.current?.getPanel();
    if (!panel) return;

    const form = panel.querySelector('#create-contact-form') as HTMLFormElement;
    const formData = new FormData(form);
    const name = formData.get('name') as string;
    const email = formData.get('email') as string;

    if (!name) {
      showToast('Please enter contact name', 'error');
      return;
    }
    if (!email) {
      showToast('Please enter email address', 'error');
      return;
    }

    try {
      await api.createContact({
        name,
        email,
        phone: (formData.get('phone') as string) || undefined,
        pronouns: (formData.get('pronouns') as string) || undefined,
        job_title: (formData.get('job_title') as string) || undefined,
        organisation: (formData.get('organisation') as string) || undefined,
        linkedin: (formData.get('linkedin') as string) || undefined,
        instagram: (formData.get('instagram') as string) || undefined,
        website: (formData.get('website') as string) || undefined,
        status: 'active',
      });
      await loadContacts();
      renderContactsTable();
      drawerRef.current?.close();
      showToast('Contact created', 'success');
    } catch (err) {
      showToast(extractErrorMessage(err, 'Failed to create contact'), 'error');
    }
  };

  drawerRef.current = showDrawer({
    title: 'Add contact',
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
        attachOrgAutocomplete(panel);
      }
    },
  });
}

// --- Edit contact drawer ---

function showEditContactDrawer(contact: Contact): void {
  const drawerRef: { current: DrawerController | null } = { current: null };
  const isAdmin = api.isAdmin();

  // Section 1: Details
  const avatarUrl = getAvatarUrl(contact);
  const detailsContent = `
    <div class="space-y-4">
      <div class="flex items-center gap-4">
        <div id="avatar-container">
          ${
            avatarUrl
              ? `<img id="avatar-img" src="${escapeHtml(avatarUrl)}" alt="" class="w-16 h-16 rounded-full object-cover" />`
              : `<div id="avatar-img" class="w-16 h-16 rounded-full bg-brand-green flex items-center justify-center text-white text-xl">${getInitial(contact.name)}</div>`
          }
        </div>
        ${
          isAdmin
            ? `
          <div class="flex items-center gap-2">
            <button type="button" id="change-avatar-btn" class="btn btn-sm btn-secondary">
              ${avatarUrl ? 'Change avatar' : 'Upload avatar'}
            </button>
            ${avatarUrl ? `<button type="button" id="remove-avatar-btn" class="btn btn-sm btn-danger">Remove</button>` : ''}
            <input type="file" id="avatar-file-input" accept="image/*" class="hidden" />
          </div>
        `
            : ''
        }
      </div>
      <div class="grid grid-cols-2 gap-4">
        <div>
          <label class="block text-sm text-gray-700 mb-1">Name *</label>
          <input type="text" name="name" class="input w-full" value="${escapeHtml(contact.name)}" ${!isAdmin ? 'disabled' : ''} required />
        </div>
        <div>
          <label class="block text-sm text-gray-700 mb-1">Email *</label>
          <input type="email" name="email" class="input w-full" value="${escapeHtml(contact.email)}" ${!isAdmin ? 'disabled' : ''} required />
        </div>
      </div>
      <div class="grid grid-cols-2 gap-4">
        <div>
          <label class="block text-sm text-gray-700 mb-1">Phone</label>
          <input type="tel" name="phone" class="input w-full" value="${escapeHtml(contact.phone || '')}" ${!isAdmin ? 'disabled' : ''} placeholder="+61 400 000 000" />
        </div>
        <div>
          <label class="block text-sm text-gray-700 mb-1">Pronouns</label>
          <input type="text" name="pronouns" class="input w-full" value="${escapeHtml(contact.pronouns || '')}" ${!isAdmin ? 'disabled' : ''} placeholder="e.g., she/her" />
        </div>
      </div>
      <div class="grid grid-cols-2 gap-4">
        <div>
          <label class="block text-sm text-gray-700 mb-1">Job title</label>
          <input type="text" name="job_title" class="input w-full" value="${escapeHtml(contact.job_title || '')}" ${!isAdmin ? 'disabled' : ''} placeholder="e.g., Senior developer" />
        </div>
        <div>
          <label class="block text-sm text-gray-700 mb-1">Organisation</label>
          <div class="relative">
            <input type="text" id="org-search" class="input w-full" value="${escapeHtml(contact.organisation_name || '')}" ${!isAdmin ? 'disabled' : ''} autocomplete="off" />
            <input type="hidden" name="organisation" id="org-id" value="${contact.organisation_id || ''}" />
            <div id="org-results" class="absolute z-10 w-full bg-white border border-gray-200 rounded-lg shadow-lg mt-1 hidden max-h-48 overflow-y-auto"></div>
          </div>
        </div>
      </div>
      <div>
        <label class="block text-sm text-gray-700 mb-1">Status</label>
        <select name="status" class="input w-full" ${!isAdmin ? 'disabled' : ''}>
          <option value="active" ${contact.status === 'active' ? 'selected' : ''}>Active</option>
          <option value="inactive" ${contact.status === 'inactive' ? 'selected' : ''}>Inactive</option>
          <option value="archived" ${contact.status === 'archived' ? 'selected' : ''}>Archived</option>
        </select>
      </div>
    </div>
  `;

  // Section 2: Bio and location
  const hasBio = !!(contact.bio || contact.location || contact.do_position);
  const bioContent = `
    <div class="space-y-4">
      <div>
        <label class="block text-sm text-gray-700 mb-1">Bio <span class="text-gray-400">(max 500)</span></label>
        <textarea name="bio" class="input w-full" rows="4" maxlength="500" ${!isAdmin ? 'disabled' : ''} placeholder="Brief biography">${escapeHtml(contact.bio || '')}</textarea>
        <div class="text-xs text-gray-400 text-right mt-1"><span id="bio-count">${(contact.bio || '').length}</span>/500</div>
      </div>
      <div class="grid grid-cols-2 gap-4">
        <div>
          <label class="block text-sm text-gray-700 mb-1">Location</label>
          <input type="text" name="location" class="input w-full" value="${escapeHtml(contact.location || '')}" ${!isAdmin ? 'disabled' : ''} placeholder="e.g., Melbourne, Australia" />
        </div>
        <div>
          <label class="block text-sm text-gray-700 mb-1">DO position</label>
          <input type="text" name="do_position" class="input w-full" value="${escapeHtml(contact.do_position || '')}" ${!isAdmin ? 'disabled' : ''} placeholder="e.g., Board member" />
        </div>
      </div>
    </div>
  `;

  // Section 3: Roles and tags
  const currentRoles = contact.roles || [];
  const currentTags = contact.tags || [];
  const hasRolesOrTags = currentRoles.length > 0 || currentTags.length > 0;
  const rolesTagsContent = `
    <div class="space-y-4">
      <div>
        <label class="block text-sm text-gray-700 mb-2">Roles</label>
        <div class="flex flex-wrap gap-2">
          ${ALL_ROLES.map(
            (role) => `
            <label class="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-full border cursor-pointer transition-colors ${currentRoles.includes(role) ? 'bg-gray-900 text-white border-gray-900' : 'bg-white text-gray-700 border-gray-300 hover:border-gray-400'} ${!isAdmin ? 'pointer-events-none opacity-75' : ''}">
              <input type="checkbox" name="roles" value="${role}" class="sr-only role-checkbox" ${currentRoles.includes(role) ? 'checked' : ''} ${!isAdmin ? 'disabled' : ''} />
              ${ROLE_CONFIG[role].label}
            </label>
          `
          ).join('')}
        </div>
      </div>
      <div>
        <label class="block text-sm text-gray-700 mb-1">Tags</label>
        <input type="text" name="tags" class="input w-full" value="${escapeHtml((currentTags).join(', '))}" ${!isAdmin ? 'disabled' : ''} placeholder="Comma-separated tags" />
        <div class="text-xs text-gray-400 mt-1">Separate tags with commas</div>
      </div>
    </div>
  `;

  // Section 4: Social and web
  const hasSocial = !!(contact.linkedin || contact.instagram || contact.website);
  const socialContent = `
    <div class="space-y-4">
      <div>
        <label class="block text-sm text-gray-700 mb-1">LinkedIn</label>
        <input type="url" name="linkedin" class="input w-full" value="${escapeHtml(contact.linkedin || '')}" ${!isAdmin ? 'disabled' : ''} placeholder="https://linkedin.com/in/..." />
      </div>
      <div>
        <label class="block text-sm text-gray-700 mb-1">Instagram</label>
        <input type="url" name="instagram" class="input w-full" value="${escapeHtml(contact.instagram || '')}" ${!isAdmin ? 'disabled' : ''} placeholder="https://instagram.com/..." />
      </div>
      <div>
        <label class="block text-sm text-gray-700 mb-1">Website</label>
        <input type="url" name="website" class="input w-full" value="${escapeHtml(contact.website || '')}" ${!isAdmin ? 'disabled' : ''} placeholder="https://example.com" />
      </div>
    </div>
  `;

  // Section 5: Activity timeline (loaded async)
  const activityContent = `
    <div id="activity-timeline">
      <div class="text-sm text-gray-500">Loading activities...</div>
    </div>
  `;

  // Section 6: Source info
  const sourceContent = `
    <div class="space-y-2 text-sm text-gray-500">
      ${contact.source ? `<p>Source: <span class="text-gray-900">${escapeHtml(contact.source)}</span></p>` : ''}
      <p>Created: <span class="text-gray-900">${new Date(contact.created).toLocaleDateString()}</span></p>
      <p>Updated: <span class="text-gray-900">${new Date(contact.updated).toLocaleDateString()}</span></p>
    </div>
  `;

  // Build form content
  const formContent = `
    <form id="edit-contact-form">
      ${renderDrawerSection({ id: 'details', title: 'Details', content: detailsContent, collapsible: false })}
      ${renderDrawerSection({ id: 'bio', title: 'Bio and location', content: bioContent, collapsible: true, collapsed: !hasBio })}
      ${renderDrawerSection({ id: 'roles-tags', title: 'Roles and tags', content: rolesTagsContent, collapsible: true, collapsed: !hasRolesOrTags, badge: currentRoles.length > 0 ? currentRoles.length : undefined })}
      ${renderDrawerSection({ id: 'social', title: 'Social and web', content: socialContent, collapsible: true, collapsed: !hasSocial })}
      ${renderDrawerSection({ id: 'activity', title: 'Activity', content: activityContent, collapsible: true, collapsed: true })}
      ${renderDrawerSection({ id: 'source', title: 'Created/updated', content: sourceContent, collapsible: true, collapsed: true })}
    </form>
  `;

  // Save handler
  const handleSave = async () => {
    const panel = drawerRef.current?.getPanel();
    if (!panel) return;
    const form = panel.querySelector('#edit-contact-form') as HTMLFormElement;
    if (!form) return;
    const formData = new FormData(form);
    const name = formData.get('name') as string;
    const email = formData.get('email') as string;

    if (!name) {
      showToast('Name is required', 'error');
      return;
    }
    if (!email) {
      showToast('Email is required', 'error');
      return;
    }

    // Read roles from checkboxes
    const checkedRoles: string[] = [];
    panel.querySelectorAll<HTMLInputElement>('.role-checkbox:checked').forEach((cb) => {
      checkedRoles.push(cb.value);
    });

    // Parse tags from comma-separated string
    const tagsString = formData.get('tags') as string;
    const tags = tagsString
      ? tagsString
          .split(',')
          .map((t) => t.trim())
          .filter(Boolean)
      : [];

    try {
      await api.updateContact(contact.id, {
        name,
        email,
        phone: (formData.get('phone') as string) || '',
        pronouns: (formData.get('pronouns') as string) || '',
        bio: (formData.get('bio') as string) || '',
        job_title: (formData.get('job_title') as string) || '',
        location: (formData.get('location') as string) || '',
        do_position: (formData.get('do_position') as string) || '',
        organisation: (formData.get('organisation') as string) || '',
        linkedin: (formData.get('linkedin') as string) || '',
        instagram: (formData.get('instagram') as string) || '',
        website: (formData.get('website') as string) || '',
        status: formData.get('status') as 'active' | 'inactive' | 'archived',
        roles: checkedRoles as ContactRole[],
        tags,
      });
      await loadContacts();
      renderContactsTable();
      drawerRef.current?.close();
      showToast('Contact updated', 'success');
    } catch (err) {
      showToast(extractErrorMessage(err, 'Failed to update contact'), 'error');
    }
  };

  // Delete handler
  const handleDelete = () => {
    showDeleteConfirmation({
      itemName: 'contact',
      itemTitle: contact.name,
      onConfirm: async () => {
        try {
          await api.deleteContact(contact.id);
          await loadContacts();
          renderContactsTable();
          drawerRef.current?.close();
          showToast('Contact deleted', 'success');
        } catch (err) {
          showToast(extractErrorMessage(err, 'Failed to delete contact'), 'error');
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

  // Open drawer
  drawerRef.current = showDrawer({
    title: isAdmin ? `Edit ${escapeHtml(contact.name)}` : escapeHtml(contact.name),
    content: formContent,
    width: 'lg',
    actions,
    onOpen: () => {
      const panel = drawerRef.current?.getPanel();
      if (!panel) return;

      // Update URL for deep link
      window.history.replaceState({}, '', `/contacts/${contact.id}`);

      // Attach section toggle handlers
      attachDrawerSectionHandlers(panel);

      // Attach org autocomplete (admin only)
      if (isAdmin) {
        attachOrgAutocomplete(panel);
      }

      // Bio character counter
      const bioInput = panel.querySelector('textarea[name="bio"]') as HTMLTextAreaElement;
      bioInput?.addEventListener('input', () => {
        const count = panel.querySelector('#bio-count');
        if (count) count.textContent = String(bioInput.value.length);
      });

      // Role checkbox toggle styling
      panel.querySelectorAll('.role-checkbox').forEach((cb) => {
        cb.addEventListener('change', () => {
          const label = cb.closest('label');
          if (!label) return;
          if ((cb as HTMLInputElement).checked) {
            label.classList.add('bg-gray-900', 'text-white', 'border-gray-900');
            label.classList.remove('bg-white', 'text-gray-700', 'border-gray-300');
          } else {
            label.classList.remove('bg-gray-900', 'text-white', 'border-gray-900');
            label.classList.add('bg-white', 'text-gray-700', 'border-gray-300');
          }
        });
      });

      // Avatar upload handlers (admin only)
      if (isAdmin) {
        attachAvatarHandlers(panel, contact);
      }

      // Load activities asynchronously
      loadActivitiesTimeline(panel, contact.id);
    },
    onClose: () => {
      // Reset URL back to list
      window.history.replaceState({}, '', '/contacts');
    },
  });
}
