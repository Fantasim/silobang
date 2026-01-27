# Phase 6: Frontend Dashboard

## Objective
Build the Preact frontend dashboard, reusing the design system from meshview. The frontend is bundled with Vite and embedded in the Go binary via `embed.FS`.

---

## Prerequisites
- Phase 1-5 completed (full backend API functional)
- meshview repository cloned alongside meshbank
- Node.js 18+ installed

---

## Tech Stack
- **Framework:** Preact + Preact Signals (shared from meshview)
- **Build:** Vite
- **Styling:** CSS Variables from meshview's design system (dark terminal aesthetic)
- **Router:** History API (Go serves index.html for all non-API routes)
- **Embed:** Go `embed.FS` serves `web/dist/`
- **Offline:** Must work fully offline (no CDN dependencies)

---

## Task 1: Project Structure

```
meshbank/
├── web-src/                        # Preact source
│   ├── src/
│   │   ├── components/
│   │   │   ├── ui/  ──────────────→ symlink to meshview/src/components/ui
│   │   │   └── dashboard/          # Dashboard-specific components
│   │   │       ├── TopicCard.jsx
│   │   │       ├── UploadZone.jsx
│   │   │       ├── DataTable.jsx
│   │   │       ├── QueryForm.jsx
│   │   │       └── NavBar.jsx
│   │   ├── styles/  ──────────────→ symlink to meshview/src/styles
│   │   ├── hooks/  ───────────────→ symlink to meshview/src/hooks
│   │   ├── constants/  ───────────→ symlink to meshview/src/constants
│   │   ├── pages/
│   │   │   ├── SetupPage.jsx
│   │   │   ├── DashboardPage.jsx
│   │   │   ├── TopicPage.jsx
│   │   │   ├── QueryPage.jsx
│   │   │   └── LogsPage.jsx
│   │   ├── store/
│   │   │   ├── config.js
│   │   │   ├── topics.js
│   │   │   ├── upload.js
│   │   │   ├── query.js
│   │   │   └── ui.js
│   │   ├── services/
│   │   │   └── api.js
│   │   ├── App.jsx
│   │   ├── Router.jsx
│   │   └── main.jsx
│   ├── index.html
│   ├── vite.config.js
│   ├── package.json
│   ├── setup.sh                    # Symlink setup script
│   └── Makefile                    # Build commands
└── web/
    └── dist/                       # Built output (git-ignored, embedded in Go)
        ├── index.html
        ├── assets/
        │   ├── index-[hash].js
        │   └── index-[hash].css
```

---

## Task 2: Setup Script & Makefile

### 2.1 Setup Script (`web-src/setup.sh`)

```bash
#!/bin/bash
set -e

# ============================================================================
# MeshBank Dashboard - Symlink Setup
# Creates symlinks to meshview's shared components and styles
# ============================================================================

# Default meshview path (override with MESHVIEW_PATH env var)
MESHVIEW_SRC="${MESHVIEW_PATH:-/home/louis/Desktop/Flyff/meshview/src}"

echo "Setting up meshbank dashboard..."
echo "Using meshview at: $MESHVIEW_SRC"

# Validate meshview exists
if [ ! -d "$MESHVIEW_SRC" ]; then
    echo ""
    echo "Error: meshview not found at $MESHVIEW_SRC"
    echo ""
    echo "Options:"
    echo "  1. Clone meshview to the expected location"
    echo "  2. Set MESHVIEW_PATH environment variable:"
    echo "     export MESHVIEW_PATH=/path/to/meshview/src"
    echo "     ./setup.sh"
    echo ""
    exit 1
fi

# Validate required directories exist in meshview
REQUIRED_DIRS=("components/ui" "styles" "hooks" "constants")
for dir in "${REQUIRED_DIRS[@]}"; do
    if [ ! -d "$MESHVIEW_SRC/$dir" ]; then
        echo "Error: Required directory not found: $MESHVIEW_SRC/$dir"
        exit 1
    fi
done

# Create src directories if they don't exist
mkdir -p src/components
mkdir -p src/pages
mkdir -p src/store
mkdir -p src/services

# Remove existing symlinks (if any)
rm -f src/components/ui
rm -f src/styles
rm -f src/hooks
rm -f src/constants

# Create symlinks (absolute paths)
ln -s "$MESHVIEW_SRC/components/ui" src/components/ui
ln -s "$MESHVIEW_SRC/styles" src/styles
ln -s "$MESHVIEW_SRC/hooks" src/hooks
ln -s "$MESHVIEW_SRC/constants" src/constants

echo ""
echo "Symlinks created successfully:"
echo "  src/components/ui -> $MESHVIEW_SRC/components/ui"
echo "  src/styles        -> $MESHVIEW_SRC/styles"
echo "  src/hooks         -> $MESHVIEW_SRC/hooks"
echo "  src/constants     -> $MESHVIEW_SRC/constants"
echo ""
echo "Run 'make install' to install dependencies"
```

### 2.2 Makefile (`web-src/Makefile`)

```makefile
# ============================================================================
# MeshBank Dashboard - Build System
# ============================================================================

.PHONY: help setup install dev build clean check-symlinks

# Default target
help:
	@echo "MeshBank Dashboard - Available Commands"
	@echo ""
	@echo "  make setup    - Create symlinks to meshview (run once)"
	@echo "  make install  - Install npm dependencies"
	@echo "  make dev      - Start development server (port 5173)"
	@echo "  make build    - Build for production (outputs to ../web/dist)"
	@echo "  make clean    - Remove build artifacts and node_modules"
	@echo "  make check    - Verify symlinks are correctly configured"
	@echo ""
	@echo "First time setup:"
	@echo "  make setup && make install && make dev"
	@echo ""
	@echo "Environment variables:"
	@echo "  MESHVIEW_PATH - Path to meshview/src (default: /home/louis/Desktop/Flyff/meshview/src)"

# Create symlinks to meshview
setup:
	@chmod +x setup.sh
	@./setup.sh

# Install dependencies
install: check-symlinks
	npm install

# Development server with hot reload
dev: check-symlinks
	npm run dev

# Production build
build: check-symlinks
	npm run build
	@echo ""
	@echo "Build complete: ../web/dist/"
	@echo "Ready to embed in Go binary"

# Clean everything
clean:
	rm -rf node_modules
	rm -rf ../web/dist
	rm -f src/components/ui
	rm -f src/styles
	rm -f src/hooks
	rm -f src/constants

# Verify symlinks exist
check-symlinks:
	@if [ ! -L src/components/ui ]; then \
		echo "Error: Symlinks not configured. Run 'make setup' first."; \
		exit 1; \
	fi
	@if [ ! -d src/components/ui ]; then \
		echo "Error: Symlink target not found. Check MESHVIEW_PATH."; \
		exit 1; \
	fi
```

---

## Task 3: Vite Configuration

### 3.1 `web-src/package.json`

