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
 * Format Unix timestamp to datetime string (YYYY-MM-DD HH:mm:ss)
 */
export function formatDateTime(timestamp) {
  if (!timestamp) return '-';

  const date = new Date(timestamp * 1000);
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, '0');
  const day = String(date.getDate()).padStart(2, '0');
  const hours = String(date.getHours()).padStart(2, '0');
  const minutes = String(date.getMinutes()).padStart(2, '0');
  const seconds = String(date.getSeconds()).padStart(2, '0');

  return `${year}-${month}-${day} ${hours}:${minutes}:${seconds}`;
}

/**
 * Format seconds into a human-readable uptime string (e.g. "2d 3h 15m")
 */
export function formatUptime(totalSeconds) {
  if (totalSeconds == null || totalSeconds < 0) return '-';

  const days = Math.floor(totalSeconds / 86400);
  const hours = Math.floor((totalSeconds % 86400) / 3600);
  const minutes = Math.floor((totalSeconds % 3600) / 60);

  const parts = [];
  if (days > 0) parts.push(`${days}d`);
  if (hours > 0) parts.push(`${hours}h`);
  parts.push(`${minutes}m`);

  return parts.join(' ');
}

/**
 * Format nanoseconds to a human-readable string (e.g. "1.2ms", "450ns")
 */
export function formatNanoseconds(ns) {
  if (ns == null) return '-';
  if (ns === 0) return '0ns';

  if (ns < 1000) return `${ns}ns`;
  if (ns < 1000000) return `${(ns / 1000).toFixed(1)}us`;
  if (ns < 1000000000) return `${(ns / 1000000).toFixed(1)}ms`;
  return `${(ns / 1000000000).toFixed(2)}s`;
}
