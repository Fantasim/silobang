import { useMemo } from 'preact/hooks';
import { SearchInput } from '@components/ui/SearchInput';
import { USERS_SEARCH_PLACEHOLDER, USERS_NO_SEARCH_RESULTS_TEXT } from '@constants/auth';
import { UserStatusBadge } from './helpers';

/**
 * UserList - Searchable user table for the master panel.
 *
 * @param {Object} props
 * @param {Array} props.users - All users
 * @param {number|null} props.selectedUserId - Currently selected user ID
 * @param {(id: number) => void} props.onSelect - Called when a user row is clicked
 * @param {string} props.searchQuery - Current search query
 * @param {(query: string) => void} props.onSearchChange - Called on search input change
 */
export function UserList({ users, selectedUserId, onSelect, searchQuery, onSearchChange }) {
  const filtered = useMemo(() => {
    if (!searchQuery) return users;
    const q = searchQuery.toLowerCase();
    return users.filter((user) =>
      user.username.toLowerCase().includes(q) ||
      (user.display_name || '').toLowerCase().includes(q) ||
      (user.is_active ? 'active' : 'disabled').includes(q) ||
      (user.is_bootstrap ? 'admin' : '').includes(q)
    );
  }, [users, searchQuery]);

  return (
    <div class="users-list-panel">
      <div class="users-list-search">
        <SearchInput
          value={searchQuery}
          onChange={onSearchChange}
          onClear={() => onSearchChange('')}
          placeholder={USERS_SEARCH_PLACEHOLDER}
          size="sm"
        />
      </div>

      <div class="users-list-scroll">
        {filtered.length === 0 ? (
          <p class="users-list-empty">{USERS_NO_SEARCH_RESULTS_TEXT}</p>
        ) : (
          <table class="users-table">
            <thead>
              <tr>
                <th>Username</th>
                <th>Display Name</th>
                <th>Status</th>
              </tr>
            </thead>
            <tbody>
              {filtered.map((user) => (
                <tr
                  key={user.id}
                  class={selectedUserId === user.id ? 'users-table-row--selected' : ''}
                  onClick={() => onSelect(user.id)}
                >
                  <td style={{ color: 'var(--terminal-cyan)' }}>{user.username}</td>
                  <td>{user.display_name}</td>
                  <td>
                    <UserStatusBadge user={user} />
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>

      <div class="users-list-footer">
        <span class="users-list-count">
          {filtered.length === users.length
            ? `${users.length} user${users.length !== 1 ? 's' : ''}`
            : `${filtered.length} of ${users.length} users`
          }
        </span>
      </div>
    </div>
  );
}
