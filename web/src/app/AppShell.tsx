import { useEffect, useRef, useState, type ReactNode } from 'react';
import { useApp, useAction, useCanEdit } from './state';
import { Icon, IconButton, SegmentedControl, ErrorMessage } from '../components/ui';
const links = [
  ['/dashboard', 'Dashboard', 'dashboard'],
  ['/liquids', 'Liquids', 'liquids'],
  ['/tools', 'Tools', 'tools'],
  ['/settings', 'Einstellungen', 'settings'],
];
export function AppShell({ path, children }: { path: string; children: ReactNode }) {
  const { me, preferences, savePreferences, navigate, logout } = useApp();
  const [mobile, setMobile] = useState(() => matchMedia('(max-width: 900px)').matches);
  const [open, setOpen] = useState(false);
  const sidebar = useRef<HTMLElement>(null);
  const toggle = useRef<HTMLButtonElement>(null);
  const content = useRef<HTMLDivElement>(null);
  const returnFocus = useRef<HTMLElement | null>(null);
  const a = useAction();
  const canEdit = useCanEdit();
  const close = () => {
    setOpen(false);
  };
  useEffect(() => {
    const media = matchMedia('(max-width: 900px)');
    const change = () => {
      setMobile(media.matches);
      setOpen(false);
      if (media.matches && sidebar.current?.contains(document.activeElement))
        toggle.current?.focus();
    };
    media.addEventListener('change', change);
    return () => media.removeEventListener('change', change);
  }, []);
  useEffect(() => {
    const el = sidebar.current;
    if (!el) return;
    if (mobile && open && !returnFocus.current)
      returnFocus.current = document.activeElement as HTMLElement | null;
    el.inert = mobile && !open;
    if (content.current) content.current.inert = mobile && open;
    if (!mobile || !open) {
      const previous = returnFocus.current;
      returnFocus.current = null;
      if (previous?.isConnected && !previous.closest('[inert]')) previous.focus();
      return;
    }
    el.querySelector<HTMLButtonElement>('button')?.focus();
    const key = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        e.preventDefault();
        close();
      }
      if (e.key === 'Tab') {
        const controls = Array.from(
          el.querySelectorAll<HTMLElement>('a[href],button:not([disabled]),input,select'),
        ).filter((e) => !e.hidden);
        const first = controls[0],
          last = controls.at(-1);
        if (e.shiftKey && document.activeElement === first) {
          e.preventDefault();
          last?.focus();
        } else if (!e.shiftKey && document.activeElement === last) {
          e.preventDefault();
          first?.focus();
        }
      }
    };
    el.addEventListener('keydown', key);
    return () => {
      el.removeEventListener('keydown', key);
    };
  }, [mobile, open]);
  const go = (url: string) => {
    if (mobile) setOpen(false);
    navigate(url);
  };
  return (
    <>
      <a href="#main" className="skip-link">
        Zum Inhalt
      </a>
      {mobile && open && <div className="nav-shade" onClick={close} />}
      <aside
        ref={sidebar}
        className={'sidebar ' + (open ? 'open' : '')}
        role={mobile && open ? 'dialog' : undefined}
        aria-modal={mobile && open ? true : undefined}
        aria-label="Hauptnavigation"
        aria-hidden={mobile && !open}
      >
        <div className="brand">
          <img src="/brand/mark.svg" alt="" />
          <span>
            puffolio<span className="brand-period">.</span>
          </span>
        </div>
        {mobile && (
          <IconButton
            className="drawer-close"
            label="Menü schließen"
            icon="close"
            onClick={close}
          />
        )}
        <nav>
          {links
            .filter(([url]) => url !== '/settings' || canEdit)
            .map(([url, label, icon]) => (
              <a
                className={'nav-item ' + (path.startsWith(url) ? 'active' : '')}
                key={url}
                href={url}
                aria-current={path.startsWith(url) ? 'page' : undefined}
                onClick={(e) => {
                  e.preventDefault();
                  go(url);
                }}
              >
                <Icon name={icon} />
                {label}
              </a>
            ))}
        </nav>
        <div className="sidebar-bottom">
          <div className="theme-switch">
            <SegmentedControl
              label="Erscheinungsbild"
              value={preferences.theme}
              options={[
                { value: 'light', label: 'Hell', icon: 'sun' },
                { value: 'dark', label: 'Dunkel', icon: 'moon' },
                { value: 'system', label: 'System', icon: 'system' },
              ]}
              onChange={(theme) => void a.run(() => savePreferences({ theme }))}
            />
          </div>
          <ErrorMessage error={a.error} />
          {canEdit ? (
            <div className="sidebar-profile">
              <div className="avatar" aria-hidden="true">
                {me.user.displayName.slice(0, 2).toLocaleUpperCase('de')}
              </div>
              <a
                href="/settings?section=profile"
                onClick={(e) => {
                  e.preventDefault();
                  go('/settings?section=profile');
                }}
              >
                <strong>{me.user.displayName}</strong>
                <span>Administrator</span>
              </a>
              <IconButton label="Abmelden" icon="logout" onClick={() => void a.run(logout)} />
            </div>
          ) : (
            <div className="sidebar-profile">
              <a
                className="nav-item"
                href="/login"
                onClick={(e) => {
                  e.preventDefault();
                  go('/login');
                }}
              >
                <Icon name="logout" />
                Anmelden
              </a>
            </div>
          )}
        </div>
      </aside>
      <div ref={content} className="app-shell">
        <header className="topbar">
          <button
            ref={toggle}
            className="icon-button menu-toggle"
            type="button"
            aria-label="Menü öffnen"
            aria-expanded={open}
            onClick={() => setOpen(true)}
          >
            <Icon name="menu" />
          </button>
          <div className="brand" aria-hidden="true">
            <img src="/brand/mark.svg" alt="" />
            <span>
              puffolio<span className="brand-period">.</span>
            </span>
          </div>
        </header>
        <main id="main" tabIndex={-1}>
          {children}
        </main>
      </div>
    </>
  );
}
