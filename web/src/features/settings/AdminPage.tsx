import { useDraftGuard } from '../../components/useDraftGuard';
import { useState } from 'react';
import { api, download } from '../../app/api';
import { useApp, useAction, useLoad } from '../../app/state';
import type { User } from '../../app/types';
import { Button, Dialog, ErrorMessage, PageHeader, Panel, Confirm } from '../../components/ui';
import { saveBlob } from '../fields/values';
function UserEditor({
  user,
  onClose,
  onSaved,
}: {
  user?: User;
  onClose: () => void;
  onSaved: () => void;
}) {
  const [username, setUsername] = useState(user?.username || '');
  const [displayName, setDisplay] = useState(user?.displayName || '');
  const [role, setRole] = useState(user?.role || 'user');
  const [disabled, setDisabled] = useState(!!user?.disabled);
  const [link, setLink] = useState('');
  const [copied, setCopied] = useState(false);
  const guard = useDraftGuard(
    !link &&
      (username !== (user?.username || '') ||
        displayName !== (user?.displayName || '') ||
        role !== (user?.role || 'user') ||
        disabled !== !!user?.disabled),
    onClose,
  );
  const a = useAction();
  return (
    <Dialog
      title={link ? 'Einladungslink' : user ? 'Nutzer bearbeiten' : 'Nutzer einladen'}
      onClose={guard.close}
      footer={
        link ? (
          <Button onClick={onClose}>Schließen</Button>
        ) : (
          <>
            <Button onClick={guard.close}>Abbrechen</Button>
            <Button
              variant="primary"
              disabled={a.busy || !username || !displayName}
              onClick={() =>
                void a.run(async () => {
                  if (user) {
                    await api('/admin/users/' + user.id, 'PATCH', {
                      version: user.version,
                      displayName,
                      role,
                      disabled,
                    });
                    onSaved();
                  } else {
                    const r = await api<{ invitationToken: string }>('/admin/users', 'POST', {
                      username,
                      displayName,
                      role,
                    });
                    setLink(
                      location.origin + '/invite?token=' + encodeURIComponent(r.invitationToken),
                    );
                  }
                })
              }
            >
              {user ? 'Speichern' : 'Einladung erstellen'}
            </Button>
          </>
        )
      }
    >
      {link ? (
        <>
          <p>24 Stunden gültig. Dieser Link wird nur jetzt angezeigt.</p>
          <input
            aria-label="Einladungslink"
            readOnly
            value={link}
            onFocus={(e) => e.target.select()}
          />
          <Button
            onClick={() =>
              void a.run(async () => {
                await navigator.clipboard.writeText(link);
                setCopied(true);
              })
            }
          >
            {copied ? 'Kopiert' : 'Link kopieren'}
          </Button>
        </>
      ) : (
        <div className="form-grid">
          <label className="form-field">
            Benutzername
            <input
              aria-label="Benutzername"
              value={username}
              readOnly={!!user}
              onChange={(e) => setUsername(e.target.value)}
            />
          </label>
          <label className="form-field">
            Anzeigename
            <input
              aria-label="Anzeigename"
              value={displayName}
              onChange={(e) => setDisplay(e.target.value)}
              maxLength={100}
            />
          </label>
          <label className="form-field">
            Rolle
            <select
              aria-label="Rolle"
              value={role}
              onChange={(e) => setRole(e.target.value as User['role'])}
            >
              <option value="user">Nutzer</option>
              <option value="admin">Administrator</option>
            </select>
          </label>
          {user && (
            <label className="check-row">
              <input
                type="checkbox"
                checked={disabled}
                onChange={(e) => setDisabled(e.target.checked)}
              />
              Deaktiviert
            </label>
          )}
        </div>
      )}
      <ErrorMessage error={a.error} />
      {guard.confirmation}
    </Dialog>
  );
}
export function AdminPage() {
  const { me } = useApp();
  const [edit, setEdit] = useState<{ user?: User }>();
  const [backup, setBackup] = useState(false);
  const [password, setPassword] = useState('');
  const a = useAction();
  const users = useLoad((s) => api<{ items: User[] }>('/admin/users', 'GET', undefined, s), []);
  const system = useLoad(
    (s) => api<Record<string, unknown>>('/admin/system', 'GET', undefined, s),
    [],
  );
  if (me.user.role !== 'admin') return <p>Keine Berechtigung.</p>;
  return (
    <>
      <PageHeader title="Verwaltung">
        <Button variant="primary" onClick={() => setEdit({})}>
          Nutzer einladen
        </Button>
      </PageHeader>
      <ErrorMessage error={users.error || system.error || a.error} />
      <Panel title="Nutzer">
        <div className="table-scroll" tabIndex={0} aria-label="Nutzerliste">
          <table>
            <thead>
              <tr>
                <th>Benutzername</th>
                <th>Anzeigename</th>
                <th>Rolle</th>
                <th>Status</th>
                <th>Aktionen</th>
              </tr>
            </thead>
            <tbody>
              {users.data?.items.map((u) => (
                <tr key={u.id}>
                  <th scope="row">{u.username}</th>
                  <td>{u.displayName}</td>
                  <td>{u.role === 'admin' ? 'Administrator' : 'Nutzer'}</td>
                  <td>{u.disabled ? 'Deaktiviert' : 'Aktiv'}</td>
                  <td>
                    <Button onClick={() => setEdit({ user: u })}>Bearbeiten</Button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </Panel>
      <Panel title="Instanzsicherung">
        <div className="actions">
          <Button onClick={() => setBackup(true)}>Vollständige Sicherung</Button>
          <a
            className="button secondary"
            href="/help/backup-restore.md"
            target="_blank"
            rel="noreferrer"
          >
            Offline-Wiederherstellung
          </a>
        </div>
      </Panel>
      <Panel title="System">
        <dl className="details-grid">
          {Object.entries(system.data || {}).map(([k, v]) => (
            <div key={k}>
              <dt>
                {{ version: 'Version', schemaVersion: 'Datenbankschema', database: 'Datenbank' }[
                  k
                ] || k}
              </dt>
              <dd>{typeof v === 'object' ? JSON.stringify(v) : String(v)}</dd>
            </div>
          ))}
        </dl>
      </Panel>
      {edit && (
        <UserEditor
          user={edit.user}
          onClose={() => {
            setEdit(undefined);
            users.reload();
          }}
          onSaved={() => {
            setEdit(undefined);
            users.reload();
          }}
        />
      )}
      {backup && (
        <Dialog
          title="Vollständige Sicherung"
          width="compact"
          onClose={() => {
            setBackup(false);
            setPassword('');
          }}
          footer={
            <>
              <Button
                onClick={() => {
                  setBackup(false);
                  setPassword('');
                }}
              >
                Abbrechen
              </Button>
              <Button
                variant="primary"
                disabled={a.busy || !password}
                onClick={() =>
                  void a.run(async () => {
                    saveBlob(
                      await download('/admin/backups', { password }),
                      'puffolio-instanz.zip',
                    );
                    setPassword('');
                    setBackup(false);
                  })
                }
              >
                Sicherung herunterladen
              </Button>
            </>
          }
        >
          <p>Enthält alle Konten und Sammlungen.</p>
          <label className="form-field">
            Administratorpasswort
            <input
              type="password"
              aria-label="Administratorpasswort"
              autoComplete="current-password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
            />
          </label>
          <ErrorMessage error={a.error} />
        </Dialog>
      )}
    </>
  );
}
