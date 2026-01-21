import PocketBase, { RecordModel } from 'pocketbase';

// Initialize PocketBase client
const pb = new PocketBase(import.meta.env.VITE_POCKETBASE_URL || 'http://127.0.0.1:8090');

// Types
export interface User extends RecordModel {
  email: string;
  name: string;
  role: 'admin' | 'viewer';
  avatar?: string;
  avatarURL?: string;
}

export type ContactRole = 'presenter' | 'speaker' | 'sponsor' | 'judge' | 'attendee' | 'staff' | 'volunteer';

export interface Contact extends RecordModel {
  email: string;
  name: string;
  phone?: string;
  pronouns?: string;
  bio?: string;
  job_title?: string;
  linkedin?: string;
  instagram?: string;
  website?: string;
  location?: string;
  do_position?: string;
  avatar?: string;
  avatar_url?: string;
  // DAM avatar variant URLs (from presenter import)
  avatar_thumb_url?: string;
  avatar_small_url?: string;
  avatar_original_url?: string;
  organisation?: string;
  organisation_id?: string;
  organisation_name?: string;
  tags?: string[];
  roles?: ContactRole[];
  status: 'active' | 'inactive' | 'archived';
  source?: string;
  source_ids?: Record<string, string>;
}

export interface Organisation extends RecordModel {
  name: string;
  website?: string;
  linkedin?: string;
  description_short?: string;
  description_medium?: string;
  description_long?: string;
  logo_square?: string;
  logo_standard?: string;
  logo_inverted?: string;
  logo_square_url?: string;
  logo_standard_url?: string;
  logo_inverted_url?: string;
  contacts?: Array<{ name: string; linkedin?: string; email?: string }>;
  tags?: string[];
  status: 'active' | 'archived';
  source?: string;
}

export interface Activity extends RecordModel {
  type: string;
  title?: string;
  contact?: string;
  organisation?: string;
  source_app: string;
  source_id?: string;
  source_url?: string;
  metadata?: Record<string, unknown>;
  occurred_at?: string;
}

export interface PaginatedResult<T> {
  items: T[];
  page: number;
  perPage: number;
  totalItems: number;
  totalPages: number;
}

export interface DashboardStats {
  contacts: {
    active: number;
    inactive: number;
    archived: number;
    total: number;
  };
  organisations: {
    active: number;
    archived: number;
    total: number;
  };
  recent_activities: number;
}

export interface App {
  key: string;
  name: string;
  url: string;
  icon: string;
}

// API class
class API {
  pb = pb;
  private apps: App[] = [];

  // Auth methods
  isLoggedIn(): boolean {
    return pb.authStore.isValid;
  }

  getCurrentUser(): User | null {
    if (!pb.authStore.isValid) return null;
    return pb.authStore.record as User;
  }

  isAdmin(): boolean {
    const user = this.getCurrentUser();
    return user?.role === 'admin';
  }

  async login(email: string, password: string): Promise<User> {
    const authData = await pb.collection('users').authWithPassword(email, password);
    return authData.record as User;
  }

  async loginWithMicrosoft(): Promise<void> {
    await pb.collection('users').authWithOAuth2({ provider: 'microsoft' });
  }

  logout(): void {
    pb.authStore.clear();
  }

  // Apps methods (for app switcher)
  async loadApps(): Promise<App[]> {
    try {
      const settings = await pb.collection('app_settings').getFullList<RecordModel>();
      const appsString = settings.find(s => s.key === 'apps')?.value;
      if (appsString) {
        this.apps = JSON.parse(appsString);
      }
    } catch {
      // Ignore errors loading apps
    }
    return this.apps;
  }

  getApps(): App[] {
    return this.apps;
  }

  // Dashboard
  async getDashboardStats(): Promise<DashboardStats> {
    const response = await fetch('/api/dashboard/stats', {
      headers: { Authorization: pb.authStore.token },
    });
    if (!response.ok) throw new Error('Failed to fetch dashboard stats');
    return response.json();
  }

  // Contacts
  async getContacts(params: {
    page?: number;
    perPage?: number;
    search?: string;
    status?: string;
    sort?: string;
  } = {}): Promise<PaginatedResult<Contact>> {
    const searchParams = new URLSearchParams();
    if (params.page) searchParams.set('page', params.page.toString());
    if (params.perPage) searchParams.set('perPage', params.perPage.toString());
    if (params.search) searchParams.set('search', params.search);
    if (params.status) searchParams.set('status', params.status);
    if (params.sort) searchParams.set('sort', params.sort);

    const response = await fetch(`/api/contacts?${searchParams}`, {
      headers: { Authorization: pb.authStore.token },
    });
    if (!response.ok) throw new Error('Failed to fetch contacts');
    return response.json();
  }

