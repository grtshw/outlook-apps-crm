import { api, DashboardStats } from '../services/api';
import { attachRouterLinks, registerPageCleanup } from '../router';
import {
  showToast,
  renderCardSkeleton,
  setDocumentTitle,
  icon,
  renderPageTemplate,
} from '@theoutlook/ui-kit';
import { preparePageTemplate } from '../components/template';
import { createPageContext } from '../utils/page-context';

export async function renderDashboardPage(): Promise<void> {
  setDocumentTitle('Dashboard');

  preparePageTemplate();

  const context = createPageContext();
  const user = api.getCurrentUser();

  await renderPageTemplate(
    {
      title: 'Dashboard',
      subtitle: user ? `Welcome back, ${user.name || user.email}` : undefined,
      render: async (container) => {
        // Show loading state
        container.innerHTML = renderCardSkeleton({ count: 3, columns: 3 });

        try {
          const stats = await api.getDashboardStats();
          renderDashboardContent(container, stats);
        } catch (error) {
          showToast('Failed to load dashboard', 'error');
          container.innerHTML = `
            <p class="text-gray-500">Failed to load dashboard data.</p>
          `;
        }
      },
    },
    context
  );

  registerPageCleanup(() => {
    // Cleanup if needed
  });
}

function renderDashboardContent(container: HTMLElement, stats: DashboardStats): void {
  container.innerHTML = `
    <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6 mb-8">
      <!-- Contacts Card -->
      <a href="/contacts" data-router-link class="block bg-white rounded-lg border border-gray-200 p-6 hover:shadow-md transition-shadow">
        <div class="flex items-center gap-4 mb-4">
          <div class="w-12 h-12 rounded-full bg-brand-green/10 flex items-center justify-center">
            ${icon('people', { class: 'w-6 h-6 text-brand-green' })}
          </div>
          <div>
            <h3 class="text-lg text-gray-900">Contacts</h3>
            <p class="text-3xl text-gray-900">${stats.contacts.total}</p>
          </div>
        </div>
        <div class="flex gap-4 text-sm text-gray-500">
          <span>${stats.contacts.active} active</span>
          <span>${stats.contacts.inactive} inactive</span>
          <span>${stats.contacts.archived} archived</span>
        </div>
      </a>

      <!-- Organisations Card -->
      <a href="/organisations" data-router-link class="block bg-white rounded-lg border border-gray-200 p-6 hover:shadow-md transition-shadow">
        <div class="flex items-center gap-4 mb-4">
          <div class="w-12 h-12 rounded-full bg-brand-purple/10 flex items-center justify-center">
            ${icon('building', { class: 'w-6 h-6 text-brand-purple' })}
          </div>
          <div>
            <h3 class="text-lg text-gray-900">Organisations</h3>
            <p class="text-3xl text-gray-900">${stats.organisations.total}</p>
          </div>
        </div>
        <div class="flex gap-4 text-sm text-gray-500">
          <span>${stats.organisations.active} active</span>
          <span>${stats.organisations.archived} archived</span>
        </div>
      </a>

      <!-- Activities Card -->
      <div class="bg-white rounded-lg border border-gray-200 p-6">
        <div class="flex items-center gap-4 mb-4">
          <div class="w-12 h-12 rounded-full bg-amber-100 flex items-center justify-center">
            ${icon('activity', { class: 'w-6 h-6 text-amber-600' })}
          </div>
          <div>
            <h3 class="text-lg text-gray-900">Recent activity</h3>
            <p class="text-3xl text-gray-900">${stats.recent_activities}</p>
          </div>
        </div>
        <p class="text-sm text-gray-500">Activities in the last 30 days</p>
      </div>
    </div>

    <!-- Quick Actions -->
    <div class="bg-white rounded-lg border border-gray-200 p-6">
      <h2 class="text-lg text-gray-900 mb-4">Quick actions</h2>
      <div class="flex flex-wrap gap-3">
        <a href="/contacts" data-router-link class="btn btn-secondary">
          ${icon('plus-lg', { class: 'w-4 h-4' })} Add contact
        </a>
        <a href="/organisations" data-router-link class="btn btn-secondary">
          ${icon('plus-lg', { class: 'w-4 h-4' })} Add organisation
        </a>
      </div>
    </div>
  `;

  attachRouterLinks(container);
}
