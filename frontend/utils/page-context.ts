/**
 * Page context helper for CRM app.
 *
 * Creates the PageTemplateContext needed by renderPageTemplate() from ui-kit.
 */

import { type PageTemplateContext, type AppSettings } from '@theoutlook/ui-kit';
import { api } from '../services/api';
import { router } from '../router';

/**
 * App settings for CRM.
 */
const CRM_SETTINGS: AppSettings = {
  id: 'crm',
  app_id: 'crm',
  app_name: 'CRM',
  app_title: 'Contact Relationship Manager',
  app_url: 'https://crm.theoutlook.io',
  app_icon: 'person-vcard',
  required_role: 'crm_access',
  sort_order: 3,
  is_active: true,
  menu_items: [],
  domain_actions: [],
  search_config: {
    placeholder: 'Search contacts, organisations...',
    collections: [],
  },
  routing: {
    default_route: '/contacts',
    login_redirect: '/contacts',
    logout_redirect: '/login',
    unauthenticated_redirect: '/login',
  },
  pagination: {
    default_per_page: 24,
    max_visible_pages: 7,
    options: [12, 24, 48],
  },
  cache_ttl: {
    list: 60 * 1000,
    detail: 2 * 60 * 1000,
    summary: 30 * 1000,
    static: 24 * 60 * 60 * 1000,
  },
  external_urls: {},
  features: {},
};

/**
 * Create a PageTemplateContext for the current page.
 *
 * @param params - Route parameters (e.g., { id: '123' })
 * @returns PageTemplateContext for use with renderPageTemplate()
 */
export function createPageContext(params?: Record<string, string>): PageTemplateContext {
  const user = api.getCurrentUser();

  // Parse query string
  const searchParams = new URLSearchParams(window.location.search);
  const query: Record<string, string> = {};
  searchParams.forEach((value, key) => {
    query[key] = value;
  });

  const isAdmin = user?.role === 'admin';

  return {
    user: user
      ? {
          id: user.id,
          name: user.name,
          email: user.email,
          is_admin: isAdmin,
          role: user.role || 'viewer',
        }
      : null,
    settings: CRM_SETTINGS,
    params: params || {},
    query,
    navigate: (path: string) => router.navigate(path),
  };
}
