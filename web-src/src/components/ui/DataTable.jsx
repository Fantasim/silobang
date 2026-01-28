import { useRef, useEffect, useState } from 'preact/hooks';
import { useSignal } from '@preact/signals';
import {
  sortColumn,
  sortDirection,
  setSorting,
  openAssetDrawer,
  selectedRows,
  toggleRowSelection,
  selectAllRows,
  clearSelection
} from '@store/query';
import { computed } from '@preact/signals';
import { formatBytes, formatDateTime } from '@utils/format';
import { api } from '@services/api';
import { showToast } from '@store/ui';
import { MetadataValue } from '@components/ui/MetadataValue';
import { DATA_TABLE_BATCH_SIZE, DATA_TABLE_SCROLL_THRESHOLD } from '@constants/table';

export function DataTable({ columns, rows, enableSelection = true }) {
  const scrollRef = useRef(null);
  const visibleCount = useSignal(DATA_TABLE_BATCH_SIZE);

  // Find the hash column index (asset_id, hash, or last_hash)
  const hashColIndex = columns.findIndex(col => {
    const lowerName = col.toLowerCase();
    return lowerName === 'asset_id' || lowerName === 'hash' || lowerName === 'last_hash';
  });

  // Get all hashes from current rows
  const getAllHashes = () => {
    if (hashColIndex === -1) return [];
    return rows.map(row => row[hashColIndex]).filter(h => h && isHash(h));
  };

  // Check if all visible rows are selected
  const allSelected = computed(() => {
    const hashes = getAllHashes();
    if (hashes.length === 0) return false;
    return hashes.every(h => selectedRows.value.has(h));
  });

  // Handle select all toggle
  const handleSelectAll = () => {
    const hashes = getAllHashes();
    if (allSelected.value) {
      clearSelection();
    } else {
      selectAllRows(hashes);
    }
  };

  // Reset visible count and selection when rows change
  useEffect(() => {
    visibleCount.value = DATA_TABLE_BATCH_SIZE;
    clearSelection();
  }, [rows]);

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

  // Visible rows (infinite scroll)
  const visibleRows = computed(() => {
    return sortedRows.value?.slice(0, visibleCount.value) || [];
  });

  // Handle scroll to load more
  const handleScroll = () => {
    const el = scrollRef.current;
    if (!el || !rows) return;

    // Load more when scrolled near bottom
    const nearBottom = el.scrollHeight - el.scrollTop - el.clientHeight < DATA_TABLE_SCROLL_THRESHOLD;

    if (nearBottom && visibleCount.value < rows.length) {
      visibleCount.value = Math.min(visibleCount.value + DATA_TABLE_BATCH_SIZE, rows.length);
    }
  };

  // Copy text to clipboard
  const copyToClipboard = async (text, e) => {
    e.stopPropagation();
    try {
      await navigator.clipboard.writeText(text);
      showToast('Copied to clipboard');
    } catch (err) {
      console.error('Failed to copy:', err);
    }
  };

  // Download asset
  const handleDownload = (hash, e) => {
    e.stopPropagation();
    api.downloadAsset(hash);
  };

  // Open asset drawer
  const handleViewAsset = (hash, e) => {
    e.stopPropagation();
    openAssetDrawer(hash);
  };

  // Detect if value looks like a hash (64 hex characters)
  const isHash = (value) => {
    return typeof value === 'string' && /^[a-f0-9]{64}$/i.test(value);
  };

  // Format cell value based on column name
  const formatCell = (value, colName, row) => {
    if (value == null) return '-';

    // Auto-detect formatting based on column name
    const lowerName = colName.toLowerCase();

    if ((lowerName.includes('size') || lowerName === 'avg_size') && typeof value === 'number') {
      return formatBytes(value);
    }

    if (lowerName.includes('_at') || lowerName === 'created_at' || lowerName === 'last_added') {
      return formatDateTime(value);
    }

    // Show hash with download actions (only for known downloadable columns)
    if ((lowerName === 'asset_id' || lowerName === 'hash' || lowerName === 'last_hash') && isHash(value)) {
      return (
        <div class="cell-hash-container">
          <span
            class="cell-hash"
            onClick={(e) => handleViewAsset(value, e)}
            title="Click to view asset details"
          >
            {value.substring(0, 16)}...
          </span>
          <div class="cell-hash-actions">
            <button
              class="cell-action-btn cell-action-btn-view"
              onClick={(e) => handleViewAsset(value, e)}
              title="View asset details"
            >
              View
            </button>
            <button
              class="cell-action-btn"
              onClick={(e) => copyToClipboard(value, e)}
              title="Copy full hash"
            >
              Copy
            </button>
            <button
              class="cell-action-btn cell-action-btn-primary"
              onClick={(e) => handleDownload(value, e)}
              title="Download asset"
            >
              Download
            </button>
          </div>
        </div>
      );
    }

    // Parent ID might be a hash
    if (lowerName === 'parent_id' && value) {
      return (
        <span
          class="cell-hash"
          onClick={(e) => copyToClipboard(value, e)}
          title="Click to copy"
        >
          {value.substring(0, 16)}...
        </span>
      );
    }

    // Other hash-like values: show with copy only (no download)
    if (isHash(value)) {
      return (
        <span
          class="cell-hash"
          onClick={(e) => copyToClipboard(value, e)}
          title="Click to copy"
        >
          {value.substring(0, 16)}...
        </span>
      );
    }

    // Handle metadata columns or JSON/object values
    if (lowerName.includes('metadata') || lowerName === 'metadata_json' || typeof value === 'object') {
      return <MetadataValue value={value} label={colName} />;
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

  // Check if a row is selected by its hash
  const isRowSelected = (row) => {
    if (hashColIndex === -1) return false;
    const hash = row[hashColIndex];
    return hash && selectedRows.value.has(hash);
  };

  // Handle row checkbox toggle
  const handleRowCheckbox = (row, e) => {
    e.stopPropagation();
    if (hashColIndex === -1) return;
    const hash = row[hashColIndex];
    if (hash && isHash(hash)) {
      toggleRowSelection(hash);
    }
  };

  const showCheckboxes = enableSelection && hashColIndex !== -1;

  return (
    <div class="data-table-container">
      <div class="data-table-scroll" ref={scrollRef} onScroll={handleScroll}>
        <table class="data-table">
          <thead>
            <tr>
              {showCheckboxes && (
                <th class="data-table-checkbox">
                  <input
                    type="checkbox"
                    checked={allSelected.value}
                    onChange={handleSelectAll}
                    title="Select all"
                  />
                </th>
              )}
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
            {visibleRows.value.map((row, i) => (
              <tr key={i} class={isRowSelected(row) ? 'selected' : ''}>
                {showCheckboxes && (
                  <td class="data-table-checkbox">
                    <input
                      type="checkbox"
                      checked={isRowSelected(row)}
                      onChange={(e) => handleRowCheckbox(row, e)}
                    />
                  </td>
                )}
                {row.map((cell, j) => (
                  <td key={j}>{formatCell(cell, columns[j])}</td>
                ))}
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {/* Loading indicator / count */}
      <div class="data-table-footer">
        <span class="data-table-count">
          Showing {visibleRows.value.length} of {rows.length}
        </span>
        {visibleCount.value < rows.length && (
          <span class="data-table-hint">Scroll for more</span>
        )}
      </div>
    </div>
  );
}
