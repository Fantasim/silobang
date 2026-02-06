import { useEffect, useState } from 'preact/hooks';
import { Button } from '@components/ui/Button';
import { Spinner } from '@components/ui/Spinner';
import { UploadZone } from '@components/ui';
import {
  topics,
  fetchTopics,
  getTopicByName
} from '@store/topics';
import {
  uploadQueue,
  uploadStats,
  totalStats,
  isUploading,
  parentId,
  cancelUpload,
  clearQueue,
  FileStatus,
} from '@store/upload';
import { navigate } from '../Router';
import { formatBytes, formatDateTime } from '../utils/format';
import { api } from '@services/api';
import { canUpload, canVerify, canQuery } from '@store/auth';
import {
  RECENT_FILES_PRESET,
  RECENT_FILES_DAYS,
  RECENT_FILES_LIMIT,
  TIME_SERIES_PRESET,
  TIME_SERIES_DAYS,
  SIZE_DISTRIBUTION_PRESET
} from '@constants/query';

export function TopicPage({ topicName }) {
  const [verifying, setVerifying] = useState(false);
  const [verifyProgress, setVerifyProgress] = useState(null);

  useEffect(() => {
    fetchTopics();
  }, [topicName]);

  const topic = getTopicByName(topicName);

  const handleVerify = () => {
    setVerifying(true);
    setVerifyProgress({ status: 'starting', message: 'Starting verification...' });

    const eventSource = api.createVerifyStream([topicName], true);

    eventSource.onmessage = (event) => {
      try {
        const envelope = JSON.parse(event.data);
        const eventType = envelope.type;
        const data = envelope.data || {};

        if (eventType === 'topic_start') {
          setVerifyProgress({
            status: 'running',
            message: `Checking ${data.dat_files} DAT files...`,
            topic: data.topic
          });
        } else if (eventType === 'dat_progress') {
          setVerifyProgress({
            status: 'running',
            message: `Verified ${data.entries_processed}/${data.total_entries} entries in ${data.dat_file}`,
            topic: data.topic
          });
        } else if (eventType === 'topic_complete') {
          const errorMsg = data.errors?.length > 0 ? data.errors.join(', ') : 'Unknown error';
          setVerifyProgress({
            status: data.valid ? 'success' : 'error',
            message: data.valid ? 'Topic verified successfully!' : `Verification failed: ${errorMsg}`,
            topic: data.topic
          });
        } else if (eventType === 'complete') {
          eventSource.close();
          setVerifying(false);
          // Refresh topics to get updated health status
          fetchTopics();
        } else if (eventType === 'error') {
          setVerifyProgress({
            status: 'error',
            message: data.message || envelope.message || 'Unknown error'
          });
          eventSource.close();
          setVerifying(false);
        }
      } catch (err) {
        console.error('Failed to parse SSE:', err);
      }
    };

    eventSource.onerror = () => {
      setVerifyProgress({ status: 'error', message: 'Connection lost' });
      eventSource.close();
      setVerifying(false);
    };
  };

  if (!topic) {
    return (
      <div class="empty-state">
        <div class="empty-state-text">Topic not found</div>
        <Button onClick={() => navigate('/')}>Back to Dashboard</Button>
      </div>
    );
  }

  const stats = topic.stats || {};

  return (
    <div class="topic-page">
      {/* Header */}
      <div class="page-header">
        <div class="page-header-left">
          <Button variant="ghost" onClick={() => navigate('/')}>
            Back
          </Button>
          <h1 class="page-title">{topicName}</h1>
          <span class={`topic-status-badge ${topic.healthy ? 'healthy' : 'unhealthy'}`}>
            {topic.healthy ? 'Healthy' : 'Unhealthy'}
          </span>
        </div>
        <div class="page-header-right">
          {canQuery.value && (
            <>
              <Button
                variant="ghost"
                onClick={() => navigate(
                  `/query?preset=${RECENT_FILES_PRESET}&topics=${encodeURIComponent(topicName)}&days=${RECENT_FILES_DAYS}&limit=${RECENT_FILES_LIMIT}`
                )}
              >
                Recent Files
              </Button>
              <Button
                variant="ghost"
                onClick={() => navigate(
                  `/query?preset=${TIME_SERIES_PRESET}&topics=${encodeURIComponent(topicName)}&days=${TIME_SERIES_DAYS}`
                )}
              >
                Upload History
              </Button>
              <Button
                variant="ghost"
                onClick={() => navigate(
                  `/query?preset=${SIZE_DISTRIBUTION_PRESET}&topics=${encodeURIComponent(topicName)}`
                )}
              >
                Size Distribution
              </Button>
            </>
          )}
          {canVerify.value && (
            <Button
              variant="ghost"
              onClick={handleVerify}
              disabled={verifying}
            >
              {verifying ? <Spinner /> : 'Verify Integrity'}
            </Button>
          )}
        </div>
      </div>

      {/* Error Banner for unhealthy topics */}
      {!topic.healthy && (
        <div class="topic-error-banner">
          <div class="topic-error-banner-title">Integrity Error</div>
          <div class="topic-error-banner-message">{topic.error}</div>
          <div class="topic-error-banner-hint">
            Run verification to diagnose the issue. This topic is read-only until repaired.
          </div>
        </div>
      )}

      {/* Verify Progress */}
      {verifyProgress && (
        <div class={`verify-progress verify-progress-${verifyProgress.status}`}>
          <span class="verify-progress-message">{verifyProgress.message}</span>
        </div>
      )}

      {/* Stats Grid */}
      {/*
      <div class="topic-stats-grid">
        <div class="topic-stat-card">
          <div class="topic-stat-card-label">Files</div>
          <div class="topic-stat-card-value">{stats.file_count?.toLocaleString() || '0'}</div>
        </div>
        <div class="topic-stat-card">
          <div class="topic-stat-card-label">Total Size</div>
          <div class="topic-stat-card-value topic-stat-card-value--size">{formatBytes(stats.total_size)}</div>
        </div>
        <div class="topic-stat-card">
          <div class="topic-stat-card-label">Database Size</div>
          <div class="topic-stat-card-value topic-stat-card-value--size">{formatBytes(stats.db_size)}</div>
        </div>
        <div class="topic-stat-card">
          <div class="topic-stat-card-label">DAT Size</div>
          <div class="topic-stat-card-value topic-stat-card-value--size">{formatBytes(stats.dat_size)}</div>
        </div>
        <div class="topic-stat-card">
          <div class="topic-stat-card-label">Average File Size</div>
          <div class="topic-stat-card-value topic-stat-card-value--size">{formatBytes(stats.avg_size)}</div>
        </div>
        <div class="topic-stat-card">
          <div class="topic-stat-card-label">Last Added</div>
          <div class="topic-stat-card-value topic-stat-card-value-small topic-stat-card-value--date">
            {stats.last_added ? formatDateTime(stats.last_added) : 'Never'}
          </div>
        </div>
      </div>\
      */}

      {/* Upload Section - Only for healthy topics */}
      {topic.healthy && (
        <div class="topic-section">
          <h2 class="topic-section-title">Upload Assets</h2>

          {/* Parent ID input */}
          <div class="topic-upload-options">
            <div class="topic-upload-option">
              <label class="topic-upload-option-label">
                Parent ID (optional, for lineage tracking)
              </label>
              <input
                type="text"
                class="topic-upload-option-input"
                placeholder="BLAKE3 hash of parent asset"
                value={parentId.value}
                onInput={(e) => parentId.value = e.target.value}
                disabled={isUploading.value}
              />
            </div>
          </div>

          {/* Upload zone - uploads start immediately on drop */}
          {canUpload.value && (
            <UploadZone disabled={isUploading.value} topicName={topicName} />
          )}

          {/* Upload controls */}
          {totalStats.value.total > 0 && (
            <div class="upload-progress">
              <div class="upload-stats">
                <div class="upload-stat">
                  Total: <span class="upload-stat-value">{uploadStats.value.total}</span>
                </div>
                <div class="upload-stat">
                  Added: <span class="upload-stat-value added">{totalStats.value.added}</span>
                </div>
                <div class="upload-stat">
                  Skipped: <span class="upload-stat-value skipped">{totalStats.value.skipped}</span>
                </div>
                <div class="upload-stat">
                  Errors: <span class="upload-stat-value errors">{totalStats.value.errors}</span>
                </div>
              </div>

              <div class="upload-actions">
                {isUploading.value ? (
                  <Button variant="danger" onClick={cancelUpload}>
                    Cancel Upload
                  </Button>
                ) : (
                  <Button variant="ghost" onClick={clearQueue}>
                    Clear
                  </Button>
                )}
              </div>

              {/* File queue list - limited to last 100 items */}
              <div class="file-queue">
                {uploadQueue.value.map((item, index) => (
                  <div key={index} class={`file-queue-item ${item.status}`}>
                    <span class="file-queue-item-name">{item.fileName}</span>
                    <span class="file-queue-item-status">
                      {item.status === FileStatus.UPLOADING && 'Uploading...'}
                      {item.status === FileStatus.SUCCESS && 'Done'}
                      {item.status === FileStatus.SKIPPED && 'Skipped (duplicate)'}
                      {item.status === FileStatus.ERROR && item.error}
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
