/**
 * DAM API Service
 *
 * Handles communication with the DAM app for file operations (logos, avatars).
 * Uses HMAC-signed requests for cross-app authentication.
 */

const DAM_URL = import.meta.env.VITE_DAM_URL || 'http://localhost:8091';

export interface AvatarUrls {
  thumb: string | null;
  small: string | null;
  original: string | null;
}

export interface LogoUrls {
  square: string | null;
  standard: string | null;
  inverted: string | null;
}

export interface UploadToken {
  org_id: string;
  logo_type: string;
  timestamp: string;
  action: string;
  signature: string;
  dam_url: string;
  expires_in: number;
}

// Simple LRU-style cache for avatar/logo URLs
const avatarCache = new Map<string, { urls: AvatarUrls; timestamp: number }>();
const logoCache = new Map<string, { urls: LogoUrls; timestamp: number }>();
const CACHE_TTL = 5 * 60 * 1000; // 5 minutes

/**
 * DAM API client for cross-app file operations
 */
export const damApi = {
  /**
   * Get avatar URLs for a contact by CRM contact ID
   */
  async getContactAvatarUrls(contactId: string): Promise<AvatarUrls | null> {
    // Check cache first
    const cached = avatarCache.get(contactId);
    if (cached && Date.now() - cached.timestamp < CACHE_TTL) {
      return cached.urls;
    }

    try {
      const response = await fetch(`${DAM_URL}/api/presenter-lookup/${contactId}`);
      if (!response.ok) {
        if (response.status === 404) return null;
        throw new Error('Failed to fetch avatar URLs');
      }

      const data = await response.json();
      const urls: AvatarUrls = {
        thumb: data.avatar_thumb_url || null,
        small: data.avatar_small_url || null,
        original: data.avatar_original_url || null,
      };

      // Cache the result
      avatarCache.set(contactId, { urls, timestamp: Date.now() });
      return urls;
    } catch (error) {
      console.error('[DAM API] Error fetching avatar URLs:', error);
      return null;
    }
  },

  /**
   * Get logo URLs for an organisation by CRM org ID
   */
  async getOrganisationLogoUrls(orgId: string): Promise<LogoUrls | null> {
    // Check cache first
    const cached = logoCache.get(orgId);
    if (cached && Date.now() - cached.timestamp < CACHE_TTL) {
      return cached.urls;
    }

    try {
      const response = await fetch(`${DAM_URL}/api/org-lookup/${orgId}`);
      if (!response.ok) {
        if (response.status === 404) return null;
        throw new Error('Failed to fetch logo URLs');
      }

      const data = await response.json();
      const urls: LogoUrls = {
        square: data.logo_square_url || null,
        standard: data.logo_standard_url || null,
        inverted: data.logo_inverted_url || null,
      };

      // Cache the result
      logoCache.set(orgId, { urls, timestamp: Date.now() });
      return urls;
    } catch (error) {
      console.error('[DAM API] Error fetching logo URLs:', error);
      return null;
    }
  },

  /**
   * Upload a logo to DAM using HMAC-signed request
   * @param orgId Organisation ID (CRM)
   * @param type Logo type: 'square' | 'standard' | 'inverted'
   * @param file File to upload
   * @param getToken Function to get HMAC token from CRM backend
   */
  async uploadOrganisationLogo(
    orgId: string,
    type: string,
    file: File,
    getToken: (orgId: string, type: string, action: string) => Promise<UploadToken>
  ): Promise<LogoUrls> {
    // Get HMAC token from CRM backend
    const token = await getToken(orgId, type, 'upload');

    // Upload to DAM with signature
    const formData = new FormData();
    formData.append('logo', file);

    const response = await fetch(`${token.dam_url}/api/org-logo/${orgId}/${type}`, {
      method: 'POST',
      headers: {
        'X-Upload-Signature': token.signature,
        'X-Upload-Timestamp': token.timestamp,
      },
      body: formData,
    });

    if (!response.ok) {
      const error = await response.json().catch(() => ({}));
      throw new Error(error.error || 'Failed to upload logo');
    }

    const data = await response.json();

    // Invalidate cache
    logoCache.delete(orgId);

    return {
      square: data.logo_square_url || null,
      standard: data.logo_standard_url || null,
      inverted: data.logo_inverted_url || null,
    };
  },

  /**
   * Delete a logo from DAM using HMAC-signed request
   * @param orgId Organisation ID (CRM)
   * @param type Logo type: 'square' | 'standard' | 'inverted'
   * @param getToken Function to get HMAC token from CRM backend
   */
  async deleteOrganisationLogo(
    orgId: string,
    type: string,
    getToken: (orgId: string, type: string, action: string) => Promise<UploadToken>
  ): Promise<void> {
    // Get HMAC token from CRM backend
    const token = await getToken(orgId, type, 'delete');

    // Delete from DAM with signature
    const response = await fetch(`${token.dam_url}/api/org-logo/${orgId}/${type}`, {
      method: 'DELETE',
      headers: {
        'X-Upload-Signature': token.signature,
        'X-Upload-Timestamp': token.timestamp,
      },
    });

    if (!response.ok) {
      const error = await response.json().catch(() => ({}));
      throw new Error(error.error || 'Failed to delete logo');
    }

    // Invalidate cache
    logoCache.delete(orgId);
  },

  /**
   * Clear all caches
   */
  clearCache(): void {
    avatarCache.clear();
    logoCache.clear();
  },

  /**
   * Invalidate cache for a specific organisation
   */
  invalidateOrgCache(orgId: string): void {
    logoCache.delete(orgId);
  },

  /**
   * Invalidate cache for a specific contact
   */
  invalidateContactCache(contactId: string): void {
    avatarCache.delete(contactId);
  },
};
