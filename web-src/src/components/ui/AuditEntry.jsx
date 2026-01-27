import { useState } from 'preact/hooks';
import {
  FileText,
  Download,
  FolderPlus,
  Search,
  Shield,
  Settings,
  Activity,
  ChevronDown,
  ChevronRight,
  User,
  Clock,
  Wifi,
  Archive,
} from 'lucide-preact';
import { AUDIT_ACTION_COLORS, TIME_THRESHOLDS } from '../../store/audit';

// Map action types to lucide icons
const ICON_MAP = {
  connected: Wifi,
  adding_topic: FolderPlus,
  adding_file: FileText,
  downloaded: Download,
  downloaded_bulk: Archive,
  downloading: Download,
  querying: Search,
  verified: Shield,
  default: Activity,
};

// Format timestamp to relative time
function formatRelativeTime(timestamp) {
  if (!timestamp) return '-';

  const now = Date.now() / 1000;
  const diff = now - timestamp;

  if (diff < TIME_THRESHOLDS.JUST_NOW) {
    return 'just now';
  }

  if (diff < TIME_THRESHOLDS.SECONDS) {
    const seconds = Math.floor(diff);
    return `${seconds}s ago`;
  }

  if (diff < TIME_THRESHOLDS.MINUTES) {
    const minutes = Math.floor(diff / 60);
    return `${minutes}m ago`;
  }

  if (diff < TIME_THRESHOLDS.HOURS) {
    const hours = Math.floor(diff / 3600);
    return `${hours}h ago`;
  }

  if (diff < TIME_THRESHOLDS.DAYS) {
    const days = Math.floor(diff / 86400);
    return `${days}d ago`;
  }

  // Fall back to date format for older entries
  return new Date(timestamp * 1000).toLocaleDateString();
}

// Format full date/time for tooltip
function formatDateTime(timestamp) {
  if (!timestamp) return '-';
  return new Date(timestamp * 1000).toLocaleString();
}

// Format action name for display
function formatActionName(action) {
  return action
    .split('_')
    .map((word) => word.charAt(0).toUpperCase() + word.slice(1))
    .join(' ');
}

// Get summary text from entry details
function getSummary(entry) {
  const details = entry.details || {};

  switch (entry.action) {
    case 'connected':
      return details.user_agent ? `${details.user_agent.substring(0, 50)}...` : 'Stream connected';
    case 'adding_topic':
      return details.topic_name || '-';
    case 'adding_file':
      return details.filename || details.hash?.substring(0, 12) || '-';
    case 'downloaded':
    case 'downloading':
      return details.filename || details.hash?.substring(0, 12) || '-';
    case 'downloaded_bulk':
      return `${details.asset_count || 0} assets`;
    case 'querying':
      return details.preset || '-';
    case 'verified':
      return `${details.topics_checked || 0} topics`;
    default:
      return '-';
  }
}

export function AuditEntry({ entry, isNew = false }) {
  const [expanded, setExpanded] = useState(false);

  const IconComponent = ICON_MAP[entry.action] || ICON_MAP.default;
  const actionColor = AUDIT_ACTION_COLORS[entry.action] || AUDIT_ACTION_COLORS.default;
  const isLive = entry._source === 'live';

  const toggleExpanded = () => setExpanded(!expanded);

  const classes = [
    'audit-entry',
    isLive && 'audit-entry-live',
    isNew && 'audit-entry-new',
    expanded && 'audit-entry-expanded',
  ]
    .filter(Boolean)
    .join(' ');

  return (
    <div class={classes}>
      {/* Main row */}
      <div class="audit-entry-main" onClick={toggleExpanded}>
        {/* Icon */}
        <div class="audit-entry-icon" style={{ color: actionColor }}>
          <IconComponent size={18} />
        </div>

        {/* Content */}
        <div class="audit-entry-content">
          <div class="audit-entry-action" style={{ color: actionColor }}>
            {formatActionName(entry.action)}
          </div>
          <div class="audit-entry-summary">{getSummary(entry)}</div>
        </div>

        {/* Metadata */}
        <div class="audit-entry-meta">
          <span class="audit-entry-username" title={entry.ip_address || ''}>
            <User size={12} />
            {entry.username || 'system'}
          </span>
          <span class="audit-entry-time" title={formatDateTime(entry.timestamp)}>
            <Clock size={12} />
            {formatRelativeTime(entry.timestamp)}
          </span>
        </div>

        {/* Expand button */}
        <button class="audit-entry-expand" type="button">
          {expanded ? <ChevronDown size={16} /> : <ChevronRight size={16} />}
        </button>
      </div>

      {/* Expanded details */}
      {expanded && entry.details && (
        <div class="audit-entry-details">
          <pre class="audit-entry-details-json">
            {JSON.stringify(entry.details, null, 2)}
          </pre>
        </div>
      )}
    </div>
  );
}