```json
{
  "name": "meshbank-dashboard",
  "version": "1.0.0",
  "private": true,
  "type": "module",
  "scripts": {
    "dev": "vite --port 5173",
    "build": "vite build",
    "preview": "vite preview"
  },
  "dependencies": {
    "preact": "^10.28.1",
    "@preact/signals": "^1.3.1"
  },
  "devDependencies": {
    "@preact/preset-vite": "^2.9.4",
    "vite": "^5.1.8"
  }
}
```

### 3.2 `web-src/vite.config.js`

```javascript
import { defineConfig } from 'vite';
import preact from '@preact/preset-vite';
import path from 'path';

export default defineConfig({
  plugins: [preact()],

  resolve: {
    alias: {
      // Dashboard-specific
      '@pages': path.resolve(__dirname, 'src/pages'),
      '@store': path.resolve(__dirname, 'src/store'),
      '@services': path.resolve(__dirname, 'src/services'),
      '@dashboard': path.resolve(__dirname, 'src/components/dashboard'),

      // Shared from meshview (via symlinks)
      '@components': path.resolve(__dirname, 'src/components'),
      '@styles': path.resolve(__dirname, 'src/styles'),
      '@hooks': path.resolve(__dirname, 'src/hooks'),
      '@constants': path.resolve(__dirname, 'src/constants'),
    },
  },

  build: {
    outDir: '../web/dist',
    emptyOutDir: true,
    // Single bundle for embedding
    rollupOptions: {
      output: {
        manualChunks: undefined,
      },
    },
  },

  server: {
    // Proxy API calls to Go backend during development
    proxy: {
      '/api': {
        target: 'http://localhost:2369',
        changeOrigin: true,
      },
    },
  },
});
```

### 3.3 `web-src/index.html`

```html
<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>MeshBank</title>
  <link rel="icon" type="image/svg+xml" href="/favicon.svg">
</head>
<body>
  <div id="app"></div>
  <script type="module" src="/src/main.jsx"></script>
</body>
</html>
```

---

## Task 4: Application Entry Points

### 4.1 `web-src/src/main.jsx`

```jsx
import { render } from 'preact';
import { App } from './App';

// Import global styles from meshview
import '@styles/tokens.css';
import '@styles/base.css';
import '@styles/components.css';

// Dashboard-specific styles
import './dashboard.css';

render(<App />, document.getElementById('app'));
```

### 4.2 `web-src/src/App.jsx`

```jsx
import { useEffect } from 'preact/hooks';
import { Router } from './Router';
import { Toast } from '@components/ui/Toast';
import { NavBar } from '@dashboard/NavBar';
import { fetchConfig } from '@store/config';

export function App() {
  useEffect(() => {
    // Load initial config on mount
    fetchConfig();
  }, []);

  return (
    <div class="app">
      <NavBar />
      <main class="main-content">
        <Router />
      </main>
      <Toast />
    </div>
  );
}
```

### 4.3 `web-src/src/Router.jsx`

```jsx
import { signal, computed } from '@preact/signals';
import { useEffect } from 'preact/hooks';

import { SetupPage } from '@pages/SetupPage';
import { DashboardPage } from '@pages/DashboardPage';
import { TopicPage } from '@pages/TopicPage';
import { QueryPage } from '@pages/QueryPage';
import { LogsPage } from '@pages/LogsPage';

import { isConfigured } from '@store/config';

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

  if (path === '/query') return { page: 'query' };
  if (path === '/logs') return { page: 'logs' };

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

  // If not configured, always show setup
  if (!isConfigured.value) {
    return <SetupPage />;
  }

  // Route to appropriate page
  const { page, name } = routeParams.value;

  switch (page) {
    case 'topic':
      return <TopicPage topicName={name} />;
    case 'query':
      return <QueryPage />;
    case 'logs':
      return <LogsPage />;
    default:
      return <DashboardPage />;
  }
}
```

### 4.4 `web-src/src/dashboard.css`

