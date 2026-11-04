import { useState } from 'react';
import { api, upload, download } from '../../app/api';
import { useApp, useAction, useLoad } from '../../app/state';
import type { Liquid, Page } from '../../app/types';
import { Button, ErrorMessage, Panel, Dialog, Confirm, Empty } from '../../components/ui';
import { saveBlob, formatDate } from '../fields/values';
interface Preview {
  token: string;
  counts: { liquids: number; fields: number; media: number };
  warnings: string[];
}
export function DataSettings() {
  const { refresh } = useApp();
  const [preview, setPreview] = useState<Preview>();
  const [notice, setNotice] = useState('');
  const [page, setPage] = useState(1);
  const [selected, setSelected] = useState<string[]>([]);
  const [permanent, setPermanent] = useState<Liquid>();
  const a = useAction();
  const trash = useLoad(
    (s) =>
      api<Page<Liquid>>('/liquids?trash=true&page=' + page + '&pageSize=24', 'GET', undefined, s),
    [page],
  );
  const reload = () => {
    trash.reload();
    setSelected([]);
  };
  return (
    <>
      <Panel title="Export / Import">
        <div className="actions">
          <Button
            onClick={() =>
              void a.run(async () =>
                saveBlob(await download('/exports', {}), 'puffolio-sammlung.zip'),
              )
            }
          >
            Gesamte Sammlung exportieren
          </Button>
          <label className="button secondary upload-button">
            Import prüfen
            <input
              type="file"
              aria-label="Importdatei"
              accept=".zip,application/zip"
              disabled={a.busy}
              onChange={(e) => {
                const file = e.target.files?.[0];
                e.target.value = '';
                if (file)
                  void a.run(async () =>
                    setPreview(await upload<Preview>('/imports/preview', file)),
                  );
              }}
            />
          </label>
        </div>
        <ErrorMessage error={a.error} />
        {notice && <p role="status">{notice}</p>}
      </Panel>
      <Panel title="Papierkorb">
        <p className="muted panel-note">Gelöschte Liquids bleiben 30 Tage wiederherstellbar.</p>
        <ErrorMessage error={trash.error} />
        {trash.loading && <p role="status">Laden…</p>}
        {selected.length > 0 && (
          <div className="selection-bar">
            <span>{selected.length} ausgewählt</span>
            <Button
              onClick={() =>
                void a.run(async () => {
                  await api('/liquids/bulk', 'POST', { ids: selected, action: 'restore' });
                  reload();
                })
              }
            >
              Auswahl wiederherstellen
            </Button>
          </div>
        )}
        {trash.data?.items.length === 0 && <Empty>Der Papierkorb ist leer.</Empty>}
        <div className="trash-list">
          {trash.data?.items.map((l) => (
            <div className="setting-line" key={l.id}>
              <label className="check-row">
                <input
                  type="checkbox"
                  aria-label={`${l.name} auswählen`}
                  checked={selected.includes(l.id)}
                  onChange={(e) =>
                    setSelected(
                      e.target.checked ? [...selected, l.id] : selected.filter((id) => id !== l.id),
                    )
                  }
                />
                <strong>{l.name}</strong>
              </label>
              <div className="actions">
                <Button
                  onClick={() =>
                    void a.run(async () => {
                      await api('/liquids/' + l.id + '/restore', 'POST', { version: l.version });
                      reload();
                    })
                  }
                >
                  Wiederherstellen
                </Button>
                <Button variant="danger" onClick={() => setPermanent(l)}>
                  Endgültig löschen
                </Button>
              </div>
            </div>
          ))}
        </div>
        {(page > 1 || (trash.data?.total || 0) > 24) && (
          <div className="pagination">
            <Button
              disabled={page === 1}
              onClick={() => {
                setPage(page - 1);
                setSelected([]);
              }}
            >
              Zurück
            </Button>
            <span>Seite {page}</span>
            <Button
              disabled={!trash.data || page * 24 >= trash.data.total}
              onClick={() => {
                setPage(page + 1);
                setSelected([]);
              }}
            >
              Weiter
            </Button>
          </div>
        )}
      </Panel>
      {preview && (
        <Dialog
          title="Import prüfen"
          onClose={() => setPreview(undefined)}
          footer={
            <>
              <Button onClick={() => setPreview(undefined)}>Abbrechen</Button>
              <Button
                variant="primary"
                disabled={a.busy}
                onClick={() =>
                  void a.run(async () => {
                    const result = await api<{ liquids?: number }>('/imports', 'POST', {
                      token: preview.token,
                      confirm: true,
                    });
                    setPreview(undefined);
                    await refresh();
                    setNotice('Import abgeschlossen.');
                    reload();
                  })
                }
              >
                Importieren
              </Button>
            </>
          }
        >
          <dl className="details-grid">
            {Object.entries(preview.counts).map(([key, value]) => (
              <div key={key}>
                <dt>{{ liquids: 'Liquids', fields: 'Felder', media: 'Bilder' }[key] || key}</dt>
                <dd>{value}</dd>
              </div>
            ))}
          </dl>
          {preview.warnings?.length > 0 && (
            <ul>
              {preview.warnings.map((w, i) => (
                <li key={i}>{w}</li>
              ))}
            </ul>
          )}
          <ErrorMessage error={a.error} />
        </Dialog>
      )}
      {permanent && (
        <Confirm
          title="Endgültig löschen?"
          danger
          onCancel={() => setPermanent(undefined)}
          onConfirm={() =>
            void a.run(async () => {
              await api('/liquids/' + permanent.id + '?permanent=true', 'DELETE', {
                version: permanent.version,
                confirm: true,
              });
              setPermanent(undefined);
              reload();
            })
          }
        >
          <p>„{permanent.name}“ wird dauerhaft entfernt.</p>
          <ErrorMessage error={a.error} />
        </Confirm>
      )}
    </>
  );
}
