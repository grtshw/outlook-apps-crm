import { api, Contact, ContactRole } from '../services/api';
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
        registerAction('showAddContact', () => {
          showToast('Add contact feature coming soon', 'info');
        });

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

    container.querySelector('#empty-add-btn')?.addEventListener('click', () => {
      showToast('Add contact feature coming soon', 'info');
    });
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
    btn.addEventListener('click', () => {
      const id = btn.dataset.edit;
      showToast(`Edit contact ${id} - coming soon`, 'info');
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