  async getContact(id: string): Promise<Contact> {
    const response = await fetch(`/api/contacts/${id}`, {
      headers: { Authorization: pb.authStore.token },
    });
    if (!response.ok) throw new Error('Failed to fetch contact');
    return response.json();
  }

  async createContact(data: Partial<Contact>): Promise<Contact> {
    const response = await fetch('/api/contacts', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        Authorization: pb.authStore.token,
      },
      body: JSON.stringify(data),
    });
    if (!response.ok) {
      const error = await response.json();
      throw new Error(error.error || 'Failed to create contact');
    }
    return response.json();
  }

  async updateContact(id: string, data: Partial<Contact>): Promise<Contact> {
    const response = await fetch(`/api/contacts/${id}`, {
      method: 'PATCH',
      headers: {
        'Content-Type': 'application/json',
        Authorization: pb.authStore.token,
      },
      body: JSON.stringify(data),
    });
    if (!response.ok) {
      const error = await response.json();
      throw new Error(error.error || 'Failed to update contact');
    }
    return response.json();
  }

  async deleteContact(id: string): Promise<void> {
    const response = await fetch(`/api/contacts/${id}`, {
      method: 'DELETE',
      headers: { Authorization: pb.authStore.token },
    });
    if (!response.ok) throw new Error('Failed to delete contact');
  }

  async getContactActivities(id: string): Promise<Activity[]> {
    const response = await fetch(`/api/contacts/${id}/activities`, {
      headers: { Authorization: pb.authStore.token },
    });
    if (!response.ok) throw new Error('Failed to fetch contact activities');
    return response.json();
  }

  // Organisations
  async getOrganisations(params: {
    page?: number;
    perPage?: number;
    search?: string;
    status?: string;
    sort?: string;
  } = {}): Promise<PaginatedResult<Organisation>> {
    const searchParams = new URLSearchParams();
    if (params.page) searchParams.set('page', params.page.toString());
    if (params.perPage) searchParams.set('perPage', params.perPage.toString());
    if (params.search) searchParams.set('search', params.search);
    if (params.status) searchParams.set('status', params.status);
    if (params.sort) searchParams.set('sort', params.sort);

    const response = await fetch(`/api/organisations?${searchParams}`, {
      headers: { Authorization: pb.authStore.token },
    });
    if (!response.ok) throw new Error('Failed to fetch organisations');
    return response.json();
  }

  async getOrganisation(id: string): Promise<Organisation> {
    const response = await fetch(`/api/organisations/${id}`, {
      headers: { Authorization: pb.authStore.token },
    });
    if (!response.ok) throw new Error('Failed to fetch organisation');
    return response.json();
  }

  async createOrganisation(data: Partial<Organisation>): Promise<Organisation> {
    const response = await fetch('/api/organisations', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        Authorization: pb.authStore.token,
      },
      body: JSON.stringify(data),
    });
    if (!response.ok) {
      const error = await response.json();
      throw new Error(error.error || 'Failed to create organisation');
    }
    return response.json();
  }

  async updateOrganisation(id: string, data: Partial<Organisation>): Promise<Organisation> {
    const response = await fetch(`/api/organisations/${id}`, {
      method: 'PATCH',
      headers: {
        'Content-Type': 'application/json',
        Authorization: pb.authStore.token,
      },
      body: JSON.stringify(data),
    });
    if (!response.ok) {
      const error = await response.json();
      throw new Error(error.error || 'Failed to update organisation');
    }
    return response.json();
  }

  async deleteOrganisation(id: string): Promise<void> {
    const response = await fetch(`/api/organisations/${id}`, {
      method: 'DELETE',
      headers: { Authorization: pb.authStore.token },
    });
    if (!response.ok) throw new Error('Failed to delete organisation');
  }

  // Activities
  async getActivities(params: {
    page?: number;
    perPage?: number;
    source_app?: string;
    type?: string;
  } = {}): Promise<PaginatedResult<Activity>> {
    const searchParams = new URLSearchParams();
    if (params.page) searchParams.set('page', params.page.toString());
    if (params.perPage) searchParams.set('perPage', params.perPage.toString());
    if (params.source_app) searchParams.set('source_app', params.source_app);
    if (params.type) searchParams.set('type', params.type);

    const response = await fetch(`/api/activities?${searchParams}`, {
      headers: { Authorization: pb.authStore.token },
    });
    if (!response.ok) throw new Error('Failed to fetch activities');
    return response.json();
  }

  // File URLs
  getFileUrl(collectionId: string, recordId: string, filename: string): string {
    return `${pb.baseURL}/api/files/${collectionId}/${recordId}/${filename}`;
  }

  // Projection
  async projectAll(): Promise<{ total: number; contacts: number; organisations: number }> {
    const response = await fetch('/api/project-all', {
      method: 'POST',
      headers: { Authorization: pb.authStore.token },
    });
    if (!response.ok) throw new Error('Failed to project all');
    return response.json();
  }
}

export const api = new API();
