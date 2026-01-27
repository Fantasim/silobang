import { useState, useRef } from 'preact/hooks';
import { startUpload, isUploading } from '@store/upload';

export function UploadZone({ disabled, topicName }) {
  const [isDragOver, setIsDragOver] = useState(false);
  const inputRef = useRef(null);

  const handleDragOver = (e) => {
    e.preventDefault();
    if (!disabled && !isUploading.value) setIsDragOver(true);
  };

  const handleDragLeave = () => {
    setIsDragOver(false);
  };

  const handleDrop = (e) => {
    e.preventDefault();
    setIsDragOver(false);
    if (disabled || isUploading.value) return;

    const files = e.dataTransfer.files;
    if (files.length > 0 && topicName) {
      startUpload(topicName, files);  // Immediate streaming upload
    }
  };

  const handleClick = () => {
    if (!disabled && !isUploading.value) inputRef.current?.click();
  };

  const handleFileSelect = (e) => {
    const files = e.target.files;
    if (files.length > 0 && topicName) {
      startUpload(topicName, files);  // Immediate streaming upload
    }
    // Reset input so same file can be selected again
    e.target.value = '';
  };

  const isDisabled = disabled || isUploading.value;

  return (
    <div
      class={`upload-zone ${isDragOver ? 'drag-over' : ''}`}
      onDragOver={handleDragOver}
      onDragLeave={handleDragLeave}
      onDrop={handleDrop}
      onClick={handleClick}
      style={{ opacity: isDisabled ? 0.5 : 1, cursor: isDisabled ? 'not-allowed' : 'pointer' }}
    >
      <input
        ref={inputRef}
        type="file"
        multiple
        style={{ display: 'none' }}
        onChange={handleFileSelect}
        disabled={isDisabled}
      />

      <div class="upload-zone-icon">
        {isUploading.value ? '...' : '\u2B06\uFE0F'}
      </div>
      <div class="upload-zone-text">
        {isUploading.value ? 'Uploading...' : 'Drag & drop files here'}
      </div>
      <div class="upload-zone-hint">
        {isUploading.value ? 'Please wait' : 'or click to browse'}
      </div>
    </div>
  );
}
