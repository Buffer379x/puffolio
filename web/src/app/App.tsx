import { useState, useEffect, useRef } from 'react';
import { api, setCSRF, APIError } from './api';
import type { Me } from './types';
import { DataProvider, VISITOR } from './state';
import { AppShell } from './AppShell';
import { AuthScreen } from '../features/auth/AuthScreen';
import { LiquidsPage } from '../features/liquids/LiquidsPage';
import { DashboardPage } from '../features/dashboard/DashboardPage';
import { SettingsPage } from '../features/settings/SettingsPage';
import { ToolsPage } from '../features/tools/ToolsPage';
import { NicotineCalculator } from '../features/tools/NicotineCalculator';
import { Dialog, Button, ErrorMessage } from '../components/ui';
export function App() {
  const [me, setMe] = useState<Me>();
  const [mode, setMode] = useState<'setup' | 'login'>('login');
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<unknown>();
  const [expired, setExpired] = useState(false);
  const [route, setRoute] = useState(location.pathname + location.search);
  const [rev, setRev] = useState(0);
  const routeRef = useRef(route);
  routeRef.current = route;
  useEffect(() => {
    let active = true;
    (async () => {
      try {
        const status = await api<{ setupRequired: boolean }>('/setup/status');
        if (!active) return;
        if (status.setupRequired) setMode('setup');
        else
          try {
            const user = await api<Me>('/me');
            if (active) {
              setCSRF(user.csrfToken);
              setMe(user);
            }
          } catch (e) {
            if (!(e instanceof APIError) || e.status !== 401) throw e;
            if (active) setMe(VISITOR);
          }
      } catch (e) {
        if (active) setError(e);
      } finally {
        if (active) setLoading(false);
      }
    })();
    return () => {
      active = false;
    };
  }, [rev]);
  useEffect(() => {
    const pop = () => {
      const event = new Event('puffolio-before-navigate', { cancelable: true });
      if (!window.dispatchEvent(event) && !window.confirm('Ungespeicherte Änderungen verwerfen?')) {
        history.pushState(null, '', routeRef.current);
        return;
      }
      setRoute(location.pathname + location.search);
    };
    const expired = () => setExpired(true);
    window.addEventListener('popstate', pop);
    window.addEventListener('puffolio-session-expired', expired);
    return () => {
      window.removeEventListener('popstate', pop);
      window.removeEventListener('puffolio-session-expired', expired);
    };
  }, []);
  const navigate = (url: string) => {
    const event = new Event('puffolio-before-navigate', { cancelable: true });
    if (!window.dispatchEvent(event) && !window.confirm('Ungespeicherte Änderungen verwerfen?'))
      return;
    const changesPage = new URL(url, location.href).pathname !== location.pathname;
    history.pushState(null, '', url);
    setRoute(url);
    if (changesPage) setTimeout(() => document.getElementById('main')?.focus(), 0);
  };
  const logout = async () => {
    const event = new Event('puffolio-before-navigate', { cancelable: true });
    if (
      !window.dispatchEvent(event) &&
      !window.confirm('Ungespeicherte Änderungen verwerfen und abmelden?')
    )
      return;
    await api('/auth/logout', 'POST');
    setCSRF('');
    setMe(VISITOR);
    setExpired(false);
    setMode('login');
    history.replaceState(null, '', '/dashboard');
    setRoute('/dashboard');
  };
  const login = (next: Me) => {
    setCSRF(next.csrfToken);
    setMe(next);
    setMode('login');
    if (!expired && !routeRef.current.startsWith('/settings')) {
      history.replaceState(null, '', '/dashboard');
      setRoute('/dashboard');
    }
    setExpired(false);
  };
  if (loading)
    return (
      <div className="auth-page">
        <p role="status">Laden…</p>
      </div>
    );
  if (error)
    return (
      <div className="auth-page">
        <ErrorMessage error={error} />
        <Button
          onClick={() => {
            setError(undefined);
            setLoading(true);
            setRev((v) => v + 1);
          }}
        >
          Erneut versuchen
        </Button>
      </div>
    );
  if (mode === 'setup' || !me) return <AuthScreen mode="setup" onSuccess={login} />;
  const [path, search = ''] = route.split('?');
  const signedIn = me.user.role !== 'visitor';
  if (!signedIn && (path === '/login' || path === '/settings'))
    return <AuthScreen mode="login" onSuccess={login} onBack={() => navigate('/dashboard')} />;
  return (
    <>
      <DataProvider key={me.user.id} me={me} setMe={setMe} logout={logout} navigate={navigate}>
        <AppShell path={path}>
          {path === '/liquids' ? (
            <LiquidsPage query={'?' + search} />
          ) : path === '/tools' ? (
            <ToolsPage />
          ) : path === '/tools/nicotine' ? (
            <NicotineCalculator />
          ) : path === '/settings' ? (
            <SettingsPage query={'?' + search} />
          ) : path === '/dashboard' || path === '/' || path === '/login' ? (
            <DashboardPage />
          ) : (
            <>
              <h1>Seite nicht gefunden</h1>
              <Button onClick={() => navigate('/dashboard')}>Dashboard öffnen</Button>
            </>
          )}
        </AppShell>
      </DataProvider>
      {expired && signedIn && (
        <Dialog title="Sitzung abgelaufen" onClose={() => {}} width="compact">
          <p>Erneut anmelden, um den Entwurf fortzusetzen.</p>
          <AuthScreen mode="login" onSuccess={login} embedded />
        </Dialog>
      )}
    </>
  );
}
