import {
  createContext,
  useContext,
  useState,
  useEffect,
  useCallback,
  useRef,
  type ReactNode,
} from 'react';
import { api, setCSRF } from './api';
import type { Field, Preferences, Me } from './types';
import { ToastProvider } from '../components/toast';
export const AppContext = createContext<{
  me: Me;
  setMe: (m: Me) => void;
  fields: Field[];
  schemaVersion: number;
  preferences: Preferences;
  savePreferences: (patch: Partial<Preferences>) => Promise<void>;
  refresh: () => Promise<void>;
  logout: () => Promise<void>;
  navigate: (path: string) => void;
}>({} as never);
export const useApp = () => useContext(AppContext);
/** Everyone can look; only the signed-in administrator changes anything. The server enforces it. */
export const VISITOR: Me = {
  user: { id: 'visitor', username: '', displayName: 'Besuch', role: 'visitor', version: 0 },
  collectionId: '',
  csrfToken: '',
};
export const useCanEdit = () => useApp().me?.user.role !== 'visitor';
const VISITOR_VIEW = 'puffolio-visitor-view';
function readVisitorView(): Partial<Preferences> {
  try {
    return JSON.parse(localStorage.getItem(VISITOR_VIEW) || '{}');
  } catch {
    return {};
  }
}
export function useLoad<T>(loader: (signal: AbortSignal) => Promise<T>, deps: unknown[] = []) {
  const [data, setData] = useState<T>();
  const [error, setError] = useState<unknown>();
  const [loading, setLoading] = useState(true);
  const [rev, setRev] = useState(0);
  useEffect(() => {
    const c = new AbortController();
    let active = true;
    setLoading(true);
    setError(undefined);
    loader(c.signal)
      .then((d) => {
        if (active) setData(d);
      })
      .catch((e) => {
        if (active && e.name !== 'AbortError') setError(e);
      })
      .finally(() => {
        if (active) setLoading(false);
      });
    return () => {
      active = false;
      c.abort();
    };
  }, [...deps, rev]);
  return { data, error, loading, reload: () => setRev((v) => v + 1), setData };
}
export function useAction() {
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<unknown>();
  const run = async (fn: () => Promise<void>) => {
    setBusy(true);
    setError(undefined);
    try {
      await fn();
    } catch (e) {
      setError(e);
    } finally {
      setBusy(false);
    }
  };
  return { busy, error, setError, run };
}
export function Theme({ theme }: { theme: Preferences['theme'] }) {
  useEffect(() => {
    const m = matchMedia('(prefers-color-scheme: dark)');
    const apply = () => {
      document.documentElement.dataset.theme =
        theme === 'system' ? (m.matches ? 'dark' : 'light') : theme;
    };
    apply();
    try {
      localStorage.setItem('puffolio-theme', theme);
    } catch {}
    m.addEventListener('change', apply);
    return () => m.removeEventListener('change', apply);
  }, [theme]);
  return null;
}
export function useDirty(dirty: boolean) {
  const ref = useRef(dirty);
  ref.current = dirty;
  useEffect(() => {
    const unload = (e: BeforeUnloadEvent) => {
      if (ref.current) {
        e.preventDefault();
        e.returnValue = '';
      }
    };
    const navigate = (e: Event) => {
      if (ref.current) e.preventDefault();
    };
    window.addEventListener('beforeunload', unload);
    window.addEventListener('puffolio-before-navigate', navigate);
    return () => {
      window.removeEventListener('beforeunload', unload);
      window.removeEventListener('puffolio-before-navigate', navigate);
    };
  }, []);
}
export function DataProvider({
  me,
  setMe,
  logout,
  navigate,
  children,
}: {
  me: Me;
  setMe: (m: Me) => void;
  logout: () => Promise<void>;
  navigate: (s: string) => void;
  children: ReactNode;
}) {
  const visitor = me.user.role === 'visitor';
  const [fields, setFields] = useState<Field[]>([]);
  const [schemaVersion, setVersion] = useState(1);
  const [preferences, setPreferences] = useState<Preferences>();
  const [error, setError] = useState<unknown>();
  const refresh = useCallback(async () => {
    const [f, p] = await Promise.all([
      api<{ items: Field[]; version: number }>('/fields'),
      api<Preferences>('/preferences'),
    ]);
    setFields(f.items || []);
    setVersion(f.version);
    setPreferences(visitor ? { ...p, theme: 'system', ...readVisitorView() } : p);
  }, [visitor]);
  useEffect(() => {
    setCSRF(me.csrfToken);
    refresh().catch(setError);
  }, [refresh, me.csrfToken]);
  const savePreferences = async (patch: Partial<Preferences>) => {
    if (!preferences) return;
    const next = { ...preferences, ...patch };
    if (visitor) {
      const { theme, view, images, listFields, gridFields } = next;
      setPreferences(next);
      try {
        localStorage.setItem(
          VISITOR_VIEW,
          JSON.stringify({ theme, view, images, listFields, gridFields }),
        );
      } catch {
        /* Private mode: the choice simply lasts for this page view. */
      }
      return;
    }
    const existing = new Set(fields.map((f) => f.id));
    next.listFields = next.listFields.filter((id) => existing.has(id));
    next.gridFields = next.gridFields.filter((id) => existing.has(id));
    next.roles = Object.fromEntries(
      Object.entries(next.roles).filter(
        ([, id]) => !id || fields.some((f) => f.id === id && !f.archived),
      ),
    );
    const p = await api<Preferences>('/preferences', 'PUT', next);
    setPreferences(p);
  };
  if (!preferences)
    return (
      <main>
        {error ? (
          <>
            <p role="alert">{error instanceof Error ? error.message : 'Laden fehlgeschlagen'}</p>
            <button onClick={() => refresh().catch(setError)}>Erneut versuchen</button>
          </>
        ) : (
          <p role="status">Laden…</p>
        )}
      </main>
    );
  return (
    <AppContext.Provider
      value={{
        me,
        setMe,
        fields,
        schemaVersion,
        preferences,
        savePreferences,
        refresh,
        logout,
        navigate,
      }}
    >
      <Theme theme={preferences.theme} />
      <ToastProvider>{children}</ToastProvider>
    </AppContext.Provider>
  );
}
