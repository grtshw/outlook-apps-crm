import { api } from './services/api';

// Page cleanup callback
type CleanupCallback = () => void;
let currentCleanup: CleanupCallback | null = null;

// Register a cleanup function for the current page
export function registerPageCleanup(cleanup: CleanupCallback): void {
  currentCleanup = cleanup;
}

// Route definition
interface Route {
  path: string;
  handler: () => Promise<void>;
  requiresAuth: boolean;
  requiresAdmin: boolean;
}

// Route registry
const routes: Route[] = [];

// Register a route
export function registerRoute(
  path: string,
  handler: () => Promise<void>,
  options: { requiresAuth?: boolean; requiresAdmin?: boolean } = {}
): void {
  routes.push({
    path,
    handler,
    requiresAuth: options.requiresAuth ?? true,
    requiresAdmin: options.requiresAdmin ?? false,
  });
}

// Match a path to a route
function matchRoute(pathname: string): Route | null {
  // Exact match first
  const exactMatch = routes.find(r => r.path === pathname);
  if (exactMatch) return exactMatch;

  // Pattern matching (e.g., /contacts/:id)
  for (const route of routes) {
    const pattern = route.path.replace(/:[^/]+/g, '([^/]+)');
    const regex = new RegExp(`^${pattern}$`);
    if (regex.test(pathname)) {
      return route;
    }
  }

  return null;
}

// Router class
class Router {
  // Navigate to a path
  navigate(path: string, replace = false): void {
    if (replace) {
      window.history.replaceState(null, '', path);
    } else {
      window.history.pushState(null, '', path);
    }
    this.handleRoute(path);
  }

  // Handle a route
  async handleRoute(pathname: string): Promise<void> {
    // Run cleanup from previous page
    if (currentCleanup) {
      currentCleanup();
      currentCleanup = null;
    }

    const route = matchRoute(pathname);

    if (!route) {
      // 404 - redirect to home
      this.navigate('/', true);
      return;
    }

    // Check auth requirements
    if (route.requiresAuth && !api.isLoggedIn()) {
      this.navigate('/login', true);
      return;
    }

    if (route.requiresAdmin && !api.isAdmin()) {
      this.navigate('/', true);
      return;
    }

    // Execute the route handler
    try {
      await route.handler();
    } catch (error) {
      console.error('Route handler error:', error);
    }
  }
}

export const router = new Router();

// Utility to attach router links to elements
export function attachRouterLinks(container: HTMLElement): void {
  const links = container.querySelectorAll<HTMLAnchorElement>('a[data-router-link]');
  links.forEach(link => {
    link.addEventListener('click', (e) => {
      e.preventDefault();
      const href = link.getAttribute('href');
      if (href) {
        router.navigate(href);
      }
    });
  });
}