```css
/* Dashboard-specific styles (extends meshview design system) */

.app {
  min-height: 100vh;
  background: var(--bg-primary);
  color: var(--text-primary);
  font-family: var(--font-mono);
}

.main-content {
  padding: var(--space-6);
  max-width: 1400px;
  margin: 0 auto;
}

/* Navigation bar */
.navbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: var(--space-3) var(--space-6);
  background: var(--bg-secondary);
  border-bottom: 1px solid var(--border-dim);
}

.navbar-brand {
  font-size: var(--font-xl);
  font-weight: 600;
  color: var(--terminal-green);
  text-decoration: none;
  letter-spacing: var(--tracking-wide);
}

.navbar-links {
  display: flex;
  gap: var(--space-4);
  padding-left: var(--space-4);
}

.navbar-link {
  color: var(--text-secondary);
  text-decoration: none;
  font-size: var(--font-sm);
  padding: var(--space-2) var(--space-3);
  border-radius: var(--radius-md);
  transition: all var(--transition-fast);
}

.navbar-link:hover {
  color: var(--text-primary);
  background: var(--bg-tertiary);
}

.navbar-link.active {
  color: var(--terminal-green);
  background: var(--bg-tertiary);
}

/* Page titles */
.page-title {
  font-size: var(--font-2xl);
  font-weight: 600;
  color: var(--text-primary);
  margin-bottom: var(--space-6);
  letter-spacing: var(--tracking-wide);
}

/* Topic cards grid */
.topics-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(350px, 1fr));
  gap: var(--space-4);
}

/* Topic card */
.topic-card {
  background: var(--bg-secondary);
  border: 1px solid var(--border-dim);
  border-radius: var(--radius-lg);
  padding: var(--space-4);
  cursor: pointer;
  transition: all var(--transition-fast);
}

.topic-card:hover {
  border-color: var(--border-bright);
  box-shadow: var(--glow-subtle);
}

.topic-card.unhealthy {
  border-color: var(--terminal-red);
  opacity: 0.7;
}

.topic-card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: var(--space-3);
}

.topic-card-name {
  font-size: var(--font-lg);
  font-weight: 600;
  color: var(--text-primary);
}

.topic-card-status {
  width: 10px;
  height: 10px;
  border-radius: var(--radius-full);
}

.topic-card-status.healthy {
  background: var(--terminal-green);
  box-shadow: 0 0 8px var(--terminal-green);
}

.topic-card-status.unhealthy {
  background: var(--terminal-red);
  box-shadow: 0 0 8px var(--terminal-red);
}

/* Upload zone */
.upload-zone {
  border: 2px dashed var(--border-dim);
  border-radius: var(--radius-lg);
  padding: var(--space-8);
  text-align: center;
  cursor: pointer;
  transition: all var(--transition-fast);
}

.upload-zone:hover,
.upload-zone.drag-over {
  border-color: var(--terminal-green);
  background: rgba(0, 255, 65, 0.05);
}

.upload-zone-icon {
  font-size: 48px;
  margin-bottom: var(--space-4);
  color: var(--text-secondary);
}

.upload-zone-text {
  color: var(--text-secondary);
  font-size: var(--font-md);
}

.upload-zone-hint {
  color: var(--text-dim);
  font-size: var(--font-sm);
  margin-top: var(--space-2);
}

/* Upload progress */
.upload-progress {
  margin-top: var(--space-4);
  padding: var(--space-4);
  background: var(--bg-tertiary);
  border-radius: var(--radius-md);
}

.upload-stats {
  display: flex;
  gap: var(--space-6);
  margin-bottom: var(--space-3);
}

.upload-stat {
  font-size: var(--font-sm);
}

.upload-stat-value {
  font-weight: 600;
}

.upload-stat-value.added { color: var(--terminal-green); }
.upload-stat-value.skipped { color: var(--terminal-amber); }
.upload-stat-value.errors { color: var(--terminal-red); }

/* File queue */
.file-queue {
  margin-top: var(--space-4);
  max-height: 200px;
  overflow-y: auto;
}

.file-queue-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: var(--space-2);
  font-size: var(--font-sm);
  border-bottom: 1px solid var(--border-dim);
}

.file-queue-item:last-child {
  border-bottom: none;
}

.file-queue-item.pending { color: var(--text-secondary); }
.file-queue-item.uploading { color: var(--terminal-cyan); }
.file-queue-item.success { color: var(--terminal-green); }
.file-queue-item.error { color: var(--terminal-red); }
.file-queue-item.skipped { color: var(--terminal-amber); }

/* Data table */
.data-table {
  width: 100%;
  border-collapse: collapse;
  font-size: var(--font-sm);
}

.data-table th {
  text-align: left;
  padding: var(--space-3);
  background: var(--bg-tertiary);
  border-bottom: 1px solid var(--border-dim);
  color: var(--text-secondary);
  font-weight: 600;
  cursor: pointer;
  user-select: none;
}

.data-table th:hover {
  color: var(--text-primary);
}

.data-table th.sorted {
  color: var(--terminal-green);
}

.data-table td {
  padding: var(--space-3);
  border-bottom: 1px solid var(--border-dim);
}

.data-table tr:hover td {
  background: var(--bg-tertiary);
}

/* Query form */
.query-form {
  background: var(--bg-secondary);
  border: 1px solid var(--border-dim);
  border-radius: var(--radius-lg);
  padding: var(--space-4);
  margin-bottom: var(--space-4);
}

.query-params {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
  gap: var(--space-3);
  margin: var(--space-4) 0;
}

.query-param {
  display: flex;
  flex-direction: column;
  gap: var(--space-1);
}

.query-param label {
  font-size: var(--font-xs);
  color: var(--text-secondary);
  text-transform: uppercase;
  letter-spacing: var(--tracking-wide);
}

.query-param input,
.query-param select {
  background: var(--bg-tertiary);
  border: 1px solid var(--border-dim);
  border-radius: var(--radius-md);
  padding: var(--space-2) var(--space-3);
  color: var(--text-primary);
  font-family: var(--font-mono);
  font-size: var(--font-md);
}

.query-param input:focus,
.query-param select:focus {
  outline: none;
  border-color: var(--terminal-green);
}

/* Empty state */
.empty-state {
  text-align: center;
  padding: var(--space-8);
  color: var(--text-secondary);
}

.empty-state-icon {
  font-size: 64px;
  margin-bottom: var(--space-4);
  opacity: 0.5;
}

.empty-state-text {
  font-size: var(--font-lg);
  margin-bottom: var(--space-2);
}

.empty-state-hint {
  font-size: var(--font-sm);
  color: var(--text-dim);
}

/* Setup page */
.setup-page {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 80vh;
}

.setup-card {
  background: var(--bg-secondary);
  border: 1px solid var(--border-dim);
  border-radius: var(--radius-lg);
  padding: var(--space-8);
  max-width: 500px;
  width: 100%;
}

.setup-title {
  font-size: var(--font-2xl);
  font-weight: 600;
  color: var(--terminal-green);
  margin-bottom: var(--space-2);
  text-align: center;
}

.setup-subtitle {
  color: var(--text-secondary);
  text-align: center;
  margin-bottom: var(--space-6);
}

.setup-input {
  width: 100%;
  background: var(--bg-tertiary);
  border: 1px solid var(--border-dim);
  border-radius: var(--radius-md);
  padding: var(--space-3);
  color: var(--text-primary);
  font-family: var(--font-mono);
  font-size: var(--font-md);
  margin-bottom: var(--space-4);
}

.setup-input:focus {
  outline: none;
  border-color: var(--terminal-green);
}

/* Pagination */
.pagination {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: var(--space-2);
  margin-top: var(--space-4);
}

.pagination-info {
  color: var(--text-secondary);
  font-size: var(--font-sm);
  margin: 0 var(--space-4);
}
```

---

## Task 5: Store (State Management)

### 5.1 `web-src/src/store/config.js`

```javascript
import { signal, computed } from '@preact/signals';
import { api } from '@services/api';

// State
export const config = signal(null);
export const configLoading = signal(true);
export const configError = signal(null);

// Computed
export const isConfigured = computed(() =>
  config.value?.configured === true
);

// Actions
export async function fetchConfig() {
  configLoading.value = true;
  configError.value = null;

  try {
    const data = await api.getConfig();
    config.value = data;
  } catch (err) {
    configError.value = err.message;
  } finally {
    configLoading.value = false;
  }
}

export async function setWorkingDirectory(path) {
  configLoading.value = true;
  configError.value = null;

  try {
    await api.setConfig(path);
    await fetchConfig(); // Refresh config
    return true;
  } catch (err) {
    configError.value = err.message;
    return false;
  } finally {
    configLoading.value = false;
  }
}
```

### 5.2 `web-src/src/store/topics.js`

```javascript
import { signal, computed } from '@preact/signals';
import { api } from '@services/api';

// State
export const topics = signal([]);
export const topicsLoading = signal(false);
export const topicsError = signal(null);

// Computed
export const healthyTopics = computed(() =>
  topics.value.filter(t => t.healthy)
);

export const unhealthyTopics = computed(() =>
  topics.value.filter(t => !t.healthy)
);

// Actions
export async function fetchTopics() {
  topicsLoading.value = true;
  topicsError.value = null;

  try {
    const data = await api.getTopics();
    topics.value = data.topics || [];
  } catch (err) {
    topicsError.value = err.message;
  } finally {
    topicsLoading.value = false;
  }
}

export async function createTopic(name) {
  try {
    await api.createTopic(name);
    await fetchTopics(); // Refresh list
    return { success: true };
  } catch (err) {
    return { success: false, error: err.message };
  }
}

export function getTopicByName(name) {
  return topics.value.find(t => t.name === name);
}
```

### 5.3 `web-src/src/store/upload.js`

