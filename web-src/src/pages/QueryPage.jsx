import { useEffect, useState } from 'preact/hooks';
import { Button } from '@components/ui/Button';
import { Modal } from '@components/ui/Modal';
import { Spinner } from '@components/ui/Spinner';
import { ErrorBanner } from '@components/ui/ErrorBanner';
import { DataTable } from '@components/ui';
import { ToggleGroup } from '@components/ui/ToggleGroup';
import { PresetSelector, AssetDrawer, SelectionActionBar, BulkMetadataModal } from '@query';
import {
  presetsLoading,
  selectedPreset,
  queryParams,
  selectedTopics,
  queryResult,
  queryError,
  fetchPresets,
  selectedRows,
  initFromParams,
} from '@store/query';
import { fetchTopics } from '@store/topics';
import { api } from '@services/api';
import { formatBytes } from '../utils/format';
import { FILENAME_FORMATS, DEFAULT_FILENAME_FORMAT, DEFAULT_INCLUDE_METADATA } from '@constants/download';
import { canQuery, canBulkDownload } from '@store/auth';
import { Icon } from '@components/ui/Icon';

export function QueryPage({ initPreset, initTopics, initParams }) {
  if (!canQuery.value) {
    return (
      <div class="permission-denied">
        <Icon name="shieldOff" size={48} />
        <p class="permission-denied-message">You do not have permission to run queries.</p>
      </div>
    );
  }
  // Bulk download state
  const [bulkDownloadOpen, setBulkDownloadOpen] = useState(false);
  const [confirmPhase, setConfirmPhase] = useState(true);
  const [includeMetadata, setIncludeMetadata] = useState(DEFAULT_INCLUDE_METADATA);
  const [filenameFormat, setFilenameFormat] = useState(DEFAULT_FILENAME_FORMAT);
  const [downloading, setDownloading] = useState(false);
  const [downloadProgress, setDownloadProgress] = useState([]);
  const [downloadComplete, setDownloadComplete] = useState(false);
  const [downloadResult, setDownloadResult] = useState(null);

  // Bulk metadata modal state
  const [bulkMetadataOpen, setBulkMetadataOpen] = useState(false);

  // Apply-all metadata modal state
  const [applyAllMetadataOpen, setApplyAllMetadataOpen] = useState(false);

  useEffect(() => {
    if (initPreset) {
      // Auto-configure and run from URL params (e.g., topic page "Recent Files" button)
      fetchTopics();
      initFromParams(initPreset, initTopics, initParams);
    } else {
      fetchPresets();
      fetchTopics();
    }
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

  const handleExportCsv = () => {
    if (!queryResult.value) return;

    const { columns, rows } = queryResult.value;

    // Escape CSV fields
    const escapeField = (val) => {
      if (val == null) return '';
      const str = String(val);
      if (str.includes(',') || str.includes('"') || str.includes('\n')) {
        return `"${str.replace(/"/g, '""')}"`;
      }
      return str;
    };

    // Build CSV
    const header = columns.map(escapeField).join(',');
    const body = rows.map(row => row.map(escapeField).join(',')).join('\n');
    const csv = `${header}\n${body}`;

    const blob = new Blob([csv], { type: 'text/csv' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `${selectedPreset.value?.name || 'query'}-results.csv`;
    a.click();
    URL.revokeObjectURL(url);
  };

  // Open bulk download confirmation modal
  const handleBulkDownload = () => {
    if (!selectedPreset.value) return;
    setBulkDownloadOpen(true);
    setConfirmPhase(true);
    setIncludeMetadata(DEFAULT_INCLUDE_METADATA);
    setFilenameFormat(DEFAULT_FILENAME_FORMAT);
    setDownloading(false);
    setDownloadProgress([]);
    setDownloadComplete(false);
    setDownloadResult(null);
  };

  // Start the actual bulk download after confirmation
  const startBulkDownload = () => {
    setConfirmPhase(false);
    setDownloading(true);

    const eventSource = api.createBulkDownloadStream({
      mode: 'query',
      preset: selectedPreset.value.name,
      params: queryParams.value,
      topics: selectedTopics.value,
      includeMetadata,
      filenameFormat,
    });

    eventSource.onmessage = (event) => {
      try {
        const envelope = JSON.parse(event.data);
        const eventType = envelope.type;
        const data = envelope.data || {};

        if (eventType === 'download_start') {
          setDownloadProgress([{
            label: 'Preparing download',
            status: 'running',
            message: `${data.total_assets} assets (${formatBytes(data.total_bytes)})`
          }]);
        } else if (eventType === 'asset_progress') {
          setDownloadProgress(prev => [{
            ...prev[0],
            message: `Processing ${data.asset_index}/${data.total_assets}: ${data.filename || data.hash?.substring(0, 8)}`
          }]);
        } else if (eventType === 'zip_progress') {
          setDownloadProgress(prev => [{
            ...prev[0],
            message: `Building ZIP: ${data.percent_complete}%`
          }]);
        } else if (eventType === 'complete') {
          setDownloadProgress([{
            label: 'Download ready',
            status: 'success',
            message: `${data.total_assets} assets (${formatBytes(data.total_size)})`
          }]);
          setDownloadResult(data);
          setDownloading(false);
          setDownloadComplete(true);
          eventSource.close();
        } else if (eventType === 'error') {
          setDownloadProgress(prev => [...prev, {
            label: 'Error',
            status: 'error',
            message: data.message || 'Unknown error'
          }]);
          setDownloading(false);
          setDownloadComplete(true);
          eventSource.close();
        }
      } catch (err) {
        console.error('Failed to parse SSE:', err);
      }
    };

    eventSource.onerror = () => {
      setDownloadProgress(prev => [...prev, {
        label: 'Connection',
        status: 'error',
        message: 'Connection lost'
      }]);
      eventSource.close();
      setDownloading(false);
      setDownloadComplete(true);
    };
  };

  const closeBulkDownloadModal = () => {
    setBulkDownloadOpen(false);
    setDownloadProgress([]);
    setDownloadComplete(false);
    setDownloadResult(null);
  };

  const handleDownloadZip = () => {
    if (downloadResult?.download_url) {
      // Extract download ID from URL (e.g., "/api/download/bulk/abc123" -> "abc123")
      const downloadId = downloadResult.download_url.split('/').pop();
      api.downloadBulkZip(downloadId);
    }
  };

  return (
    <div class="query-page">
      {presetsLoading.value ? (
        <Spinner />
      ) : (
        <>
          {/* Preset selector with integrated params and run button */}
          <PresetSelector hasResults={!!queryResult.value} />

          {/* Error */}
          {queryError.value && (
            <ErrorBanner message={queryError.value} />
          )}

          {/* Results */}
          {queryResult.value && (
            <div class="query-results-v2">
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 'var(--space-3)', flexShrink: 0 }}>
                <span style={{ color: 'var(--text-secondary)', fontSize: 'var(--font-sm)' }}>
                  {queryResult.value.row_count} row{queryResult.value.row_count !== 1 ? 's' : ''}
                </span>
                <div style={{ display: 'flex', gap: 'var(--space-2)' }}>
                  {canBulkDownload.value && queryResult.value.columns.includes('asset_id') && (
                    <Button variant="ghost" onClick={handleBulkDownload}>
                      Bulk Download
                    </Button>
                  )}
                  <Button variant="ghost" onClick={handleExportCsv}>
                    Export CSV
                  </Button>
                  <Button variant="ghost" onClick={handleExportJson}>
                    Export JSON
                  </Button>
                </div>
              </div>

              <DataTable
                columns={queryResult.value.columns}
                rows={queryResult.value.rows}
              />
            </div>
          )}
        </>
      )}

      {/* Bulk Download Modal */}
      <Modal
        isOpen={bulkDownloadOpen}
        onClose={confirmPhase || downloadComplete ? closeBulkDownloadModal : undefined}
        title="Bulk Download"
      >
        {confirmPhase ? (
          <div class="bulk-download-confirm">
            <div class="bulk-download-confirm-summary">
              Download {queryResult.value?.row_count || 0} asset{queryResult.value?.row_count !== 1 ? 's' : ''} from preset "{selectedPreset.value?.name}"
            </div>

            <div class="bulk-download-confirm-options">
              <label class="bulk-download-confirm-option">
                <input
                  type="checkbox"
                  checked={includeMetadata}
                  onChange={(e) => setIncludeMetadata(e.target.checked)}
                />
                <span>Include metadata</span>
              </label>

              <div class="bulk-download-confirm-option">
                <span class="bulk-download-confirm-label">Filename format</span>
                <ToggleGroup
                  options={FILENAME_FORMATS}
                  activeId={filenameFormat}
                  onToggle={setFilenameFormat}
                  compact
                />
              </div>
            </div>

            <div class="bulk-download-confirm-footer">
              <Button onClick={startBulkDownload}>
                Start Download
              </Button>
              <Button variant="ghost" onClick={closeBulkDownloadModal}>
                Cancel
              </Button>
            </div>
          </div>
        ) : (
          <div class="verify-modal-content">
            {downloading && downloadProgress.length === 0 && (
              <div class="verify-modal-starting">
                <Spinner />
                <span>Starting download...</span>
              </div>
            )}

            {downloadProgress.length > 0 && (
              <div class="verify-modal-progress">
                {downloadProgress.map((item, i) => (
                  <div key={i} class={`verify-modal-item verify-modal-item-${item.status}`}>
                    <span class="verify-modal-item-topic">{item.label}</span>
                    <span class="verify-modal-item-message">{item.message}</span>
                    <span class={`verify-modal-item-status verify-modal-item-status-${item.status}`}>
                      {item.status === 'running' && <Spinner />}
                      {item.status === 'success' && '\u2713'}
                      {item.status === 'error' && '\u2717'}
                    </span>
                  </div>
                ))}
              </div>
            )}

            {downloadComplete && (
              <div class="verify-modal-footer">
                {downloadResult && (
                  <div class="verify-modal-summary">
                    {downloadResult.total_assets} assets ready
                    {downloadResult.failed_assets > 0 && ` (${downloadResult.failed_assets} failed)`}
                  </div>
                )}
                <div style={{ display: 'flex', gap: 'var(--space-2)' }}>
                  {downloadResult && (
                    <Button onClick={handleDownloadZip}>
                      Download ZIP
                    </Button>
                  )}
                  <Button variant="ghost" onClick={closeBulkDownloadModal}>
                    Close
                  </Button>
                </div>
              </div>
            )}
          </div>
        )}
      </Modal>

      {/* Selection Action Bar */}
      {selectedRows.value.size > 0 && (
        <SelectionActionBar onSetMetadata={() => setBulkMetadataOpen(true)} />
      )}

      {/* Bulk Metadata Modal (selected rows) */}
      <BulkMetadataModal
        isOpen={bulkMetadataOpen}
        onClose={() => setBulkMetadataOpen(false)}
        mode="selected"
      />

      {/* Apply-All Metadata Modal (all query results) */}
      <BulkMetadataModal
        isOpen={applyAllMetadataOpen}
        onClose={() => setApplyAllMetadataOpen(false)}
        mode="apply-all"
        totalCount={queryResult.value?.row_count || 0}
      />

      {/* Asset Drawer */}
      <AssetDrawer />
    </div>
  );
}
