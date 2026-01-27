import { Icon } from './Icon';
import { SimpleTooltip } from './SmartTooltip';
import { showToast } from '@store/ui';
import { ICON_BUTTON_SIZES } from '@constants/ui';

/**
 * IconButton - Unified icon button for toolbars with full customization.
 *
 * @param {string} icon - Icon name from ICONS constant (or path data if starts with 'M')
 * @param {string} emoji - Optional emoji/character to render instead of SVG icon
 * @param {boolean} active - Whether button is active
 * @param {Function} onClick - Click handler
 * @param {string} size - Button size: 'large' | 'medium' | 'small'
 * @param {string} tooltip - Tooltip description text
 * @param {string} tooltipPosition - Tooltip position: 'top' | 'bottom' | 'left' | 'right'
 * @param {string} toastOn - Toast message when activated
 * @param {string} toastOff - Toast message when deactivated
 * @param {string} borderColor - Custom border color (CSS variable or hex)
 * @param {string} iconColor - Custom icon/text color (CSS variable or hex)
 * @param {string} activeColor - Custom active state color (CSS variable or hex)
 * @param {string} hoverColor - Custom hover color (CSS variable or hex)
 * @param {boolean} dashed - Whether border is dashed (for modal-opening actions)
 * @param {string} className - Additional CSS classes
 */
export function IconButton({
  icon,
  emoji,
  active = false,
  onClick,
  size = 'large',
  tooltip,
  tooltipPosition = 'top',
  toastOn,
  toastOff,
  borderColor,
  iconColor,
  activeColor,
  hoverColor,
  dashed = false,
  className = '',
}) {
  // Determine if icon is a path (starts with 'M') or a name
  const isPath = icon && icon.startsWith('M');

  const handleClick = (e) => {
    if (onClick) {
      onClick(e);
    }
    // Show toast based on new state (after toggle)
    if (toastOn || toastOff) {
      const willBeActive = !active;
      const message = willBeActive ? toastOn : toastOff;
      if (message) {
        showToast(message);
      }
    }
  };

  const sizeConfig = ICON_BUTTON_SIZES[size] || ICON_BUTTON_SIZES.large;

  const classes = [
    'icon-btn',
    size !== 'large' && `icon-btn-${size}`,
    active && 'active',
    dashed && 'icon-btn-dashed',
    className,
  ]
    .filter(Boolean)
    .join(' ');

  // Build custom style object for color overrides
  const customStyle = {};
  if (borderColor) customStyle['--ib-border'] = borderColor;
  if (iconColor) customStyle['--ib-color'] = iconColor;
  if (activeColor) customStyle['--ib-active'] = activeColor;
  if (hoverColor) customStyle['--ib-hover'] = hoverColor;

  const button = (
    <button
      class={classes}
      onClick={handleClick}
      style={Object.keys(customStyle).length > 0 ? customStyle : undefined}
    >
      {emoji ? (
        <span class="icon-btn-emoji" aria-hidden="true">{emoji}</span>
      ) : (
        <Icon
          name={isPath ? undefined : icon}
          path={isPath ? icon : undefined}
          size={sizeConfig.icon}
        />
      )}
    </button>
  );

  // Wrap with tooltip if provided
  if (tooltip) {
    return (
      <SimpleTooltip text={tooltip} preferredPosition={tooltipPosition}>
        {button}
      </SimpleTooltip>
    );
  }

  return button;
}

/**
 * Button component with variants.
 *
 * @param {ReactNode} children - Button content
 * @param {Function} onClick - Click handler
 * @param {string} variant - Button variant: 'default' | 'danger' | 'small' | 'ghost' | 'ghost-danger'
 * @param {boolean} icon - Whether this is an icon-only button
 * @param {boolean} active - Whether button is in active state
 * @param {string} className - Additional CSS classes
 * @param {string} title - Tooltip text
 */
export function Button({
  children,
  onClick,
  variant = 'default',
  icon = false,
  active = false,
  className = '',
  title,
  ...props
}) {
  const classes = [
    'cp-btn',
    variant === 'danger' && 'cp-btn-danger',
    variant === 'small' && 'cp-btn-small',
    variant === 'ghost' && 'cp-btn-ghost',
    variant === 'ghost-danger' && 'cp-btn-ghost cp-btn-ghost-danger',
    icon && 'cp-btn-icon',
    active && 'active',
    className,
  ]
    .filter(Boolean)
    .join(' ');

  return (
    <button class={classes} onClick={onClick} title={title} {...props}>
      {children}
    </button>
  );
}

/**
 * Tool button for toolbars.
 *
 * @param {string} icon - Icon character/emoji
 * @param {string} label - Button label
 * @param {boolean} active - Whether button is active
 * @param {Function} onClick - Click handler
 * @param {string} title - Tooltip text
 */
export function ToolButton({ icon, label, active, onClick, title }) {
  return (
    <button
      class={`cp-tool-btn ${active ? 'active' : ''}`}
      onClick={onClick}
      title={title || label}
    >
      {icon}
    </button>
  );
}

/**
 * Bottom toolbar button with SVG icon and optional label.
 * @param {string} iconPath - SVG path data from ICONS constant
 * @param {boolean} active - Whether button is active
 * @param {Function} onClick - Click handler
 * @param {string} label - Optional button label
 */
export function BottomToolButton({ iconPath, active, onClick, label }) {
  return (
    <button class={`bottom-tool-btn ${active ? 'active' : ''} ${label ? 'btb-labeled' : ''}`} onClick={onClick}>
      <svg
        class="btn-icon"
        width="24"
        height="24"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        stroke-width="2"
        stroke-linecap="round"
        stroke-linejoin="round"
      >
        <path d={iconPath} />
      </svg>
      {label && <span class="btb-label">{label}</span>}
    </button>
  );
}