```javascript
import { signal, computed } from '@preact/signals';
import { api } from '@services/api';

// File status enum
export const FileStatus = {
  PENDING: 'pending',
  UPLOADING: 'uploading',
  SUCCESS: 'success',
  SKIPPED: 'skipped',
  ERROR: 'error',
};

// State
export const uploadQueue = signal([]);  // Array of { file, status, error?, result? }
export const isUploading = signal(false);
export const currentUploadIndex = signal(-1);
export const parentId = signal('');

// Computed
export const uploadStats = computed(() => {
  const queue = uploadQueue.value;
  return {
    total: queue.length,
    pending: queue.filter(f => f.status === FileStatus.PENDING).length,
    uploading: queue.filter(f => f.status === FileStatus.UPLOADING).length,
    success: queue.filter(f => f.status === FileStatus.SUCCESS).length,
    skipped: queue.filter(f => f.status === FileStatus.SKIPPED).length,
    errors: queue.filter(f => f.status === FileStatus.ERROR).length,
  };
});

export const uploadProgress = computed(() => {
  const stats = uploadStats.value;
  const completed = stats.success + stats.skipped + stats.errors;
  return stats.total > 0 ? Math.round((completed / stats.total) * 100) : 0;
});

export const uploadErrors = computed(() =>
  uploadQueue.value.filter(f => f.status === FileStatus.ERROR)
);

// Actions
export function addFilesToQueue(files) {
  const newFiles = Array.from(files).map(file => ({
    file,
    status: FileStatus.PENDING,
    error: null,
    result: null,
  }));
  uploadQueue.value = [...uploadQueue.value, ...newFiles];
}

export function removeFromQueue(index) {
  if (isUploading.value) return; // Can't remove during upload
  uploadQueue.value = uploadQueue.value.filter((_, i) => i !== index);
}

export function clearQueue() {
  if (isUploading.value) return;
  uploadQueue.value = [];
  currentUploadIndex.value = -1;
}

export function cancelUpload() {
  isUploading.value = false;
}

export async function startUpload(topicName) {
  if (isUploading.value || uploadQueue.value.length === 0) return;

  isUploading.value = true;

  for (let i = 0; i < uploadQueue.value.length; i++) {
    if (!isUploading.value) break; // Check for cancel

    const item = uploadQueue.value[i];
    if (item.status !== FileStatus.PENDING) continue;

    currentUploadIndex.value = i;

    // Update status to uploading
    uploadQueue.value = uploadQueue.value.map((f, idx) =>
      idx === i ? { ...f, status: FileStatus.UPLOADING } : f
    );

    try {
      const result = await api.uploadAsset(
        topicName,
        item.file,
        parentId.value || null
      );

      // Update with result
      uploadQueue.value = uploadQueue.value.map((f, idx) =>
        idx === i ? {
          ...f,
          status: result.skipped ? FileStatus.SKIPPED : FileStatus.SUCCESS,
          result,
        } : f
      );
    } catch (err) {
      // Update with error
      uploadQueue.value = uploadQueue.value.map((f, idx) =>
        idx === i ? {
          ...f,
          status: FileStatus.ERROR,
          error: err.message,
        } : f
      );
    }
  }

  isUploading.value = false;
  currentUploadIndex.value = -1;
}
```

### 5.4 `web-src/src/store/query.js`

```javascript
import { signal } from '@preact/signals';
import { api } from '@services/api';

// State
export const presets = signal([]);
export const presetsLoading = signal(false);
export const selectedPreset = signal(null);
export const queryParams = signal({});
export const selectedTopics = signal([]);
export const queryResult = signal(null);
export const queryLoading = signal(false);
export const queryError = signal(null);

// Pagination
export const currentPage = signal(1);
export const pageSize = signal(100);

// Sorting
export const sortColumn = signal(null);
export const sortDirection = signal('asc');

// Actions
export async function fetchPresets() {
  presetsLoading.value = true;

  try {
    const data = await api.getQueries();
    presets.value = data.presets || [];
  } catch (err) {
    console.error('Failed to fetch presets:', err);
  } finally {
    presetsLoading.value = false;
  }
}

export function selectPreset(presetName) {
  const preset = presets.value.find(p => p.name === presetName);
  selectedPreset.value = preset || null;

  // Initialize params with defaults
  if (preset) {
    const defaults = {};
    for (const param of preset.params) {
      defaults[param.name] = param.default || '';
    }
    queryParams.value = defaults;
  } else {
    queryParams.value = {};
  }

  // Reset results
  queryResult.value = null;
  queryError.value = null;
  currentPage.value = 1;
}

export function setQueryParam(name, value) {
  queryParams.value = { ...queryParams.value, [name]: value };
}

export async function runQuery() {
  if (!selectedPreset.value) return;

  queryLoading.value = true;
  queryError.value = null;

  try {
    const result = await api.runQuery(
      selectedPreset.value.name,
      queryParams.value,
      selectedTopics.value
    );
    queryResult.value = result;
    currentPage.value = 1;
    sortColumn.value = null;
    sortDirection.value = 'asc';
  } catch (err) {
    queryError.value = err.message;
    queryResult.value = null;
  } finally {
    queryLoading.value = false;
  }
}

export function setSorting(column) {
  if (sortColumn.value === column) {
    sortDirection.value = sortDirection.value === 'asc' ? 'desc' : 'asc';
  } else {
    sortColumn.value = column;
    sortDirection.value = 'asc';
  }
}
```

### 5.5 `web-src/src/store/ui.js`

```javascript
import { signal } from '@preact/signals';

// Toast notification
export const toastMessage = signal(null);
let toastTimeout = null;

export function showToast(message, duration = 2000) {
  if (toastTimeout) clearTimeout(toastTimeout);
  toastMessage.value = message;
  toastTimeout = setTimeout(() => {
    toastMessage.value = null;
  }, duration);
}

// Modal state helper
export function createModal(name) {
  const isOpen = signal(false);

  return {
    isOpen,
    open: () => { isOpen.value = true; },
    close: () => { isOpen.value = false; },
    toggle: () => { isOpen.value = !isOpen.value; },
  };
}

// Create topic modal
export const createTopicModal = createModal('create-topic');
```

---

## Task 6: API Service

### 6.1 `web-src/src/services/api.js`

```javascript
const API_BASE = '/api';

class ApiError extends Error {
  constructor(message, code, status) {
    super(message);
    this.code = code;
    this.status = status;
  }
}

async function request(endpoint, options = {}) {
  const url = `${API_BASE}${endpoint}`;

  const response = await fetch(url, {
    headers: {
      'Content-Type': 'application/json',
      ...options.headers,
    },
    ...options,
  });

  const data = await response.json();

  if (data.error) {
    throw new ApiError(data.message, data.code, response.status);
  }

  return data;
}

export const api = {
  // Config
  async getConfig() {
    return request('/config');
  },

  async setConfig(workingDirectory) {
    return request('/config', {
      method: 'POST',
      body: JSON.stringify({ working_directory: workingDirectory }),
    });
  },

  // Topics
  async getTopics() {
    return request('/topics');
  },

  async createTopic(name) {
    return request('/topics', {
      method: 'POST',
      body: JSON.stringify({ name }),
    });
  },

  // Assets
  async uploadAsset(topicName, file, parentId = null) {
    const formData = new FormData();
    formData.append('file', file);
    if (parentId) {
      formData.append('parent_id', parentId);
    }

    const response = await fetch(`${API_BASE}/topics/${topicName}/assets`, {
      method: 'POST',
      body: formData,
      // Don't set Content-Type - browser sets it with boundary for multipart
    });

    const data = await response.json();

    if (data.error) {
      throw new ApiError(data.message, data.code, response.status);
    }

    return data;
  },

  async downloadAsset(hash) {
    window.open(`${API_BASE}/assets/${hash}/download`, '_blank');
  },

  // Queries
  async getQueries() {
    return request('/queries');
  },

  async runQuery(preset, params = {}, topics = []) {
    return request(`/query/${preset}`, {
      method: 'POST',
      body: JSON.stringify({ params, topics }),
    });
  },

  // Logs
  async getLogs() {
    return request('/logs');
  },
};
```

