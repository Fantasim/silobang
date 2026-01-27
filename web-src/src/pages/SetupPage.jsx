import { useState } from 'preact/hooks';
import { Button } from '@components/ui/Button';
import { ErrorBanner } from '@components/ui/ErrorBanner';
import { Spinner } from '@components/ui/Spinner';
import { Icon } from '@components/ui/Icon';
import { setWorkingDirectory, configLoading, configError } from '@store/config';
import {
  bootstrapCredentials,
  clearBootstrapCredentials,
  markConfigured,
} from '@store/auth';

function CredentialRow({ label, value }) {
  const [copied, setCopied] = useState(false);

  const handleCopy = async () => {
    try {
      await navigator.clipboard.writeText(value);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch {
      // Fallback: select text for manual copy
    }
  };

  return (
    <div class="credential-row">
      <span class="credential-label">{label}</span>
      <div class="credential-value">
        <span class="credential-value-text">{value}</span>
        <button
          class={`credential-copy-btn ${copied ? 'credential-copy-btn--copied' : ''}`}
          onClick={handleCopy}
          title={copied ? 'Copied' : 'Copy to clipboard'}
          type="button"
        >
          <Icon name={copied ? 'check' : 'copy'} size={14} />
        </button>
      </div>
    </div>
  );
}

function BootstrapCredentialDisplay() {
  const creds = bootstrapCredentials.value;

  const handleAcknowledge = () => {
    clearBootstrapCredentials();
    markConfigured();
  };

  return (
    <div class="setup-page">
      <div class="setup-card setup-card--wide">
        <h1 class="setup-title">Admin Account Created</h1>

        <div class="bootstrap-warning">
          <Icon name="alertTriangle" size={18} />
          <span>Save these credentials now. They will NOT be shown again.</span>
        </div>

        <div class="bootstrap-credentials">
          <CredentialRow label="Username" value={creds.username} />
          <CredentialRow label="Password" value={creds.password} />
          <CredentialRow label="API Key" value={creds.api_key} />
        </div>

        <Button
          onClick={handleAcknowledge}
          style={{ width: '100%', marginTop: 'var(--space-4)' }}
        >
          I have saved these credentials — Continue to Login
        </Button>
      </div>
    </div>
  );
}

export function SetupPage() {
  const [path, setPath] = useState('');

  // If bootstrap credentials exist, show them instead of the setup form
  if (bootstrapCredentials.value) {
    return <BootstrapCredentialDisplay />;
  }

  const handleSubmit = async (e) => {
    e.preventDefault();
    if (!path.trim()) return;
    await setWorkingDirectory(path.trim());
  };

  return (
    <div class="setup-page">
      <div class="setup-card">
        <h1 class="setup-title">MeshBank</h1>
        <p class="setup-subtitle">
          Content-addressed asset storage for 3D models
        </p>

        <form onSubmit={handleSubmit}>
          <label style={{ display: 'block', marginBottom: 'var(--space-2)', color: 'var(--text-secondary)', fontSize: 'var(--font-sm)' }}>
            Working Directory
          </label>
          <input
            type="text"
            class="setup-input"
            placeholder="/path/to/your/data"
            value={path}
            onInput={(e) => setPath(e.target.value)}
            disabled={configLoading.value}
          />

          {configError.value && (
            <ErrorBanner
              message={configError.value}
              onDismiss={() => {}}
            />
          )}

          <Button
            type="submit"
            disabled={configLoading.value || !path.trim()}
            style={{ width: '100%', marginTop: 'var(--space-4)' }}
          >
            {configLoading.value ? <Spinner /> : 'Set Working Directory'}
          </Button>
        </form>
      </div>
    </div>
  );
}
