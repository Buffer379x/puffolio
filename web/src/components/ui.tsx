import {
  useEffect,
  useId,
  useRef,
  useState,
  type ReactNode,
  type ButtonHTMLAttributes,
} from 'react';
import { APIError } from '../app/api';
export function Button({
  variant = 'secondary',
  className = '',
  ...p
}: ButtonHTMLAttributes<HTMLButtonElement> & { variant?: 'primary' | 'secondary' | 'danger' }) {
  return <button type="button" {...p} className={`button ${variant} ${className}`} />;
}
/** One icon set for the whole app; every glyph is drawn on the same 24px grid. */
const icons: Record<string, ReactNode> = {
  dashboard: (
    <>
      <rect x="3" y="3" width="7" height="7" rx="1.5" />
      <rect x="14" y="3" width="7" height="11" rx="1.5" />
      <rect x="3" y="14" width="7" height="7" rx="1.5" />
      <rect x="14" y="18" width="7" height="3" rx="1" />
    </>
  ),
  liquids: (
    <>
      <path d="M12 3c-2 4-7 8-7 12a7 7 0 0 0 14 0c0-4-5-8-7-12Z" />
      <path d="M9 16a3 3 0 0 0 3 3" />
    </>
  ),
  tools: (
    <>
      <path d="M4 7h7m6 0h3M4 17h3m6 0h7" />
      <circle cx="14" cy="7" r="3" />
      <circle cx="10" cy="17" r="3" />
    </>
  ),
  settings: (
    <>
      <circle cx="12" cy="12" r="3" />
      <path d="M19.4 15a1.7 1.7 0 0 0 .3 1.8l.1.1a2 2 0 1 1-2.9 2.9l-.1-.1a1.7 1.7 0 0 0-2.9 1.2V21a2 2 0 0 1-4 0v-.1a1.7 1.7 0 0 0-2.9-1.2l-.1.1a2 2 0 1 1-2.9-2.9l.1-.1a1.7 1.7 0 0 0-1.2-2.9H3a2 2 0 0 1 0-4h.1a1.7 1.7 0 0 0 1.2-2.9l-.1-.1a2 2 0 1 1 2.9-2.9l.1.1a1.7 1.7 0 0 0 1.9.3H9a1.7 1.7 0 0 0 1-1.5V3a2 2 0 0 1 4 0v.1a1.7 1.7 0 0 0 2.9 1.2l.1-.1a2 2 0 1 1 2.9 2.9l-.1.1a1.7 1.7 0 0 0-.3 1.9V9a1.7 1.7 0 0 0 1.5 1H21a2 2 0 0 1 0 4h-.1a1.7 1.7 0 0 0-1.5 1Z" />
    </>
  ),
  admin: <path d="M12 3 4 6v6c0 5 4 8 8 9 4-1 8-4 8-9V6Z" />,
  menu: <path d="M4 6h16M4 12h16M4 18h16" />,
  close: <path d="m6 6 12 12M6 18 18 6" />,
  star: <path d="m12 3 2.8 5.7 6.2.9-4.5 4.4 1.1 6.2-5.6-3-5.6 3 1.1-6.2L3 9.6l6.2-.9Z" />,
  heart: <path d="m12 21-8-8a5.5 5.5 0 0 1 8-7.5 5.5 5.5 0 0 1 8 7.5Z" />,
  edit: <path d="m16 3 5 5-13 13H3v-5Zm-2 2 5 5" />,
  plus: <path d="M12 5v14M5 12h14" />,
  help: (
    <>
      <circle cx="12" cy="12" r="9" />
      <path d="M9.5 9a2.5 2.5 0 1 1 4.5 1.5c-1 1-2 1-2 3M12 17h.01" />
    </>
  ),
  arrow: <path d="M5 12h14m-5-5 5 5-5 5" />,
  up: <path d="m6 15 6-6 6 6" />,
  down: <path d="m6 9 6 6 6-6" />,
  sortAsc: <path d="M12 19V5m-6 6 6-6 6 6" />,
  sortDesc: <path d="M12 5v14m-6-6 6 6 6-6" />,
  trash: <path d="M3 6h18M9 6V3h6v3M6 6l1 15h10l1-15M10 10v7M14 10v7" />,
  archive: <path d="M3 4h18v4H3ZM5 8v12h14V8M10 12h4" />,
  restore: <path d="M3 12a9 9 0 1 0 3-6.7L3 8M3 3v5h5" />,
  sun: (
    <>
      <circle cx="12" cy="12" r="4" />
      <path d="M12 2v2m0 16v2M2 12h2m16 0h2M5 5l1 1m12 12 1 1M5 19l1-1M18 6l1-1" />
    </>
  ),
  moon: <path d="M20 14A8 8 0 0 1 10 4a8 8 0 1 0 10 10Z" />,
  system: (
    <>
      <rect x="3" y="3" width="18" height="13" rx="2" />
      <path d="M12 16v5m-5 0h10" />
    </>
  ),
  image: (
    <>
      <rect x="3" y="3" width="18" height="18" rx="2" />
      <circle cx="8" cy="8" r="2" />
      <path d="m3 17 5-5 4 4 4-7 5 8" />
    </>
  ),
  logout: <path d="M9 3H4v18h5M9 12h12m-4-4 4 4-4 4" />,
  search: (
    <>
      <circle cx="10.5" cy="10.5" r="6.5" />
      <path d="m16 16 4.5 4.5" />
    </>
  ),
  filter: <path d="M3 5h18l-7 8v6l-4 2v-8Z" />,
  sliders: (
    <>
      <path d="M4 7h7m6 0h3M4 17h3m6 0h7" />
      <circle cx="14" cy="7" r="3" />
      <circle cx="10" cy="17" r="3" />
    </>
  ),
  grid: (
    <>
      <rect x="3" y="3" width="7" height="7" rx="1" />
      <rect x="14" y="3" width="7" height="7" rx="1" />
      <rect x="3" y="14" width="7" height="7" rx="1" />
      <rect x="14" y="14" width="7" height="7" rx="1" />
    </>
  ),
  list: <path d="M8 6h13M8 12h13M8 18h13M3 6h.01M3 12h.01M3 18h.01" />,
  box: <path d="m3 7 9-4 9 4v10l-9 4-9-4Zm0 0 9 4 9-4m-9 4v10" />,
  calendar: (
    <>
      <rect x="3" y="5" width="18" height="16" rx="2" />
      <path d="M7 3v4m10-4v4M3 11h18" />
    </>
  ),
  check: <path d="m5 12 4 4L19 6" />,
  copy: (
    <>
      <rect x="9" y="9" width="12" height="12" rx="2" />
      <path d="M5 15H4a1 1 0 0 1-1-1V4a1 1 0 0 1 1-1h10a1 1 0 0 1 1 1v1" />
    </>
  ),
  tag: (
    <>
      <path d="M3 3h8l10 10-8 8L3 11Z" />
      <circle cx="7.5" cy="7.5" r="1" />
    </>
  ),
};
export function Icon({ name }: { name: string }) {
  return (
    <svg
      className="icon"
      width="20"
      height="20"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.6"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      {icons[name] || icons.liquids}
    </svg>
  );
}
/** Read-only rating. "Unrated" renders as a dash so it never reads as zero stars. */
export function Stars({ value, max = 5 }: { value: number; max?: number }) {
  return (
    <span className="stars" role="img" aria-label={`${value} von ${max}`}>
      {Array.from({ length: max }, (_, i) => (
        <span key={i} className={i < value ? 'star on' : 'star'}>
          <Icon name="star" />
        </span>
      ))}
    </span>
  );
}
export function IconButton({
  label,
  icon,
  ...p
}: ButtonHTMLAttributes<HTMLButtonElement> & { label: string; icon: string }) {
  return (
    <button
      type="button"
      {...p}
      className={'icon-button ' + (p.className || '')}
      aria-label={label}
      title={label}
    >
      <Icon name={icon} />
    </button>
  );
}
export function PageHeader({
  title,
  meta,
  children,
}: {
  title: string;
  meta?: ReactNode;
  children?: ReactNode;
}) {
  return (
    <div className="page-heading">
      <div>
        <h1>{title}</h1>
        {meta && <div className="heading-meta">{meta}</div>}
      </div>
      <div className="heading-actions">{children}</div>
    </div>
  );
}
export function Panel({
  title,
  subtitle,
  actions,
  children,
  className = '',
}: {
  title?: string;
  subtitle?: ReactNode;
  actions?: ReactNode;
  children: ReactNode;
  className?: string;
}) {
  return (
    <section className={'panel ' + className}>
      {title && (
        <header className="panel-head">
          <div className="panel-title">
            <h2>{title}</h2>
            {subtitle && <p className="panel-subtitle">{subtitle}</p>}
          </div>
          {actions}
        </header>
      )}
      <div className="panel-body">{children}</div>
    </section>
  );
}
export interface FieldBindings {
  id: string;
  'aria-labelledby': string;
  'aria-describedby'?: string;
  'aria-invalid': boolean;
}
export function FormField({
  label,
  children,
  error,
  hint,
  wide = false,
  group = false,
  required = false,
  compact = false,
}: {
  label: string;
  children: (bindings: FieldBindings) => ReactNode;
  error?: string;
  hint?: string;
  wide?: boolean;
  group?: boolean;
  required?: boolean;
  compact?: boolean;
}) {
  const id = useId();
  const labelId = id + '-label';
  const description =
    [hint ? id + '-hint' : null, error ? id + '-error' : null].filter(Boolean).join(' ') ||
    undefined;
  const bindings: FieldBindings = {
    id: id + '-control',
    'aria-labelledby': labelId,
    'aria-describedby': description,
    'aria-invalid': !!error,
  };
  const className = 'form-field ' + (wide ? 'full ' : '') + (compact ? 'field-compact' : '');
  const caption = (
    <>
      {label}
      {required && <span aria-hidden="true"> *</span>}
    </>
  );
  const body = (
    <>
      <div className="field-content">{children(bindings)}</div>
      {hint && (
        <span id={id + '-hint'} className="muted">
          {hint}
        </span>
      )}
      {error && (
        <span id={id + '-error'} className="field-error" role="alert">
          {error}
        </span>
      )}
    </>
  );
  return group ? (
    <fieldset
      className={className}
      aria-labelledby={labelId}
      aria-describedby={description}
      aria-invalid={!!error}
    >
      <legend className="field-label" id={labelId}>
        {caption}
      </legend>
      {body}
    </fieldset>
  ) : (
    <div className={className}>
      <label className="field-label" id={labelId} htmlFor={bindings.id}>
        {caption}
      </label>
      {body}
    </div>
  );
}
export function SegmentedControl<T extends string>({
  value,
  options,
  onChange,
  label,
}: {
  value: T;
  options: { value: T; label: string; icon?: string }[];
  onChange: (v: T) => void;
  label: string;
}) {
  return (
    <div className="segmented" aria-label={label} role="group">
      {options.map((o) => (
        <button
          type="button"
          key={o.value}
          aria-pressed={value === o.value}
          aria-label={o.icon ? o.label : undefined}
          title={o.icon ? o.label : undefined}
          onClick={() => onChange(o.value)}
        >
          {o.icon ? <Icon name={o.icon} /> : o.label}
        </button>
      ))}
    </div>
  );
}
export function ErrorMessage({ error }: { error: unknown }) {
  if (!error) return null;
  return (
    <div className="error-box" role="alert">
      {error instanceof Error ? error.message : 'Anfrage fehlgeschlagen.'}
      {error instanceof APIError && error.status === 409 && (
        <p>Der Entwurf bleibt erhalten. Aktuelle Daten neu laden und Änderungen vergleichen.</p>
      )}
      {error instanceof APIError &&
        Object.entries(error.fields).map(([k, v]) => <p key={k}>{v}</p>)}
    </div>
  );
}
export function Empty({ children, action }: { children: ReactNode; action?: ReactNode }) {
  return (
    <div className="empty-state">
      <Icon name="liquids" />
      <p>{children}</p>
      {action}
    </div>
  );
}
let dialogDepth = 0;
export function Dialog({
  open = true,
  title,
  onClose,
  children,
  footer,
  width = 'standard',
  liquid = false,
}: {
  open?: boolean;
  title: string;
  onClose: () => void;
  children: ReactNode;
  footer?: ReactNode;
  width?: 'compact' | 'standard' | 'wide';
  liquid?: boolean;
}) {
  const ref = useRef<HTMLDialogElement>(null);
  const closeRef = useRef(onClose);
  closeRef.current = onClose;
  const titleId = useId();
  useEffect(() => {
    const el = ref.current;
    if (!el || !open) return;
    const previous = document.activeElement as HTMLElement | null;
    dialogDepth++;
    document.body.classList.add('dialog-open');
    el.showModal();
    return () => {
      el.close();
      dialogDepth--;
      if (!dialogDepth) document.body.classList.remove('dialog-open');
      if (previous?.isConnected && !previous.closest('[inert]')) {
        // Safari leaves a clicked button unfocused and focus lands on the nearest
        // scroll region. Refocusing that would draw a ring around the whole list.
        if (previous.matches('a[href],button,input,select,textarea,summary')) previous.focus();
        else if (document.activeElement === previous) previous.blur();
      }
    };
  }, [open]);
  if (!open) return null;
  return (
    <dialog
      ref={ref}
      aria-labelledby={titleId}
      className={`${width} ${liquid ? 'liquid-dialog' : ''}`}
      onCancel={(e) => {
        e.preventDefault();
        e.stopPropagation();
        closeRef.current();
      }}
    >
      <div className="dialog-content">
        <header className="modal-head">
          <h2 id={titleId}>{title}</h2>
          <IconButton label="Schließen" icon="close" onClick={onClose} />
        </header>
        <div className="modal-body">{children}</div>
        {footer && <footer className="modal-footer">{footer}</footer>}
      </div>
    </dialog>
  );
}
export function Confirm({
  title,
  children,
  onCancel,
  onConfirm,
  danger = false,
}: {
  title: string;
  children: ReactNode;
  onCancel: () => void;
  onConfirm: () => void;
  danger?: boolean;
}) {
  return (
    <Dialog
      title={title}
      width="compact"
      onClose={onCancel}
      footer={
        <>
          <Button onClick={onCancel}>Abbrechen</Button>
          <Button variant={danger ? 'danger' : 'primary'} onClick={onConfirm}>
            Bestätigen
          </Button>
        </>
      }
    >
      {children}
    </Dialog>
  );
}
export function HelpPopover({ label, children }: { label: string; children: ReactNode }) {
  const [open, setOpen] = useState(false);
  return (
    <>
      <IconButton label={label} icon="help" onClick={() => setOpen(true)} />
      {open && (
        <Dialog title={label} width="compact" onClose={() => setOpen(false)}>
          {children}
        </Dialog>
      )}
    </>
  );
}
export function ItemCard({
  image,
  title,
  children,
  actions,
}: {
  image?: ReactNode;
  title: ReactNode;
  children?: ReactNode;
  actions?: ReactNode;
}) {
  return (
    <article className="item-card">
      {image}
      <div className="item-card-content">
        <h2>{title}</h2>
        {children}
        {actions && <div className="item-card-actions">{actions}</div>}
      </div>
    </article>
  );
}
