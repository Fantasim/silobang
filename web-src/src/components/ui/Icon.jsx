/**
 * Icon Component - Lucide-based icon system
 *
 * Uses Lucide icons (https://lucide.dev) for consistent, tree-shakeable SVG icons.
 * All icons are rendered as Preact components with customizable size, color, and stroke.
 *
 * @example
 * // Basic usage
 * <Icon name="play" />
 *
 * // With custom size and color
 * <Icon name="settings" size={24} color="var(--accent)" />
 *
 * // Filled icon (currentDot)
 * <Icon name="currentDot" color="var(--accent)" />
 *
 * @module components/ui/Icon
 */

import {
  // Navigation & Control
  Play,
  Pause,
  ChevronLeft,
  ChevronRight,
  ChevronUp,
  ChevronDown,
  ArrowLeft,
  ArrowRight,
  ArrowDown,

  // Status & Validation
  Check,
  CheckCircle,
  X,
  XCircle,
  AlertTriangle,
  AlertCircle,
  Info,
  ShieldCheck,

  // Actions
  Plus,
  Search,
  Filter,
  Download,
  Trash2,
  Pencil,
  Settings,
  Clipboard,

  // Geometry & Shapes
  Box,
  Layers,
  Grid3X3,
  Triangle,
  Hexagon,
  Octagon,
  Circle,
  Minus,

  // Transform
  Move,
  Maximize,
  Minimize,
  Crosshair,
  Target,

  // Topology
  GitBranch,
  GitMerge,
  Share2,

  // Media & Content
  Image,
  ImageOff,
  Layout,
  Crop,
  Scissors,
  FolderOpen,
  PlayCircle,

  // Materials & Effects
  Droplet,
  Palette,
  Sun,
  Zap,
  EyeOff,
  Lightbulb,
  Feather,
  Package,

  // Auth & Users
  UserPlus,
  ShieldOff,
  Copy,
  LogOut,

  // Rigging & Animation
  User,
  Activity,
  Slash,
  Link,
  Unlock,
  Anchor,
  Compass,
  TrendingDown,
  Meh,
  Frown,
  FastForward,
  SkipForward,
  Repeat,
  RefreshCw,

  // Files & Export
  FileX,
  FileText,
  CloudUpload,

  // Social/External
  Github,

  // Monitoring & System
  HardDrive,
  Clock,
  Server,
  Cpu,

  // Tools
  Wrench,
  SlidersHorizontal,

  // Viewer Controls
  Bone,
  RotateCw,
  List,
  LayoutGrid,
} from 'lucide-preact';

// =============================================================================
// ICON MAPPING
// =============================================================================

/**
 * Maps icon names to Lucide components.
 * Organized by category for maintainability.
 */
