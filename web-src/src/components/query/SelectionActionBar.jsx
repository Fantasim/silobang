import { Button } from '@components/ui/Button';
import { selectedRows, clearSelection, getSelectedHashes } from '@store/query';
import { canMetadata } from '@store/auth';

export function SelectionActionBar({ onSetMetadata }) {
  const count = selectedRows.value.size;

  if (count === 0) return null;

  return (
    <div class="selection-action-bar">
      <span class="selection-action-bar-count">
        {count} asset{count !== 1 ? 's' : ''} selected
      </span>
      <div class="selection-action-bar-actions">
        {canMetadata.value && (
          <Button onClick={onSetMetadata}>
            Set Metadata
          </Button>
        )}
        <Button variant="ghost" onClick={clearSelection}>
          Clear
        </Button>
      </div>
    </div>
  );
}
