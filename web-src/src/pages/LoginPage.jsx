import { useState } from 'preact/hooks';
import { Button } from '@components/ui/Button';
import { ErrorBanner } from '@components/ui/ErrorBanner';
import { Spinner } from '@components/ui/Spinner';
import { login } from '@store/auth';
import { AUTH_ERROR_MESSAGES } from '@constants/auth';

export function LoginPage() {
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState(null);

  const handleSubmit = async (e) => {
    e.preventDefault();
    if (!username.trim() || !password) return;

    setLoading(true);
    setError(null);

    const result = await login(username.trim(), password);

    if (!result.success) {
      const errCode = result.error?.code;
      const message = AUTH_ERROR_MESSAGES[errCode] || result.error?.message || 'Login failed.';
      setError(message);
    }

    setLoading(false);
  };

  return (
    <div class="login-page">
      <div class="login-card">
        <h1 class="login-title">SILOBANG</h1>
        <p class="login-subtitle">Authenticate to continue</p>

        <form onSubmit={handleSubmit}>
          <label class="login-label">Username</label>
          <input
            type="text"
            class="login-input"
            placeholder="username"
            value={username}
            onInput={(e) => setUsername(e.target.value)}
            disabled={loading}
            autoFocus
            autocomplete="username"
          />

          <label class="login-label">Password</label>
          <input
            type="password"
            class="login-input"
            placeholder="password"
            value={password}
            onInput={(e) => setPassword(e.target.value)}
            disabled={loading}
            autocomplete="current-password"
          />

          {error && (
            <ErrorBanner
              message={error}
              onDismiss={() => setError(null)}
            />
          )}

          <Button
            type="submit"
            disabled={loading || !username.trim() || !password}
            style={{ width: '100%', marginTop: 'var(--space-4)' }}
          >
            {loading ? <Spinner /> : 'Log In'}
          </Button>
        </form>
      </div>
    </div>
  );
}