---

## Task 7: Pages

### 7.1 `web-src/src/pages/SetupPage.jsx`

```jsx
import { useState } from 'preact/hooks';
import { Button } from '@components/ui/Button';
import { ErrorBanner } from '@components/ui/ErrorBanner';
import { Spinner } from '@components/ui/Spinner';
import { setWorkingDirectory, configLoading, configError } from '@store/config';

export function SetupPage() {
  const [path, setPath] = useState('');

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
```

### 7.2 `web-src/src/pages/DashboardPage.jsx`

```jsx
import { useEffect } from 'preact/hooks';
import { Button } from '@components/ui/Button';
import { Modal } from '@components/ui/Modal';
import { Spinner } from '@components/ui/Spinner';
import { TopicCard } from '@dashboard/TopicCard';
import {
  topics,
  topicsLoading,
  fetchTopics,
  createTopic
} from '@store/topics';
import { createTopicModal, showToast } from '@store/ui';
import { navigate } from '../Router';
import { useState } from 'preact/hooks';

export function DashboardPage() {
  const [newTopicName, setNewTopicName] = useState('');
  const [createError, setCreateError] = useState(null);

  useEffect(() => {
    fetchTopics();
  }, []);

  const handleCreateTopic = async () => {
    if (!newTopicName.trim()) return;

    setCreateError(null);
    const result = await createTopic(newTopicName.trim());

    if (result.success) {
      createTopicModal.close();
      setNewTopicName('');
      showToast(`Topic "${newTopicName}" created`);
    } else {
      setCreateError(result.error);
    }
  };

  if (topicsLoading.value && topics.value.length === 0) {
    return (
      <div class="empty-state">
        <Spinner />
      </div>
    );
  }

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 'var(--space-6)' }}>
        <h1 class="page-title">Topics</h1>
        <Button onClick={createTopicModal.open}>
          + Create Topic
        </Button>
      </div>

      {topics.value.length === 0 ? (
        <div class="empty-state">
          <div class="empty-state-icon">📦</div>
          <div class="empty-state-text">No topics yet</div>
          <div class="empty-state-hint">Create a topic to start storing assets</div>
        </div>
      ) : (
        <div class="topics-grid">
          {topics.value.map(topic => (
            <TopicCard
              key={topic.name}
              topic={topic}
              onClick={() => navigate(`/topic/${topic.name}`)}
            />
          ))}
        </div>
      )}

      {/* Create Topic Modal */}
      <Modal
        isOpen={createTopicModal.isOpen.value}
        onClose={createTopicModal.close}
        title="Create Topic"
      >
        <div style={{ padding: 'var(--space-4)' }}>
          <label style={{ display: 'block', marginBottom: 'var(--space-2)', color: 'var(--text-secondary)', fontSize: 'var(--font-sm)' }}>
            Topic Name
          </label>
          <input
            type="text"
            class="setup-input"
            placeholder="my-topic-name"
            value={newTopicName}
            onInput={(e) => setNewTopicName(e.target.value)}
            style={{ marginBottom: 'var(--space-2)' }}
          />
          <div style={{ fontSize: 'var(--font-xs)', color: 'var(--text-dim)', marginBottom: 'var(--space-4)' }}>
            Lowercase letters, numbers, hyphens, and underscores only
          </div>

          {createError && (
            <ErrorBanner message={createError} />
          )}

          <div style={{ display: 'flex', gap: 'var(--space-2)', justifyContent: 'flex-end' }}>
            <Button variant="ghost" onClick={createTopicModal.close}>
              Cancel
            </Button>
            <Button onClick={handleCreateTopic} disabled={!newTopicName.trim()}>
              Create
            </Button>
          </div>
        </div>
      </Modal>
    </div>
  );
}
```

### 7.3 `web-src/src/pages/TopicPage.jsx`

```jsx
import { useEffect } from 'preact/hooks';
import { StatsGrid } from '@components/ui/StatsGrid';
import { Button } from '@components/ui/Button';
import { UploadZone } from '@dashboard/UploadZone';
import {
  topics,
  fetchTopics,
  getTopicByName
} from '@store/topics';
import {
  uploadQueue,
  uploadStats,
  isUploading,
  parentId,
  startUpload,
  cancelUpload,
  clearQueue,
  FileStatus,
} from '@store/upload';
import { navigate } from '../Router';
import { formatBytes, formatDate } from '../utils/format';

export function TopicPage({ topicName }) {
  useEffect(() => {
    fetchTopics();
  }, [topicName]);

  const topic = getTopicByName(topicName);

  if (!topic) {
    return (
      <div class="empty-state">
        <div class="empty-state-text">Topic not found</div>
        <Button onClick={() => navigate('/')}>Back to Dashboard</Button>
      </div>
    );
  }

  const stats = topic.stats || {};
  const statsItems = [
    { label: 'Files', value: stats.file_count?.toLocaleString() || '0' },
    { label: 'Total Size', value: formatBytes(stats.total_size) },
    { label: 'DB Size', value: formatBytes(stats.db_size) },
    { label: 'DAT Size', value: formatBytes(stats.dat_size) },
    { label: 'Avg Size', value: formatBytes(stats.avg_size) },
    { label: 'Last Added', value: formatDate(stats.last_added) },
  ];

  const handleStartUpload = () => {
    startUpload(topicName);
  };

  return (
    <div>
      <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--space-4)', marginBottom: 'var(--space-6)' }}>
        <Button variant="ghost" onClick={() => navigate('/')}>
          ← Back
        </Button>
        <h1 class="page-title" style={{ marginBottom: 0 }}>{topicName}</h1>
        {!topic.healthy && (
          <span style={{ color: 'var(--terminal-red)', fontSize: 'var(--font-sm)' }}>
            Unhealthy: {topic.error}
          </span>
        )}
      </div>

      {/* Stats */}
      <div style={{ marginBottom: 'var(--space-6)' }}>
        <StatsGrid items={statsItems} columns={3} />
      </div>

      {/* Upload Section */}
      {topic.healthy && (
        <div>
          <h2 style={{ fontSize: 'var(--font-lg)', marginBottom: 'var(--space-4)' }}>
            Upload Assets
          </h2>

          {/* Parent ID input */}
          <div style={{ marginBottom: 'var(--space-4)' }}>
            <label style={{ display: 'block', marginBottom: 'var(--space-1)', color: 'var(--text-secondary)', fontSize: 'var(--font-sm)' }}>
              Parent ID (optional, for lineage tracking)
            </label>
            <input
              type="text"
              class="setup-input"
              placeholder="BLAKE3 hash of parent asset"
              value={parentId.value}
              onInput={(e) => parentId.value = e.target.value}
              disabled={isUploading.value}
              style={{ maxWidth: '500px', marginBottom: 0 }}
            />
          </div>

          {/* Upload zone */}
          <UploadZone disabled={isUploading.value} />

          {/* Upload controls */}
          {uploadQueue.value.length > 0 && (
            <div class="upload-progress">
              <div class="upload-stats">
                <div class="upload-stat">
                  Total: <span class="upload-stat-value">{uploadStats.value.total}</span>
                </div>
                <div class="upload-stat">
                  Added: <span class="upload-stat-value added">{uploadStats.value.success}</span>
                </div>
                <div class="upload-stat">
                  Skipped: <span class="upload-stat-value skipped">{uploadStats.value.skipped}</span>
                </div>
                <div class="upload-stat">
                  Errors: <span class="upload-stat-value errors">{uploadStats.value.errors}</span>
                </div>
              </div>

              <div style={{ display: 'flex', gap: 'var(--space-2)' }}>
                {!isUploading.value ? (
                  <>
                    <Button onClick={handleStartUpload} disabled={uploadStats.value.pending === 0}>
                      Start Upload
                    </Button>
                    <Button variant="ghost" onClick={clearQueue}>
                      Clear Queue
                    </Button>
                  </>
                ) : (
                  <Button variant="danger" onClick={cancelUpload}>
                    Cancel Upload
                  </Button>
                )}
              </div>

              {/* File queue list */}
              <div class="file-queue">
                {uploadQueue.value.map((item, index) => (
                  <div key={index} class={`file-queue-item ${item.status}`}>
                    <span>{item.file.name}</span>
                    <span>
                      {item.status === FileStatus.UPLOADING && '⏳'}
                      {item.status === FileStatus.SUCCESS && '✓'}
                      {item.status === FileStatus.SKIPPED && '⊘'}
                      {item.status === FileStatus.ERROR && `❌ ${item.error}`}
                      {item.status === FileStatus.PENDING && (
                        <button
                          onClick={() => removeFromQueue(index)}
                          style={{ background: 'none', border: 'none', color: 'var(--text-dim)', cursor: 'pointer' }}
                        >
                          ✕
                        </button>
                      )}
                    </span>
                  </div>
                ))}
              </div>
            </div>
          )}
        </div>
      )}
    </div>
  );
}
```

