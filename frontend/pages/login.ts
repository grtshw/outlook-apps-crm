import { api } from '../services/api';
import { router } from '../router';
import { initLoginPage, renderAccessDenied, type LoginPageOptions } from '@theoutlook/ui-kit';
import { hideAppShell } from '../components/template';

export async function renderLogin(): Promise<void> {
  hideAppShell();

  const loginSection = document.getElementById('login-section');
  if (!loginSection) return;

  const options: LoginPageOptions = {
    appName: 'CRM',
    logoUrl: '/images/logo-white.svg',
    logoAlt: 'CRM',
    onSSOLogin: async () => {
      await api.loginWithMicrosoft();
    },
    onPasswordLogin: async (email: string, password: string) => {
      await api.login(email, password);
    },
    onLoginSuccess: () => {
      router.navigate('/');
    },
    checkAppAccess: () => {
      const user = api.getCurrentUser();
      if (!user) return false;
      // If no role, deny access
      const role = user.role;
      return role === 'admin' || role === 'viewer';
    },
    onLogout: () => {
      api.logout();
      router.navigate('/login');
    },
  };

  initLoginPage(loginSection, options);
}

export { renderAccessDenied };
