import { currentPath, navigate } from '../../Router';
import { currentUser, logout, canManageUsers, canViewAudit, canManageConfig } from '@store/auth';
import { Icon } from './Icon';

export function NavBar() {
  const path = currentPath.value;
  const user = currentUser.value;

  const handleLogout = async () => {
    await logout();
  };

  return (
    <nav class="navbar">
      <a
        class="navbar-brand"
        href="/"
        onClick={(e) => { e.preventDefault(); navigate('/'); }}
      >
        MESHBANK
      </a>

      <div class="navbar-links">
        <a
          class={`navbar-link ${path === '/' ? 'active' : ''}`}
          href="/"
          onClick={(e) => { e.preventDefault(); navigate('/'); }}
        >
          Topics
        </a>
        <a
          class={`navbar-link ${path === '/query' ? 'active' : ''}`}
          href="/query"
          onClick={(e) => { e.preventDefault(); navigate('/query'); }}
        >
          Query
        </a>
        <a
          class={`navbar-link ${path === '/api' ? 'active' : ''}`}
          href="/api"
          onClick={(e) => { e.preventDefault(); navigate('/api'); }}
        >
          API
        </a>
        {canViewAudit.value && (
          <a
            class={`navbar-link ${path === '/audit' ? 'active' : ''}`}
            href="/audit"
            onClick={(e) => { e.preventDefault(); navigate('/audit'); }}
          >
            Audit
          </a>
        )}
        {canManageConfig.value && (
          <a
            class={`navbar-link ${path === '/monitoring' ? 'active' : ''}`}
            href="/monitoring"
            onClick={(e) => { e.preventDefault(); navigate('/monitoring'); }}
          >
            Monitoring
          </a>
        )}
        {canManageUsers.value && (
          <a
            class={`navbar-link ${path.startsWith('/users') ? 'active' : ''}`}
            href="/users"
            onClick={(e) => { e.preventDefault(); navigate('/users'); }}
          >
            Users
          </a>
        )}
      </div>

      <div class="navbar-user">
        <span class="navbar-username">
          <Icon name="user" size={14} />
          {user?.display_name || user?.username}
        </span>
        <button class="navbar-logout" onClick={handleLogout} title="Log out">
          Log out
        </button>
      </div>
    </nav>
  );
}