const ICON_MAP = {
  // ---------------------------------------------------------------------------
  // UI Controls - Navigation, actions, status indicators
  // ---------------------------------------------------------------------------
  panelOpen: ChevronLeft,
  panelClose: ChevronRight,
  play: Play,
  pause: Pause,
  arrowLeft: ArrowLeft,
  arrowRight: ArrowRight,
  arrowDown: ArrowDown,
  chevronUp: ChevronUp,
  chevronDown: ChevronDown,
  chevronRight: ChevronRight,
  chevronLeft: ChevronLeft,
  currentDot: Circle,
  emDash: Minus,
  check: Check,
  checkmark: Check,
  checkCircle: CheckCircle,
  x: X,
  xMark: X,
  xCircle: XCircle,
  warning: AlertTriangle,
  info: Info,
  close: X,
  plus: Plus,
  clipboard: Clipboard,
  edit: Pencil,
  search: Search,
  filter: Filter,
  filterActive: Target,
  download: Download,
  trash: Trash2,

  // ---------------------------------------------------------------------------
  // Filter Presets - Model filtering in sidebar
  // ---------------------------------------------------------------------------
  filterError: XCircle,
  filterWarning: AlertTriangle,
  filterValid: CheckCircle,
  filterAnimated: Play,
  filterStatic: Box,
  filterMover: Move,
  filterHeavy: Package,
  filterLight: Feather,
  filterLargeFile: Package,
  filterHasTextures: Image,
  filterNoTextures: ImageOff,
  filterHasLights: Lightbulb,
  filterMultiMaterial: Layers,
  categoryValidation: ShieldCheck,
  categoryAnimation: PlayCircle,
  categoryComplexity: Layers,
  categoryContent: FolderOpen,

  // ---------------------------------------------------------------------------
  // Tag Presets - Model categorization tags
  // ---------------------------------------------------------------------------
  // Geometry
  alertTriangle: AlertTriangle,
  cube: Box,
  box: Box,
  layers: Layers,
  grid: Grid3X3,
  octagon: Octagon,
  hexagon: Hexagon,
  triangle: Triangle,

  // Transform
  move: Move,
  maximize: Maximize,
  minimize: Minimize,
  crosshair: Crosshair,
  target: Target,

  // Topology
  gitBranch: GitBranch,
  gitMerge: GitMerge,
  share2: Share2,

  // UV/Texture
  image: Image,
  layout: Layout,
  cropIcon: Crop,
  grid3x3: Grid3X3,
  scissors: Scissors,

  // Material
  droplet: Droplet,
  palette: Palette,
  sun: Sun,
  sunOrbit: Sun,
  zap: Zap,
  eyeOff: EyeOff,

  // Rigging/Animation
  user: User,
  activity: Activity,
  slash: Slash,
  link: Link,
  unlock: Unlock,
  anchor: Anchor,
  compass: Compass,
  trendingDown: TrendingDown,
  meh: Meh,
  frown: Frown,
  fastForward: FastForward,
  skipForward: SkipForward,
  repeat: Repeat,
  refreshCw: RefreshCw,

  // Export/File
  fileX: FileX,
  fileText: FileText,
  uploadCloud: CloudUpload,
  alertCircle: AlertCircle,

  // Tools/Pipeline
  tool: Wrench,
  wrench: Wrench,
  settings: Settings,
  sliders: SlidersHorizontal,

  // Auth & Users
  userPlus: UserPlus,
  shieldOff: ShieldOff,
  copy: Copy,
  pencil: Pencil,
  trash2: Trash2,
  logout: LogOut,

  // Monitoring & System
  hardDrive: HardDrive,
  clock: Clock,
  server: Server,
  cpu: Cpu,

  // Social/External
  github: Github,

  // ---------------------------------------------------------------------------
  // Viewer Controls - Bottom toolbar icons
  // ---------------------------------------------------------------------------
  gridIcon: Grid3X3,
  wireframeIcon: Box,
  bonesIcon: Bone,
  rotateIcon: RotateCw,
  lightsIcon: Lightbulb,
  viewList: List,
  viewGrid: LayoutGrid,
};

/**
 * Season icons rendered as emoji for day/night clock display.
 */
const SEASON_EMOJI = {
  seasonSpring: '\u{1F338}', // Cherry blossom
  seasonSummer: '\u{2600}\u{FE0F}', // Sun
  seasonAutumn: '\u{1F342}', // Fallen leaf
  seasonWinter: '\u{2744}\u{FE0F}', // Snowflake
};

// =============================================================================
// COMPONENT
// =============================================================================

/**
 * Renders a Lucide icon by name.
 *
 * @param {Object} props - Component props
 * @param {string} props.name - Icon name (must exist in ICON_MAP or SEASON_EMOJI)
 * @param {number} [props.size=16] - Icon size in pixels
 * @param {string} [props.color='currentColor'] - Stroke/fill color
 * @param {string} [props.fill='none'] - Fill color (overridden for filled icons)
 * @param {number} [props.strokeWidth=2] - Stroke width
 * @param {string} [props.className=''] - Additional CSS classes
 * @param {Object} [props.style={}] - Additional inline styles
 * @returns {import('preact').JSX.Element|null} Icon element or null if not found
 */
export function Icon({
  name,
  size = 16,
  color = 'currentColor',
  fill = 'none',
  strokeWidth = 2,
  className = '',
  style = {},
  ...rest
}) {
  // Season icons are rendered as emoji spans
  if (SEASON_EMOJI[name]) {
    return (
      <span
        className={`icon icon-emoji ${className}`.trim()}
        style={{ fontSize: size, lineHeight: 1, ...style }}
        {...rest}
      >
        {SEASON_EMOJI[name]}
      </span>
    );
  }

  const LucideIcon = ICON_MAP[name];

  if (!LucideIcon) {
    console.warn(`Icon: Unknown icon name "${name}"`);
    return null;
  }

  // currentDot is rendered filled instead of stroked
  const isFilled = name === 'currentDot';

  return (
    <LucideIcon
      size={size}
      strokeWidth={strokeWidth}
      stroke={isFilled ? 'none' : color}
      fill={isFilled ? color : fill}
      className={`icon ${className}`.trim()}
      style={style}
      {...rest}
    />
  );
}

export default Icon;
