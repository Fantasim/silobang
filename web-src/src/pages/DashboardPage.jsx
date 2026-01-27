import { useEffect, useState, useMemo } from 'preact/hooks';
import { Button } from '@components/ui/Button';
import { Modal } from '@components/ui/Modal';
import { Spinner } from '@components/ui/Spinner';
import { ErrorBanner } from '@components/ui/ErrorBanner';
import { TopicCard } from '@components/ui';
import {
  topics,
  serviceInfo,
  topicsLoading,
  fetchTopics,
  createTopic
} from '@store/topics';
import { formatBytes } from '../utils/format';
import { createTopicModal, showToast } from '@store/ui';
import { navigate } from '../Router';
import { api } from '@services/api';
import { canManageTopics, canVerify } from '@store/auth';

function ServiceInfoBanner({ info }) {
  if (!info) return null;

  const { topics_summary, storage_summary, total_indexed_hashes, orchestrator_db_size, version } = info;

  return (
    <div class="service-info">
      {/* Column 1: Topics */}
      <div class="service-info-card">
        <div class="service-info-card-header">
          <span class="service-info-card-title">Topics</span>
          <span class={`service-info-card-status ${topics_summary?.unhealthy > 0 ? 'has-issues' : ''}`}>
            {topics_summary?.unhealthy > 0 ? `${topics_summary.unhealthy} unhealthy` : 'All healthy'}
          </span>
        </div>
        <div class="service-info-card-value service-info-card-value--count">{topics_summary?.total || 0}</div>
      </div>


      <div class="service-info-card">
        <div class="service-info-card-header">
          <span class="service-info-card-title">Total DAT</span>
        </div>
        <div class="service-info-card-value service-info-card-value--count">{storage_summary?.total_dat_files}</div>
      </div>

      {/* Column 2: Assets */}
      <div class="service-info-card">
        <div class="service-info-card-header">
          <span class="service-info-card-title">Indexed Assets</span>
        </div>
        <div class="service-info-card-value service-info-card-value--count">{(total_indexed_hashes || 0).toLocaleString()}</div>
      </div>

      {/* Column 3: Storage */}
      <div class="service-info-card">
        <div class="service-info-card-header">
          <span class="service-info-card-title">Total Storage</span>
        </div>
        <div class="service-info-card-value service-info-card-value--size">{formatBytes(storage_summary?.total_dat_size)}</div>
      </div>

      {/* Column 4: DB Size */}
      <div class="service-info-card">
        <div class="service-info-card-header">
          <span class="service-info-card-title">Database Size</span>
        </div>
        <div class="service-info-card-value service-info-card-value--size">{formatBytes(storage_summary?.total_db_size)}</div>
      </div>

      {/* Column 5: Orchestrator DB Size */}
      <div class="service-info-card">
        <div class="service-info-card-header">
          <span class="service-info-card-title">Orchestrator DB Size</span>
        </div>
        <div class="service-info-card-value service-info-card-value--size">{formatBytes(orchestrator_db_size)}</div>
      </div>

    </div>
  );
}

