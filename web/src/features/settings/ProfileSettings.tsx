import { useState } from 'react';
import { api } from '../../app/api';
import { useApp, useAction, useDirty } from '../../app/state';
import type { Me } from '../../app/types';
import { Button, ErrorMessage, Panel } from '../../components/ui';
export function ProfileSettings() {
  const { me, setMe, preferences, savePreferences, logout } = useApp();
  const [displayName, setDisplayName] = useState(me.user.displayName);
  const [timezone, setTimezone] = useState(preferences.timezone);
  const [currentPassword, setCurrent] = useState('');
  const [newPassword, setNew] = useState('');
  const [notice, setNotice] = useState('');
  const a = useAction();
  useDirty(
    displayName !== me.user.displayName ||
      timezone !== preferences.timezone ||
      !!currentPassword ||
      !!newPassword,
  );
  return (
    <>
      <Panel title="Profil">
        <form
          onSubmit={(e) => {
            e.preventDefault();
            void a.run(async () => {
              if (displayName !== me.user.displayName)
                setMe(await api<Me>('/me', 'PATCH', { version: me.user.version, displayName }));
              if (timezone !== preferences.timezone) await savePreferences({ timezone });
              setNotice('Profil gespeichert.');
            });
          }}
        >
          <div className="form-grid">
            <label className="form-field">
              Benutzername
              <input aria-label="Benutzername" value={me.user.username} readOnly />
            </label>
            <label className="form-field">
              Anzeigename
              <input
                aria-label="Anzeigename"
                required
                maxLength={100}
                value={displayName}
                onChange={(e) => setDisplayName(e.target.value)}
              />
            </label>
            <label className="form-field full">
              Zeitzone
              <input
                aria-label="Zeitzone"
                required
                placeholder="Europe/Berlin"
                value={timezone}
                onChange={(e) => setTimezone(e.target.value)}
                list="timezones"
              />
              <datalist id="timezones">
                {Intl.supportedValuesOf('timeZone').map((z) => (
                  <option key={z} value={z} />
                ))}
              </datalist>
            </label>
          </div>
          <Button type="submit" variant="primary" disabled={a.busy}>
            Speichern
          </Button>
        </form>
      </Panel>
      <Panel title="Passwort">
        <form
          onSubmit={(e) => {
            e.preventDefault();
            void a.run(async () => {
              await api('/auth/password', 'POST', { currentPassword, newPassword });
              setMe(await api<Me>('/me'));
              setCurrent('');
              setNew('');
              setNotice('Passwort geändert. Andere Sitzungen wurden beendet.');
            });
          }}
        >
          <div className="form-grid">
            <label className="form-field">
              Aktuelles Passwort
              <input
                aria-label="Aktuelles Passwort"
                type="password"
                autoComplete="current-password"
                value={currentPassword}
                required
                onChange={(e) => setCurrent(e.target.value)}
              />
            </label>
            <label className="form-field">
              Neues Passwort
              <input
                aria-label="Neues Passwort"
                type="password"
                autoComplete="new-password"
                minLength={12}
                value={newPassword}
                required
                onChange={(e) => setNew(e.target.value)}
              />
            </label>
          </div>
          <Button type="submit" disabled={a.busy}>
            Passwort ändern
          </Button>
        </form>
      </Panel>
      <ErrorMessage error={a.error} />
      {notice && <p role="status">{notice}</p>}
      <Button onClick={() => void a.run(logout)}>Abmelden</Button>
    </>
  );
}
