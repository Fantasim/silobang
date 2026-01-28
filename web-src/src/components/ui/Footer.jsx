import { serviceInfo } from '@store/topics';
import { APP_REPO_URL, APP_DISPLAY_NAME } from '@constants/ui';

/**
 * Check if a version string represents a real release (not a dev build).
 * Dev builds use the default "dev" value set in version.go.
 */
function isReleaseVersion(v) {
  return v && v !== 'dev';
}

/**
 * Application footer displayed on every authenticated page.
 * Shows the app version with a link to the GitHub repository.
 * Sticks to the bottom of the viewport when page content is short.
 */
export function Footer() {
  const version = serviceInfo.value?.version_info?.app_version;
  const versionLabel = isReleaseVersion(version) ? ` v${version}` : '';

  return (
    <footer class="app-footer">
      <a
        href={APP_REPO_URL}
        target="_blank"
        rel="noopener noreferrer"
        class="app-footer-link"
      >
        {APP_DISPLAY_NAME}{versionLabel}
      </a>
    </footer>
  );
}