export function DashboardPage() {
  const [newTopicName, setNewTopicName] = useState('');
  const [createError, setCreateError] = useState(null);
  const [searchQuery, setSearchQuery] = useState('');
  const [filterStatus, setFilterStatus] = useState('all'); // 'all', 'healthy', 'unhealthy'

  // Verify state
  const [verifyModalOpen, setVerifyModalOpen] = useState(false);
  const [verifying, setVerifying] = useState(false);
  const [verifyProgress, setVerifyProgress] = useState([]);
  const [verifyComplete, setVerifyComplete] = useState(false);

  useEffect(() => {
    fetchTopics();
  }, []);

  // Global verify handler
  const handleVerifyAll = () => {
    setVerifyModalOpen(true);
    setVerifying(true);
    setVerifyProgress([]);
    setVerifyComplete(false);

    // Start verification for all topics (empty array = all topics)
    const eventSource = api.createVerifyStream([], true);

    eventSource.onmessage = (event) => {
      try {
        const envelope = JSON.parse(event.data);
        const eventType = envelope.type;
        const data = envelope.data || {}; // Nested data object

        if (eventType === 'topic_start') {
          setVerifyProgress(prev => [...prev, {
            topic: data.topic,
            status: 'running',
            message: `Checking ${data.dat_files} DAT files...`
          }]);
        } else if (eventType === 'dat_progress') {
          setVerifyProgress(prev => prev.map(p =>
            p.topic === data.topic
              ? { ...p, message: `Verified ${data.entries_processed}/${data.total_entries} entries in ${data.dat_file}` }
              : p
          ));
        } else if (eventType === 'topic_complete') {
          // Get error message from errors array (server sends array, not single string)
          const errorMsg = data.errors?.length > 0 ? data.errors.join(', ') : 'Unknown error';

          setVerifyProgress(prev => {
            // Check if topic exists in our list
            const exists = prev.some(p => p.topic === data.topic);
            if (exists) {
              return prev.map(p =>
                p.topic === data.topic
                  ? {
                      ...p,
                      status: data.valid ? 'success' : 'error',
                      message: data.valid ? 'Verified OK' : errorMsg
                    }
                  : p
              );
            } else {
              // Add new entry for topic that completed without a start event
              return [...prev, {
                topic: data.topic,
                status: data.valid ? 'success' : 'error',
                message: data.valid ? 'Verified OK' : errorMsg
              }];
            }
          });
        } else if (eventType === 'complete') {
          eventSource.close();
          setVerifying(false);
          setVerifyComplete(true);
          // Refresh topics to get updated health status
          fetchTopics();
        } else if (eventType === 'error') {
          setVerifyProgress(prev => [...prev, {
            topic: 'System',
            status: 'error',
            message: data.message || envelope.message || 'Unknown error'
          }]);
          eventSource.close();
          setVerifying(false);
          setVerifyComplete(true);
        }
      } catch (err) {
        console.error('Failed to parse SSE:', err);
      }
    };

    eventSource.onerror = () => {
      setVerifyProgress(prev => [...prev, {
        topic: 'Connection',
        status: 'error',
        message: 'Connection lost'
      }]);
      eventSource.close();
      setVerifying(false);
      setVerifyComplete(true);
    };
  };

  const closeVerifyModal = () => {
    setVerifyModalOpen(false);
    setVerifyProgress([]);
    setVerifyComplete(false);
  };

  // Filter topics based on search and status
  const filteredTopics = useMemo(() => {
    let result = topics.value;

    // Filter by search query
    if (searchQuery.trim()) {
      const query = searchQuery.toLowerCase();
      result = result.filter(t => t.name.toLowerCase().includes(query));
    }

    // Filter by status
    if (filterStatus === 'healthy') {
      result = result.filter(t => t.healthy);
    } else if (filterStatus === 'unhealthy') {
      result = result.filter(t => !t.healthy);
    }

    return result;
  }, [topics.value, searchQuery, filterStatus]);

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

  const hasUnhealthy = topics.value.some(t => !t.healthy);

  return (
    <div>
      <div class="page-header">
        <h1 class="page-title">Topics</h1>
        <div style={{ display: 'flex', gap: 'var(--space-2)' }}>
          {canVerify.value && (
            <Button variant="ghost" onClick={handleVerifyAll}>
              Verify All
            </Button>
          )}
          {canManageTopics.value && (
            <Button onClick={createTopicModal.open}>
              + Create Topic
            </Button>
          )}
        </div>
      </div>

      <ServiceInfoBanner info={serviceInfo.value} />

      {/* Search and Filter Bar */}
      {topics.value.length > 0 && (
        <div class="topics-toolbar">
          <div class="topics-search">
            <input
              type="text"
              class="topics-search-input"
              placeholder="Search topics..."
              value={searchQuery}
              onInput={(e) => setSearchQuery(e.target.value)}
            />
          </div>
          <div class="topics-filters">
            <button
              class={`filter-chip ${filterStatus === 'all' ? 'active' : ''}`}
              onClick={() => setFilterStatus('all')}
            >
              All ({topics.value.length})
            </button>
            <button
              class={`filter-chip ${filterStatus === 'healthy' ? 'active' : ''}`}
              onClick={() => setFilterStatus('healthy')}
            >
              Healthy ({topics.value.filter(t => t.healthy).length})
            </button>
            {hasUnhealthy && (
              <button
                class={`filter-chip filter-chip-danger ${filterStatus === 'unhealthy' ? 'active' : ''}`}
                onClick={() => setFilterStatus('unhealthy')}
              >
                Unhealthy ({topics.value.filter(t => !t.healthy).length})
              </button>
            )}
          </div>
        </div>
      )}

      {topics.value.length === 0 ? (
        <div class="empty-state">
          <div class="empty-state-icon">📦</div>
          <div class="empty-state-text">No topics yet</div>
          <div class="empty-state-hint">Create a topic to start storing assets</div>
        </div>
      ) : filteredTopics.length === 0 ? (
        <div class="empty-state">
          <div class="empty-state-text">No matching topics</div>
          <div class="empty-state-hint">Try adjusting your search or filters</div>
        </div>
      ) : (
        <div class="topics-grid">
          {filteredTopics.map(topic => (
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

      {/* Verify All Modal */}
      <Modal
        isOpen={verifyModalOpen}
        onClose={verifyComplete ? closeVerifyModal : undefined}
        title="Verify All Topics"
      >
        <div class="verify-modal-content">
          {verifying && verifyProgress.length === 0 && (
            <div class="verify-modal-starting">
              <Spinner />
              <span>Starting verification...</span>
            </div>
          )}

          {verifyProgress.length > 0 && (
            <div class="verify-modal-progress">
              {verifyProgress.map((item, i) => (
                <div key={i} class={`verify-modal-item verify-modal-item-${item.status}`}>
                  <span class="verify-modal-item-topic">{item.topic}</span>
                  <span class="verify-modal-item-message">{item.message}</span>
                  <span class={`verify-modal-item-status verify-modal-item-status-${item.status}`}>
                    {item.status === 'running' && <Spinner />}
                    {item.status === 'success' && '✓'}
                    {item.status === 'error' && '✗'}
                  </span>
                </div>
              ))}
            </div>
          )}

          {verifyComplete && (
            <div class="verify-modal-footer">
              <div class="verify-modal-summary">
                {verifyProgress.filter(p => p.status === 'success').length} passed,{' '}
                {verifyProgress.filter(p => p.status === 'error').length} failed
              </div>
              <Button onClick={closeVerifyModal}>
                Close
              </Button>
            </div>
          )}
        </div>
      </Modal>
    </div>
  );
}