### 7.4 `web-src/src/pages/QueryPage.jsx`

```jsx
import { useEffect } from 'preact/hooks';
import { Button } from '@components/ui/Button';
import { Spinner } from '@components/ui/Spinner';
import { ErrorBanner } from '@components/ui/ErrorBanner';
import { DataTable } from '@dashboard/DataTable';
import { QueryForm } from '@dashboard/QueryForm';
import {
  presets,
  presetsLoading,
  selectedPreset,
  queryResult,
  queryLoading,
  queryError,
  fetchPresets,
  selectPreset,
  runQuery,
} from '@store/query';

export function QueryPage() {
  useEffect(() => {
    fetchPresets();
  }, []);

  const handleExportJson = () => {
    if (!queryResult.value) return;

    const blob = new Blob(
      [JSON.stringify(queryResult.value, null, 2)],
      { type: 'application/json' }
    );
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `${selectedPreset.value?.name || 'query'}-results.json`;
    a.click();
    URL.revokeObjectURL(url);
  };

  return (
    <div>
      <h1 class="page-title">Query Runner</h1>

      {presetsLoading.value ? (
        <Spinner />
      ) : (
        <>
          {/* Preset selector */}
          <div class="query-form">
            <label style={{ display: 'block', marginBottom: 'var(--space-2)', color: 'var(--text-secondary)', fontSize: 'var(--font-sm)' }}>
              Select Query Preset
            </label>
            <select
              value={selectedPreset.value?.name || ''}
              onChange={(e) => selectPreset(e.target.value)}
              style={{
                width: '100%',
                maxWidth: '300px',
                background: 'var(--bg-tertiary)',
                border: '1px solid var(--border-dim)',
                borderRadius: 'var(--radius-md)',
                padding: 'var(--space-2) var(--space-3)',
                color: 'var(--text-primary)',
                fontFamily: 'var(--font-mono)',
                fontSize: 'var(--font-md)',
              }}
            >
              <option value="">-- Select a preset --</option>
              {presets.value.map(p => (
                <option key={p.name} value={p.name}>
                  {p.name} - {p.description}
                </option>
              ))}
            </select>

            {/* Dynamic params */}
            {selectedPreset.value && (
              <QueryForm />
            )}

            {/* Run button */}
            <div style={{ marginTop: 'var(--space-4)' }}>
              <Button
                onClick={runQuery}
                disabled={!selectedPreset.value || queryLoading.value}
              >
                {queryLoading.value ? <Spinner /> : 'Run Query'}
              </Button>
            </div>
          </div>

          {/* Error */}
          {queryError.value && (
            <ErrorBanner message={queryError.value} />
          )}

          {/* Results */}
          {queryResult.value && (
            <div style={{ marginTop: 'var(--space-4)' }}>
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 'var(--space-3)' }}>
                <span style={{ color: 'var(--text-secondary)', fontSize: 'var(--font-sm)' }}>
                  {queryResult.value.row_count} row{queryResult.value.row_count !== 1 ? 's' : ''}
                </span>
                <Button variant="ghost" onClick={handleExportJson}>
                  Export JSON
                </Button>
              </div>

              <DataTable
                columns={queryResult.value.columns}
                rows={queryResult.value.rows}
              />
            </div>
          )}
        </>
      )}
    </div>
  );
}
```

### 7.5 `web-src/src/pages/LogsPage.jsx`

```jsx
import { useEffect, useState } from 'preact/hooks';
import { api } from '@services/api';

// TODO: Implement real log storage backend (Phase 8)
// Currently returns empty logs

export function LogsPage() {
  const [logs, setLogs] = useState([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    fetchLogs();
  }, []);

  const fetchLogs = async () => {
    setLoading(true);
    try {
      const data = await api.getLogs();
      setLogs(data.logs || []);
    } catch (err) {
      console.error('Failed to fetch logs:', err);
    } finally {
      setLoading(false);
    }
  };

  return (
    <div>
      <h1 class="page-title">System Logs</h1>

      {loading ? (
        <div class="empty-state">Loading...</div>
      ) : logs.length === 0 ? (
        <div class="empty-state">
          <div class="empty-state-icon">📋</div>
          <div class="empty-state-text">No logs</div>
          <div class="empty-state-hint">System logs will appear here when there are errors or warnings</div>
        </div>
      ) : (
        <table class="data-table">
          <thead>
            <tr>
              <th>Timestamp</th>
              <th>Level</th>
              <th>Topic</th>
              <th>Message</th>
            </tr>
          </thead>
          <tbody>
            {logs.map((log, i) => (
              <tr key={i}>
                <td>{new Date(log.timestamp * 1000).toLocaleString()}</td>
                <td style={{ color: log.level === 'error' ? 'var(--terminal-red)' : 'var(--terminal-amber)' }}>
                  {log.level.toUpperCase()}
                </td>
                <td>{log.topic || '-'}</td>
                <td>{log.message}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  );
}
```

