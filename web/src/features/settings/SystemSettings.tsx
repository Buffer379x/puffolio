import { useState } from 'react';
import { api, download } from '../../app/api';
import { useAction, useLoad } from '../../app/state';
import { Button, Dialog, ErrorMessage, Panel } from '../../components/ui';
import { saveBlob } from '../fields/values';
const labels: Record<string, string> = {
  version: 'Version',
  schemaVersion: 'Datenbankschema',
  database: 'Datenbank',
};
/** Backup and instance status, beside the collection's own data. */
export function SystemSettings() {
  const [open, setOpen] = useState(false);
  const [password, setPassword] = useState('');
  const a = useAction();
  const system = useLoad(
    (s) => api<Record<string, unknown>>('/admin/system', 'GET', undefined, s),
    [],
  );
  const close = () => {
    setOpen(false);
    setPassword('');
  };
  return (
    <>
      <Panel title="Instanzsicherung">
        <div className="actions">
          <Button onClick={() => setOpen(true)}>Vollständige Sicherung</Button>
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
        <ErrorMessage error={system.error} />
        <dl className="details-grid">
          {Object.entries(system.data || {}).map(([k, v]) => (
            <div key={k}>
              <dt>{labels[k] || k}</dt>
              <dd>{typeof v === 'object' ? JSON.stringify(v) : String(v)}</dd>
            </div>
          ))}
        </dl>
      </Panel>
      {open && (
        <Dialog
          title="Vollständige Sicherung"
          width="compact"
          onClose={close}
          footer={
            <>
              <Button onClick={close}>Abbrechen</Button>
              <Button
                variant="primary"
                disabled={a.busy || !password}
                onClick={() =>
                  void a.run(async () => {
                    saveBlob(
                      await download('/admin/backups', { password }),
                      'puffolio-instanz.zip',
                    );
                    close();
                  })
                }
              >
                Sicherung herunterladen
              </Button>
            </>
          }
        >
          <p>Enthält Datenbank, Bilder und Konto.</p>
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
