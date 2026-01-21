// Redirect from fly.dev domain to theoutlook.io domain
if (typeof window !== 'undefined' && window.location.hostname === 'outlook-apps-crm.fly.dev') {
  window.location.replace(`https://crm.theoutlook.io${window.location.pathname}${window.location.search}`);
}

import { api } from './services/api';
import { router, registerRoute } from './router';
import { initToast } from '@theoutlook/ui-kit';
import { preparePageTemplate, subscribeToUserUpdates, getShellHandlers } from './components/template';

// Import pages
import { renderLogin } from './pages/login';
import { renderDashboardPage } from './pages/dashboard';
import { renderContactsPage } from './pages/contacts';
import { renderOrganisationsPage } from './pages/organisations';

// Register routes
registerRoute('/login', renderLogin, { requiresAuth: false });
registerRoute('/', renderDashboardPage, { requiresAuth: true });
registerRoute('/contacts', renderContactsPage, { requiresAuth: true });
registerRoute('/organisations', renderOrganisationsPage, { requiresAuth: true });
registerRoute('/organisations/:id', renderOrganisationsPage, { requiresAuth: true });

// Re-export shell handlers for external use
export { getShellHandlers };

async function init() {
  const appEl = document.getElementById('app');
  if (!appEl) return;

  // Initialize toast system
  initToast();

  // Load apps for app switcher
  await api.loadApps();

  // Check auth status
  const isLoggedIn = api.isLoggedIn();

  if (!isLoggedIn) {
    router.navigate('/login');
  } else {
    // Initialize app shell
    await preparePageTemplate();

    // Subscribe to user updates for avatar sync
    subscribeToUserUpdates();

    // Navigate to current path
    router.navigate(window.location.pathname || '/');
  }

  // Listen for navigation
  window.addEventListener('popstate', () => {
    router.handleRoute(window.location.pathname);
  });
}

init();