---

## Task 8: Dashboard Components

### 8.1 `web-src/src/components/dashboard/NavBar.jsx`

```jsx
import { currentPath } from '../../Router';
import { navigate } from '../../Router';

export function NavBar() {
  const path = currentPath.value;

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
          class={`navbar-link ${path === '/logs' ? 'active' : ''}`}
          href="/logs"
          onClick={(e) => { e.preventDefault(); navigate('/logs'); }}
        >
          Logs
        </a>
      </div>
    </nav>
  );
}
```

### 8.2 `web-src/src/components/dashboard/TopicCard.jsx`

```jsx
import { StatsGrid } from '@components/ui/StatsGrid';
import { formatBytes, formatDate } from '../../utils/format';

export function TopicCard({ topic, onClick }) {
  const stats = topic.stats || {};

  const statsItems = [
    { label: 'Files', value: stats.file_count?.toLocaleString() || '0' },
    { label: 'Size', value: formatBytes(stats.total_size) },
  ];

  return (
    <div
      class={`topic-card ${topic.healthy ? '' : 'unhealthy'}`}
      onClick={onClick}
    >
      <div class="topic-card-header">
        <span class="topic-card-name">{topic.name}</span>
        <span class={`topic-card-status ${topic.healthy ? 'healthy' : 'unhealthy'}`} />
      </div>

      {topic.healthy ? (
        <StatsGrid items={statsItems} columns={2} />
      ) : (
        <div style={{ color: 'var(--terminal-red)', fontSize: 'var(--font-sm)' }}>
          {topic.error}
        </div>
      )}
    </div>
  );
}
```

### 8.3 `web-src/src/components/dashboard/UploadZone.jsx`

```jsx
import { useState, useRef } from 'preact/hooks';
import { addFilesToQueue } from '@store/upload';
import { Icon } from '@components/ui/Icon';

export function UploadZone({ disabled }) {
  const [isDragOver, setIsDragOver] = useState(false);
  const inputRef = useRef(null);

  const handleDragOver = (e) => {
    e.preventDefault();
    if (!disabled) setIsDragOver(true);
  };

  const handleDragLeave = () => {
    setIsDragOver(false);
  };

  const handleDrop = (e) => {
    e.preventDefault();
    setIsDragOver(false);
    if (disabled) return;

    const files = e.dataTransfer.files;
    if (files.length > 0) {
      addFilesToQueue(files);
    }
  };

  const handleClick = () => {
    if (!disabled) inputRef.current?.click();
  };

  const handleFileSelect = (e) => {
    const files = e.target.files;
    if (files.length > 0) {
      addFilesToQueue(files);
    }
    // Reset input so same file can be selected again
    e.target.value = '';
  };

  return (
    <div
      class={`upload-zone ${isDragOver ? 'drag-over' : ''}`}
      onDragOver={handleDragOver}
      onDragLeave={handleDragLeave}
      onDrop={handleDrop}
      onClick={handleClick}
      style={{ opacity: disabled ? 0.5 : 1, cursor: disabled ? 'not-allowed' : 'pointer' }}
    >
      <input
        ref={inputRef}
        type="file"
        multiple
        style={{ display: 'none' }}
        onChange={handleFileSelect}
        disabled={disabled}
      />

      <div class="upload-zone-icon">
        <Icon name="upload" size={48} color="var(--text-secondary)" />
      </div>
      <div class="upload-zone-text">
        Drag & drop files here
      </div>
      <div class="upload-zone-hint">
        or click to browse
      </div>
    </div>
  );
}
```

### 8.4 `web-src/src/components/dashboard/DataTable.jsx`

```jsx
import { sortColumn, sortDirection, setSorting, currentPage, pageSize } from '@store/query';
import { computed } from '@preact/signals';
import { formatBytes, formatDate } from '../../utils/format';
import { Button } from '@components/ui/Button';

export function DataTable({ columns, rows }) {
  // Client-side sorting
  const sortedRows = computed(() => {
    if (!sortColumn.value || !rows) return rows;

    const colIndex = columns.indexOf(sortColumn.value);
    if (colIndex === -1) return rows;

    return [...rows].sort((a, b) => {
      const aVal = a[colIndex];
      const bVal = b[colIndex];

      // Handle nulls
      if (aVal == null && bVal == null) return 0;
      if (aVal == null) return 1;
      if (bVal == null) return -1;

      // Compare
      let cmp = 0;
      if (typeof aVal === 'number' && typeof bVal === 'number') {
        cmp = aVal - bVal;
      } else {
        cmp = String(aVal).localeCompare(String(bVal));
      }

      return sortDirection.value === 'asc' ? cmp : -cmp;
    });
  });

  // Pagination
  const totalPages = computed(() =>
    Math.ceil((rows?.length || 0) / pageSize.value)
  );

  const paginatedRows = computed(() => {
    const start = (currentPage.value - 1) * pageSize.value;
    return sortedRows.value.slice(start, start + pageSize.value);
  });

  // Format cell value based on column name
  const formatCell = (value, colName) => {
    if (value == null) return '-';

    // Auto-detect formatting based on column name
    const lowerName = colName.toLowerCase();

    if (lowerName.includes('size') || lowerName === 'avg_size') {
      return formatBytes(value);
    }

    if (lowerName.includes('_at') || lowerName === 'created_at' || lowerName === 'last_added') {
      return formatDate(value);
    }

    if (lowerName === 'asset_id' || lowerName === '_topic') {
      // Truncate long hashes
      if (typeof value === 'string' && value.length > 16) {
        return value.slice(0, 16) + '...';
      }
    }

    return String(value);
  };

  if (!rows || rows.length === 0) {
    return (
      <div class="empty-state">
        <div class="empty-state-text">No results</div>
      </div>
    );
  }

  return (
    <div>
      <table class="data-table">
        <thead>
          <tr>
            {columns.map(col => (
              <th
                key={col}
                class={sortColumn.value === col ? 'sorted' : ''}
                onClick={() => setSorting(col)}
              >
                {col}
                {sortColumn.value === col && (
                  <span> {sortDirection.value === 'asc' ? '↑' : '↓'}</span>
                )}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {paginatedRows.value.map((row, i) => (
            <tr key={i}>
              {row.map((cell, j) => (
                <td key={j}>{formatCell(cell, columns[j])}</td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>

      {/* Pagination */}
      {totalPages.value > 1 && (
        <div class="pagination">
          <Button
            variant="ghost"
            disabled={currentPage.value === 1}
            onClick={() => currentPage.value = currentPage.value - 1}
          >
            ← Prev
          </Button>
          <span class="pagination-info">
            Page {currentPage.value} of {totalPages.value}
          </span>
          <Button
            variant="ghost"
            disabled={currentPage.value === totalPages.value}
            onClick={() => currentPage.value = currentPage.value + 1}
          >
            Next →
          </Button>
        </div>
      )}
    </div>
  );
}
```

### 8.5 `web-src/src/components/dashboard/QueryForm.jsx`

