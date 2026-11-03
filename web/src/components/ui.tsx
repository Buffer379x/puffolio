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
const paths: Record<string, string> = {
  dashboard: 'M3 3h7v7H3z M14 3h7v7h-7z M3 14h7v7H3z M14 14h7v7h-7z',
  liquids: 'M12 2C10 6 5 10 5 15a7 7 0 0014 0c0-5-5-9-7-13Z',
  tools: 'm14 6 4-4 4 4-4 4 M16 8 3 21 M3 3l6 6 M2 6l4-4 M18 15l4 4-3 3-4-4',
  settings:
    'M12 8a4 4 0 100 8 4 4 0 000-8 M12 2v3 M12 19v3 M2 12h3 M19 12h3 M5 5l2 2 M17 17l2 2 M5 19l2-2 M17 7l2-2',
  admin: 'M12 2 3 6v6c0 5 9 10 9 10s9-5 9-10V6Z',
  menu: 'M3 6h18 M3 12h18 M3 18h18',
  close: 'm6 6 12 12 M18 6 6 18',
  star: 'm12 2 3 6 7 1-5 5 1 7-6-3-6 3 1-7-5-5 7-1Z',
  edit: 'm4 16 12-12 4 4L8 20H4Z',
  plus: 'M12 4v16 M4 12h16',
  help: 'M9 8a3 3 0 116 0c0 3-3 2-3 6 M12 18v1',
  arrow: 'M5 12h14 M13 6l6 6-6 6',
  trash: 'M3 6h18 M9 6V3h6v3 M6 6l1 15h10l1-15 M10 10v7 M14 10v7',
  sun: 'M12 7a5 5 0 100 10 5 5 0 000-10 M12 1v3 M12 20v3 M1 12h3 M20 12h3',
  moon: 'M21 13A9 9 0 0111 3a9 9 0 1010 10Z',
  system: 'M3 3h18v14H3Z M8 21h8 M12 17v4',
  image: 'M3 3h18v18H3Z m0 14 6-6 4 4 3-3 5 5 M15 7h1',
  logout: 'M9 3H3v18h6 M8 12h13 M17 8l4 4-4 4',
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
      strokeWidth="1.7"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      <path d={paths[name] || paths.liquids} />
    </svg>
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
  actions,
  children,
  className = '',
}: {
  title?: string;
  actions?: ReactNode;
  children: ReactNode;
  className?: string;
}) {
  return (
    <section className={'panel ' + className}>
      {title && (
        <header className="panel-head">
          <h2>{title}</h2>
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
  options: { value: T; label: string }[];
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
          onClick={() => onChange(o.value)}
        >
          {o.label}
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
      if (previous?.isConnected && !previous.closest('[inert]')) previous.focus();
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
