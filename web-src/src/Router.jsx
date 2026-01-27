import { signal, computed } from '@preact/signals';
import { useEffect } from 'preact/hooks';

import { SetupPage } from '@pages/SetupPage';
import { LoginPage } from '@pages/LoginPage';
import { DashboardPage } from '@pages/DashboardPage';
import { TopicPage } from '@pages/TopicPage';
import { QueryPage } from '@pages/QueryPage';
import { ApiPage } from '@pages/ApiPage';
import { AuditLogPage } from '@pages/AuditLogPage';
import { MonitoringPage } from '@pages/MonitoringPage';
import { UsersPage } from '@pages/UsersPage';

import { authStatus, bootstrapCredentials } from '@store/auth';
import { AUTH_STATUS } from '@constants/auth';
import { Spinner } from '@components/ui';

// Current route signal
export const currentPath = signal(window.location.pathname);

// Parse route params
export const routeParams = computed(() => {
  const path = currentPath.value;

  // /topic/:name
  const topicMatch = path.match(/^\/topic\/([^/]+)$/);
  if (topicMatch) {
    return { page: 'topic', name: decodeURIComponent(topicMatch[1]) };
  }

  // /users/:id
  const userMatch = path.match(/^\/users\/(\d+)$/);
  if (userMatch) {
    return { page: 'users', userId: parseInt(userMatch[1], 10) };
  }

  if (path === '/query') return { page: 'query' };
  if (path === '/api') return { page: 'api' };
  if (path === '/audit') return { page: 'audit' };
  if (path === '/monitoring') return { page: 'monitoring' };
  if (path === '/users') return { page: 'users' };

  return { page: 'home' };
});

// Navigation function
export function navigate(path) {
  window.history.pushState({}, '', path);
  currentPath.value = path;
}

export function Router() {
  // Handle browser back/forward
  useEffect(() => {
    const handlePopState = () => {
      currentPath.value = window.location.pathname;
    };
    window.addEventListener('popstate', handlePopState);
    return () => window.removeEventListener('popstate', handlePopState);
  }, []);

  const status = authStatus.value;

  // Loading — checking auth status
  if (status === AUTH_STATUS.LOADING) {
    return (
      <div class="setup-page">
        <Spinner />
      </div>
    );
  }

  // Not configured or bootstrap credentials being shown
  if (status === AUTH_STATUS.UNCONFIGURED || bootstrapCredentials.value) {
    return <SetupPage />;
  }

  // Configured but not authenticated
  if (status === AUTH_STATUS.UNAUTHENTICATED) {
    return <LoginPage />;
  }

  // Authenticated — route to pages
  const { page, name, userId } = routeParams.value;

  switch (page) {
    case 'topic':
      return <TopicPage topicName={name} />;
    case 'query':
      return <QueryPage />;
    case 'api':
      return <ApiPage />;
    case 'audit':
      return <AuditLogPage />;
    case 'monitoring':
      return <MonitoringPage />;
    case 'users':
      return <UsersPage userId={userId} />;
    default:
      return <DashboardPage />;
  }
}
