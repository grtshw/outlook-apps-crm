import {
  initAppShell,
  type AppShellHandlers,
  type SearchResult,
  showToast,
} from '@theoutlook/ui-kit';
import { api } from '../services/api';
import { router } from '../router';

// Shell state
let shellHandlers: AppShellHandlers | null = null;

/**
 * Search handler for the top bar
 */
async function handleSearch(query: string, _signal: AbortSignal): Promise<SearchResult[]> {
  try {
    const results: SearchResult[] = [];

    // Search contacts
    const contactsResult = await api.getContacts({ page: 1, perPage: 50, search: query });
    for (const contact of contactsResult.items.slice(0, 5)) {
      results.push({
        id: contact.id,
        label: contact.name,
        subtitle: contact.organisation_name || contact.email,
        icon: 'person',
        href: `/contacts/${contact.id}`,
        category: 'Contacts',
      });
    }

    // Search organisations
    const orgsResult = await api.getOrganisations({ page: 1, perPage: 50, search: query });
    for (const org of orgsResult.items.slice(0, 5)) {
      results.push({
        id: org.id,
        label: org.name,
        subtitle: org.website || undefined,
        icon: 'building',
        href: `/organisations/${org.id}`,
        category: 'Organisations',
      });
    }

    return results;
  } catch (error) {
    console.error('Search error:', error);
    return [];
  }
}

/**
 * Shows the app shell (sidebar + main content) and hides login section.
 */
function showAppShell(): void {
  const loginSection = document.getElementById('login-section');
  const appShell = document.getElementById('app-shell');

  if (loginSection) loginSection.innerHTML = '';
  if (appShell) appShell.classList.add('authenticated');
}

/**
 * Hides the app shell and shows login section.
 */
export function hideAppShell(): void {
  const appShell = document.getElementById('app-shell');
  if (appShell) appShell.classList.remove('authenticated');
}

/**
 * Initialize the app shell (sidebar + top bar) using ui-kit.
 */
async function initShell(): Promise<void> {
  // Only initialize once
  if (shellHandlers) return;

  shellHandlers = await initAppShell({
    pb: api.pb as any,
    appId: 'crm',
    onNavigate: (path) => router.navigate(path),
    onLogout: async () => {
      await api.logout();
      router.navigate('/login');
    },
    onSearch: handleSearch,
    additionalDomainActions: [
      {
        id: 'project-all',
        icon: 'database-up',
        tooltip: 'Project all',
        onClick: async () => {
          try {
            showToast('Projecting...', 'info');
            await api.projectAll();
            showToast('Projection complete', 'success');
          } catch (error) {
            console.error('Project all error:', error);
            showToast('Failed to project', 'error');
          }
        },
      },
    ],
  });

  // Set up mobile menu toggle
  const mobileMenuBtn = document.getElementById('mobile-menu-btn');
  if (mobileMenuBtn) {
    mobileMenuBtn.addEventListener('click', () => {
      window.dispatchEvent(new CustomEvent('sidebar-open'));
    });
  }
}

/**
 * Clear the breadcrumbs container.
 */
export function clearBreadcrumbs(): void {
  const container = document.getElementById('page-breadcrumbs');
  if (container) {
    container.innerHTML = '';
  }
}

/**
 * Clear the page menu container.
 */
export function clearPageMenu(): void {
  const container = document.getElementById('page-menu');
  if (container) {
    container.innerHTML = '';
  }
}

/**
 * Prepare the page template for rendering with renderPageTemplate from ui-kit.
 * Sets up the app shell (sidebar, topbar) without rendering content.
 * Content is rendered by renderPageTemplate to the appropriate containers.
 */
export async function preparePageTemplate(): Promise<void> {
  showAppShell();
  clearPageMenu();
  clearBreadcrumbs();
  await initShell();
}

/**
 * Get the shell handlers for external use
 */
export function getShellHandlers(): AppShellHandlers | null {
  return shellHandlers;
}

/**
 * Subscribe to user record changes for avatar updates.
 */
export function subscribeToUserUpdates(): void {
  const user = api.getCurrentUser();
  if (!user) return;

  api.pb.collection('users').subscribe(user.id, async (e) => {
    if (e.action === 'update') {
      await api.pb.collection('users').authRefresh();
      if (shellHandlers) {
        shellHandlers.rerenderSidebar();
      }
    }
  });
}

/**
 * Renders content to the login section (for unauthenticated state).
 */
export function renderLoginSection(content: string): void {
  hideAppShell();
  const loginSection = document.getElementById('login-section');
  if (loginSection) {
    loginSection.innerHTML = content;
  }
}
