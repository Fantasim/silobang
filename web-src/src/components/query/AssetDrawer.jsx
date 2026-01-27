import { useEffect, useState, useRef } from 'preact/hooks';
import { Button } from '@components/ui/Button';
import { Spinner } from '@components/ui/Spinner';
import { MetadataValue } from '@components/ui/MetadataValue';
import { api } from '@services/api';
import { showToast } from '@store/ui';
import { assetDrawerOpen, assetDrawerHash, closeAssetDrawer } from '@store/query';
import { formatDateTime, formatBytes } from '../../utils/format';
import { TOAST_TYPES } from '@constants/ui';
import { DEFAULT_METADATA_PROCESSOR, DEFAULT_METADATA_PROCESSOR_VERSION } from '@constants/metadata';

export function AssetDrawer() {
  const [loading, setLoading] = useState(false);
  const [assetData, setAssetData] = useState(null);
  const [error, setError] = useState(null);

  // Add metadata form state
  const [newKey, setNewKey] = useState('');
  const newValueRef = useRef(null);
  const [processor, setProcessor] = useState(DEFAULT_METADATA_PROCESSOR);
  const [processorVersion, setProcessorVersion] = useState(DEFAULT_METADATA_PROCESSOR_VERSION);
  const [showAdvanced, setShowAdvanced] = useState(false);
  const [submitting, setSubmitting] = useState(false);

  const isOpen = assetDrawerOpen.value;
  const hash = assetDrawerHash.value;

  // Fetch asset metadata when drawer opens
  useEffect(() => {
    if (isOpen && hash) {
      fetchAssetData();
    } else {
      setAssetData(null);
      setError(null);
      setNewKey('');
      if (newValueRef.current) {
        newValueRef.current.value = '';
      }
      setProcessor(DEFAULT_METADATA_PROCESSOR);
      setProcessorVersion(DEFAULT_METADATA_PROCESSOR_VERSION);
      setShowAdvanced(false);
    }
  }, [isOpen, hash]);

  const fetchAssetData = async () => {
    setLoading(true);
    setError(null);
    try {
      const data = await api.getAssetMetadata(hash);
      setAssetData(data);
    } catch (err) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  };

  const handleAddMetadata = async (e) => {
    e.preventDefault();
    if (!newKey.trim()) {
      showToast('Key is required', TOAST_TYPES.warning);
      return;
    }

    setSubmitting(true);
    try {
      await api.setAssetMetadata(
        hash,
        'set',
        newKey.trim(),
        newValueRef.current?.value || '',
        processor,
        processorVersion
      );
      showToast('Metadata added', TOAST_TYPES.success);
      setNewKey('');
      if (newValueRef.current) {
        newValueRef.current.value = '';
      }
      fetchAssetData(); // Refresh data
    } catch (err) {
      showToast(`Failed to add metadata: ${err.message}`, TOAST_TYPES.error);
    } finally {
      setSubmitting(false);
    }
  };

  const handleDeleteMetadata = async (key) => {
    try {
      await api.setAssetMetadata(
        hash,
        'delete',
        key,
        null,
        processor,
        processorVersion
      );
      showToast('Metadata deleted', TOAST_TYPES.success);
      fetchAssetData(); // Refresh data
    } catch (err) {
      showToast(`Failed to delete metadata: ${err.message}`, TOAST_TYPES.error);
    }
  };

  const handleDownload = () => {
    api.downloadAsset(hash);
  };

  const copyToClipboard = async (text) => {
    try {
      await navigator.clipboard.writeText(text);
      showToast('Copied to clipboard', TOAST_TYPES.info);
    } catch (err) {
      showToast('Failed to copy', TOAST_TYPES.error);
    }
  };

  const handleOverlayClick = (e) => {
    if (e.target.classList.contains('asset-drawer-overlay')) {
      closeAssetDrawer();
    }
  };

  if (!isOpen) return null;

  const asset = assetData?.asset;
  const metadataWithProcessor = assetData?.metadata_with_processor || [];
  // Sort by key for consistent display
  const sortedMetadata = [...metadataWithProcessor].sort((a, b) => a.key.localeCompare(b.key));

  return (
    <div class="asset-drawer-overlay" onClick={handleOverlayClick}>
      <div class="asset-drawer">
        {/* Header */}
        <div class="asset-drawer-header">
          <h2 class="asset-drawer-title">Asset Details</h2>
          <button class="asset-drawer-close" onClick={closeAssetDrawer}>
            &times;
          </button>
        </div>

        {/* Content */}
        <div class="asset-drawer-content">
          {loading && (
            <div class="asset-drawer-loading">
              <Spinner />
              <span>Loading asset data...</span>
            </div>
          )}

          {error && (
            <div class="asset-drawer-error">
              <span>Error: {error}</span>
              <Button variant="ghost" onClick={fetchAssetData}>Retry</Button>
            </div>
          )}

          {!loading && !error && asset && (
            <>
              {/* Asset Info Section */}
              <div class="asset-drawer-section">
                <h3 class="asset-drawer-section-title">Asset Information</h3>

                <div class="asset-info-grid">
                  <div class="asset-info-row">
                    <span class="asset-info-label">Hash</span>
                    <span
                      class="asset-info-value asset-info-hash"
                      onClick={() => copyToClipboard(hash)}
                      title="Click to copy full hash"
                    >
                      {hash}
                    </span>
                  </div>

                  <div class="asset-info-row">
                    <span class="asset-info-label">Original Name</span>
                    <span class="asset-info-value">{asset.origin_name || '-'}</span>
                  </div>

                  <div class="asset-info-row">
                    <span class="asset-info-label">Extension</span>
                    <span class="asset-info-value">{asset.extension || '-'}</span>
                  </div>

                  <div class="asset-info-row">
                    <span class="asset-info-label">Size</span>
                    <span class="asset-info-value">{formatBytes(asset.size)}</span>
                  </div>

                  <div class="asset-info-row">
                    <span class="asset-info-label">Created At</span>
                    <span class="asset-info-value">{formatDateTime(asset.created_at)}</span>
                  </div>

                  {asset.parent_id && (
                    <div class="asset-info-row">
                      <span class="asset-info-label">Parent ID</span>
                      <span
                        class="asset-info-value asset-info-hash"
                        onClick={() => copyToClipboard(asset.parent_id)}
                        title="Click to copy parent hash"
                      >
                        {asset.parent_id.substring(0, 16)}...
                      </span>
                    </div>
                  )}

                  {asset.topic && (
                    <div class="asset-info-row">
                      <span class="asset-info-label">Topic</span>
                      <span class="asset-info-value">{asset.topic}</span>
                    </div>
                  )}
                </div>

                <div class="asset-info-actions">
                  <Button onClick={handleDownload}>
                    Download Asset
                  </Button>
                  <Button variant="ghost" onClick={() => copyToClipboard(hash)}>
                    Copy Hash
                  </Button>
                </div>
              </div>

              {/* Metadata Section */}
              <div class="asset-drawer-section">
                <h3 class="asset-drawer-section-title">Computed Metadata</h3>

                {sortedMetadata.length === 0 ? (
                  <div class="asset-metadata-empty">
                    No metadata set for this asset
                  </div>
                ) : (
                  <div class="asset-metadata-list">
                    {sortedMetadata.map(item => (
                      <div
                        key={item.key}
                        class="asset-metadata-item"
                        title={`${item.processor} v${item.processor_version}`}
                      >
                        <span class="asset-metadata-key">{item.key}</span>
                        <MetadataValue
                          value={item.value}
                          label={item.key}
                          className="asset-metadata-value"
                        />
                        <button
                          class="asset-metadata-delete"
                          onClick={() => handleDeleteMetadata(item.key)}
                          title="Delete this metadata"
                        >
                          &times;
                        </button>
                      </div>
                    ))}
                  </div>
                )}

                {/* Add Metadata Form */}
                <form class="asset-metadata-form" onSubmit={handleAddMetadata}>
                  <h4 class="asset-metadata-form-title">Add Metadata</h4>
                  <div class="asset-metadata-form-inputs">
                    <input
                      type="text"
                      class="asset-metadata-input"
                      placeholder="Key"
                      value={newKey}
                      onInput={(e) => setNewKey(e.target.value)}
                      disabled={submitting}
                    />
                    <textarea
                      ref={newValueRef}
                      class="asset-metadata-input"
                      placeholder="Value"
                      disabled={submitting}
                      rows={1}
                    />
                    <Button type="submit" disabled={submitting || !newKey.trim()}>
                      {submitting ? 'Adding...' : 'Add'}
                    </Button>
                  </div>

                  <div
                    class="bulk-metadata-advanced-toggle"
                    onClick={() => setShowAdvanced(!showAdvanced)}
                  >
                    {showAdvanced ? '▼' : '▶'} Advanced
                  </div>

                  {showAdvanced && (
                    <div class="bulk-metadata-advanced">
                      <div class="bulk-metadata-field">
                        <label htmlFor="drawer-processor">Processor</label>
                        <input
                          id="drawer-processor"
                          type="text"
                          placeholder="Processor name"
                          value={processor}
                          onInput={(e) => setProcessor(e.target.value)}
                          disabled={submitting}
                        />
                      </div>
                      <div class="bulk-metadata-field">
                        <label htmlFor="drawer-processor-version">Version</label>
                        <input
                          id="drawer-processor-version"
                          type="text"
                          placeholder="Version"
                          value={processorVersion}
                          onInput={(e) => setProcessorVersion(e.target.value)}
                          disabled={submitting}
                        />
                      </div>
                    </div>
                  )}
                </form>
              </div>
            </>
          )}
        </div>
      </div>
    </div>
  );
}