```jsx
import { selectedPreset, queryParams, setQueryParam } from '@store/query';

export function QueryForm() {
  const preset = selectedPreset.value;
  if (!preset || !preset.params || preset.params.length === 0) {
    return null;
  }

  // Infer input type from default value
  const inferType = (param) => {
    const defaultVal = param.default || '';

    // Check if default is a number
    if (/^\d+$/.test(defaultVal)) {
      return 'number';
    }

    // Check for date-like patterns
    if (/^\d{4}-\d{2}-\d{2}/.test(defaultVal)) {
      return 'date';
    }

    return 'text';
  };

  return (
    <div class="query-params">
      {preset.params.map(param => (
        <div key={param.name} class="query-param">
          <label>
            {param.name}
            {param.required && <span style={{ color: 'var(--terminal-red)' }}> *</span>}
          </label>
          <input
            type={inferType(param)}
            value={queryParams.value[param.name] || ''}
            placeholder={param.default || ''}
            onChange={(e) => setQueryParam(param.name, e.target.value)}
          />
        </div>
      ))}
    </div>
  );
}
```

---

## Task 9: Utility Functions

### 9.1 `web-src/src/utils/format.js`

```javascript
/**
 * Format bytes to human-readable string
 */
export function formatBytes(bytes) {
  if (bytes == null || bytes === 0) return '0 B';

  const units = ['B', 'KB', 'MB', 'GB', 'TB'];
  const k = 1024;
  const i = Math.floor(Math.log(bytes) / Math.log(k));

  return `${(bytes / Math.pow(k, i)).toFixed(i > 0 ? 1 : 0)} ${units[i]}`;
}

/**
 * Format Unix timestamp to date string
 */
export function formatDate(timestamp) {
  if (!timestamp) return '-';

  const date = new Date(timestamp * 1000);
  return date.toLocaleDateString();
}

/**
 * Format Unix timestamp to datetime string
 */
export function formatDateTime(timestamp) {
  if (!timestamp) return '-';

  const date = new Date(timestamp * 1000);
  return date.toLocaleString();
}
```

---

## Task 10: Go Static File Serving

Update Go server to serve the embedded frontend.

### 10.1 Update `internal/server/server.go`

```go
package server

import (
	"context"
	"embed"
	"io/fs"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"meshbank/internal/logger"
)

//go:embed all:../../web/dist
var webFS embed.FS

// ... existing Server struct ...

// registerRoutes sets up all API routes
func (s *Server) registerRoutes(mux *http.ServeMux) {
	// API routes
	mux.HandleFunc("/api/config", s.handleConfig)
	mux.HandleFunc("/api/topics", s.handleTopics)
	mux.HandleFunc("/api/topics/", s.handleTopicRoutes)
	mux.HandleFunc("/api/assets/", s.handleAssetRoutes)
	mux.HandleFunc("/api/logs", s.handleLogs)
	mux.HandleFunc("/api/queries", s.handleQueries)
	mux.HandleFunc("/api/query/", s.handleQueryExecution)

	// Static files (frontend)
	distFS, err := fs.Sub(webFS, "web/dist")
	if err != nil {
		s.logger.Error("Failed to setup static file server: %v", err)
		return
	}

	fileServer := http.FileServer(http.FS(distFS))

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Serve static files, but fall back to index.html for SPA routes
		path := r.URL.Path

		// Check if file exists
		if path != "/" {
			if _, err := fs.Stat(distFS, strings.TrimPrefix(path, "/")); err == nil {
				fileServer.ServeHTTP(w, r)
				return
			}
		}

		// Serve index.html for SPA routes
		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
	})
}
```

---

## Verification Checklist

After completing Phase 6, verify:

1. **Setup & Build:**
   - [ ] `make setup` creates symlinks correctly
   - [ ] `make install` installs dependencies
   - [ ] `make dev` starts dev server on port 5173
   - [ ] `make build` outputs to `../web/dist/`
   - [ ] Go binary embeds and serves frontend

2. **Setup Flow:**
   - [ ] Fresh install shows setup page
   - [ ] Can set working directory
   - [ ] Redirects to dashboard after setup
   - [ ] Invalid path shows error

3. **Dashboard:**
   - [ ] Lists all topics with stats
   - [ ] Shows health status (green/red indicator)
   - [ ] Can create new topic via modal
   - [ ] Topic name validation works
   - [ ] Clicking topic navigates to detail

4. **Topic Detail:**
   - [ ] Displays topic stats
   - [ ] Can drag-drop multiple files
   - [ ] Files appear in queue before upload
   - [ ] Can remove files from queue
   - [ ] Start Upload button works
   - [ ] Cancel button stops upload
   - [ ] Progress updates in real-time
   - [ ] Errors displayed clearly
   - [ ] Skipped duplicates shown

5. **Query Runner:**
   - [ ] Lists all presets from queries.yaml
   - [ ] Dynamic params render correctly
   - [ ] Type inference for params works
   - [ ] Results display in sortable table
   - [ ] Export JSON works
   - [ ] Pagination works

6. **Logs Page:**
   - [ ] Shows "No logs" empty state
   - [ ] TODO: Full implementation in Phase 8

7. **Navigation:**
   - [ ] History API routing works
   - [ ] Browser back/forward works
   - [ ] Direct URL access works
   - [ ] Links update URL correctly

8. **Desktop:**
   - [ ] Works on 1024px+ screens
   - [ ] No horizontal scroll

---

## Files to Create

| File | Description |
|------|-------------|
| `web-src/setup.sh` | Symlink setup script |
| `web-src/Makefile` | Build commands |
| `web-src/package.json` | npm dependencies |
| `web-src/vite.config.js` | Vite configuration |
| `web-src/index.html` | HTML entry point |
| `web-src/src/main.jsx` | App entry point |
| `web-src/src/App.jsx` | Root component |
| `web-src/src/Router.jsx` | SPA router |
| `web-src/src/dashboard.css` | Dashboard styles |
| `web-src/src/store/*.js` | State management |
| `web-src/src/services/api.js` | API client |
| `web-src/src/pages/*.jsx` | Page components |
| `web-src/src/components/dashboard/*.jsx` | Dashboard components |
| `web-src/src/utils/format.js` | Formatting utilities |
| `internal/server/server.go` | Update for static serving |

---

## TODOs for Future Phases

- [ ] **Phase 8:** Implement real log storage backend for `/api/logs`
- [ ] **Phase 8:** Add topic health repair/re-check actions
- [ ] Consider WebSocket for real-time upload progress (if sequential HTTP becomes too slow)

---

## Notes for Agent

- **Symlinks:** Always run `make setup` first on a fresh clone
- **Development:** Use `make dev` with Go backend running on port 2369
- **Production:** `make build` then rebuild Go binary to embed new frontend
- **Browser support:** Modern browsers only (Chrome, Firefox, Safari, Edge latest 2 versions)
- **No mobile:** Desktop/laptop only (1024px+ screens)
- **Design system:** All styles come from meshview via symlinks - do not duplicate
- **Icons:** Use `<Icon name="..." />` from meshview's Icon component
- **Toast:** Use `showToast(message)` from store/ui for notifications
- **Errors:** Use `<ErrorBanner message={...} />` for inline errors
