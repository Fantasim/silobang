import { getStoredToken, appendAuthToken, notifyUnauthorized } from '@services/token';
import { AUTH_ERROR_CODES } from '@constants/auth';

const API_BASE = '/api';

export class ApiError extends Error {
  constructor(message, code, status) {
    super(message);
    this.code = code;
    this.status = status;
  }
}

async function request(endpoint, options = {}) {
  const url = `${API_BASE}${endpoint}`;

  const headers = {
    'Content-Type': 'application/json',
    ...options.headers,
  };

  const token = getStoredToken();
  if (token) {
    headers['Authorization'] = `Bearer ${token}`;
  }

  const response = await fetch(url, {
    ...options,
    headers,
  });

  const data = await response.json();

  if (data.error) {
    // Global 401 handling — redirect to login for expired/invalid sessions,
    // but NOT for login failures (AUTH_INVALID_CREDENTIALS is a form error)
    if (response.status === 401 && data.code !== AUTH_ERROR_CODES.AUTH_INVALID_CREDENTIALS) {
      notifyUnauthorized();
    }
    throw new ApiError(data.message, data.code, response.status);
  }

  return data;
}

export const api = {
  // =========================================================================
  // AUTH
  // =========================================================================

  async getAuthStatus() {
    return request('/auth/status');
  },

  async login(username, password) {
    return request('/auth/login', {
      method: 'POST',
      body: JSON.stringify({ username, password }),
    });
  },

  async logout() {
    return request('/auth/logout', { method: 'POST' });
  },

  async getAuthMe() {
    return request('/auth/me');
  },

  async getAuthMeQuota() {
    return request('/auth/me/quota');
  },

  // =========================================================================
  // USER MANAGEMENT (requires manage_users grant)
  // =========================================================================

  async getUsers() {
    return request('/auth/users');
  },

  async createUser(username, displayName, password) {
    return request('/auth/users', {
      method: 'POST',
      body: JSON.stringify({ username, display_name: displayName, password }),
    });
  },

  async getUser(userId) {
    return request(`/auth/users/${userId}`);
  },

  async updateUser(userId, updates) {
    return request(`/auth/users/${userId}`, {
      method: 'PATCH',
      body: JSON.stringify(updates),
    });
  },

  async regenerateAPIKey(userId) {
    return request(`/auth/users/${userId}/api-key`, { method: 'POST' });
  },

  async getUserGrants(userId) {
    return request(`/auth/users/${userId}/grants`);
  },

  async createGrant(userId, action, constraintsJson = null) {
    return request(`/auth/users/${userId}/grants`, {
      method: 'POST',
      body: JSON.stringify({ action, constraints_json: constraintsJson }),
    });
  },

  async updateGrant(grantId, constraintsJson) {
    return request(`/auth/grants/${grantId}`, {
      method: 'PATCH',
      body: JSON.stringify({ constraints_json: constraintsJson }),
    });
  },

  async revokeGrant(grantId) {
    return request(`/auth/grants/${grantId}`, { method: 'DELETE' });
  },

  async getUserQuota(userId) {
    return request(`/auth/users/${userId}/quota`);
  },

  // =========================================================================
  // CONFIG
  // =========================================================================

  async getConfig() {
    return request('/config');
  },

  async setConfig(workingDirectory) {
    return request('/config', {
      method: 'POST',
      body: JSON.stringify({ working_directory: workingDirectory }),
    });
  },

  // =========================================================================
  // TOPICS
  // =========================================================================

  async getTopics() {
    return request('/topics');
  },

  async createTopic(name) {
    return request('/topics', {
      method: 'POST',
      body: JSON.stringify({ name }),
    });
  },

  // =========================================================================
  // ASSETS
  // =========================================================================

  async uploadAsset(topicName, file, parentId = null) {
    const formData = new FormData();
    formData.append('file', file);
    if (parentId) {
      formData.append('parent_id', parentId);
    }

    const headers = {};
    const token = getStoredToken();
    if (token) {
      headers['Authorization'] = `Bearer ${token}`;
    }

    const response = await fetch(`${API_BASE}/topics/${topicName}/assets`, {
      method: 'POST',
      body: formData,
      headers,
      // Don't set Content-Type - browser sets it with boundary for multipart
    });

    const data = await response.json();

    if (data.error) {
      if (response.status === 401 && data.code !== AUTH_ERROR_CODES.AUTH_INVALID_CREDENTIALS) {
        notifyUnauthorized();
      }
      throw new ApiError(data.message, data.code, response.status);
    }

    return data;
  },

  async downloadAsset(hash) {
    window.open(appendAuthToken(`${API_BASE}/assets/${hash}/download`), '_blank');
  },

  async getAssetMetadata(hash) {
    return request(`/assets/${hash}/metadata`);
  },

  async setAssetMetadata(hash, op, key, value, processor, processorVersion) {
    return request(`/assets/${hash}/metadata`, {
      method: 'POST',
      body: JSON.stringify({
        op,
        key,
        value,
        processor,
        processor_version: processorVersion,
      }),
    });
  },

  // =========================================================================
  // VERIFICATION — SSE stream
  // =========================================================================

  createVerifyStream(topics = [], checkIndex = true) {
    const params = new URLSearchParams();
    if (topics.length > 0) {
      params.set('topics', topics.join(','));
    }
    if (!checkIndex) {
      params.set('check_index', 'false');
    }
    const url = `${API_BASE}/verify?${params.toString()}`;
    return new EventSource(appendAuthToken(url));
  },

  // =========================================================================
  // QUERIES
  // =========================================================================

  async getQueries() {
    return request('/queries');
  },

  async runQuery(preset, params = {}, topics = []) {
    return request(`/query/${preset}`, {
      method: 'POST',
      body: JSON.stringify({ params, topics }),
    });
  },

  // =========================================================================
  // AUDIT LOG
  // =========================================================================

  createAuditStream(filter = '') {
    const params = filter ? `?filter=${filter}` : '';
    const url = `${API_BASE}/audit/stream${params}`;
    return new EventSource(appendAuthToken(url));
  },

  async getAuditLogs(params = {}) {
    const searchParams = new URLSearchParams();
    if (params.offset) searchParams.set('offset', params.offset);
    if (params.limit) searchParams.set('limit', params.limit);
    if (params.filter) searchParams.set('filter', params.filter);
    if (params.action) searchParams.set('action', params.action);
    if (params.username) searchParams.set('username', params.username);

    const query = searchParams.toString();
    return request(`/audit${query ? `?${query}` : ''}`);
  },

  async getAuditActions() {
    return request('/audit/actions');
  },

  // =========================================================================
  // BATCH METADATA
  // =========================================================================

  async batchMetadata(operations, processor, processorVersion) {
    return request('/metadata/batch', {
      method: 'POST',
      body: JSON.stringify({
        operations,
        processor,
        processor_version: processorVersion,
      }),
    });
  },

  async applyMetadata(queryPreset, queryParams, topics, op, key, value, processor, processorVersion) {
    return request('/metadata/apply', {
      method: 'POST',
      body: JSON.stringify({
        query_preset: queryPreset,
        query_params: queryParams,
        topics: topics,
        op,
        key,
        value,
        processor,
        processor_version: processorVersion,
      }),
    });
  },

  // =========================================================================
  // API SCHEMA & PROMPTS
  // =========================================================================

  async getSchema() {
    return request('/schema');
  },

  async getPrompts() {
    return request('/prompts');
  },

  async getPromptTemplate(name) {
    return request(`/prompts/${name}`);
  },

  // =========================================================================
  // BULK DOWNLOAD — SSE stream
  // =========================================================================

  createBulkDownloadStream(options) {
    const params = new URLSearchParams();
    params.set('mode', options.mode);

    if (options.mode === 'query') {
      if (options.preset) {
        params.set('preset', options.preset);
      }
      if (options.params) {
        params.set('params', JSON.stringify(options.params));
      }
      if (options.topics?.length) {
        params.set('topics', options.topics.join(','));
      }
    } else if (options.mode === 'ids') {
      if (options.assetIds?.length) {
        params.set('asset_ids', options.assetIds.join(','));
      }
    }

    if (options.includeMetadata) {
      params.set('include_metadata', 'true');
    }
    if (options.filenameFormat) {
      params.set('filename_format', options.filenameFormat);
    }

    const url = `${API_BASE}/download/bulk/start?${params.toString()}`;
    return new EventSource(appendAuthToken(url));
  },

  downloadBulkZip(downloadId) {
    window.open(appendAuthToken(`${API_BASE}/download/bulk/${downloadId}`), '_blank');
  },

  // =========================================================================
  // MONITORING
  // =========================================================================

  async getMonitoring() {
    return request('/monitoring');
  },

  viewLogFile(level, filename) {
    window.open(appendAuthToken(`${API_BASE}/monitoring/logs/${level}/${encodeURIComponent(filename)}`), '_blank');
  },
};
