import { useState } from 'react';
import { api } from '../../app/api';
import { useAction, useApp, useCanEdit } from '../../app/state';
import type { Preferences, SavedView } from '../../app/types';
import { useDraftGuard } from '../../components/useDraftGuard';
import { Button, Dialog, ErrorMessage } from '../../components/ui';
import type { ViewState } from './viewState';
const shape = (p: Preferences) =>
  JSON.stringify({ images: p.images, listFields: p.listFields, gridFields: p.gridFields });
/** Everything about how the collection is displayed, in one place. */
export function ViewDialog({
  query,
  views,
  onViewsChanged,
  onApply,
  onClose,
}: {
  query: ViewState;
  views: SavedView[];
  onViewsChanged: () => void;
  onApply: (view: SavedView) => void;
  onClose: () => void;
}) {
  const { fields, preferences, savePreferences } = useApp();
  const [draft, setDraft] = useState(preferences);
  const [initial] = useState(() => shape(preferences));
  const [name, setName] = useState('');
  const guard = useDraftGuard(shape(draft) !== initial, onClose);
  const a = useAction();
  const canEdit = useCanEdit();
  return (
    <Dialog
      title="Ansicht"
      onClose={guard.close}
      footer={
        <>
          <Button onClick={guard.close}>Abbrechen</Button>
          <Button
            variant="primary"
            disabled={a.busy}
            onClick={() =>
              void a.run(async () => {
                await savePreferences({
                  images: draft.images,
                  listFields: draft.listFields,
                  gridFields: draft.gridFields,
                });
                onClose();
              })
            }
          >
            Speichern
          </Button>
        </>
      }
    >
      {guard.confirmation}
      <label className="check-row">
        <input
          type="checkbox"
          checked={draft.images}
          onChange={(e) => setDraft({ ...draft, images: e.target.checked })}
        />
        Bilder anzeigen
      </label>
      {(['grid', 'list'] as const).map((view) => {
        const key = view === 'list' ? 'listFields' : 'gridFields';
        return (
          <fieldset key={view}>
            <legend>{view === 'list' ? 'Listenspalten' : 'Kachelfelder'}</legend>
            <div className="field-choice">
              {fields
                .filter((f) => !f.archived && f.visibility[view])
                .map((f) => (
                  <label className="check-row" key={f.id}>
                    <input
                      type="checkbox"
                      checked={draft[key].includes(f.id)}
                      onChange={(e) =>
                        setDraft({
                          ...draft,
                          [key]: e.target.checked
                            ? [...draft[key], f.id]
                            : draft[key].filter((id) => id !== f.id),
                        })
                      }
                    />
                    {f.label}
                  </label>
                ))}
            </div>
          </fieldset>
        );
      })}
      {canEdit && (
        <section className="dialog-section">
          <h3>Gespeicherte Ansichten</h3>
          {views.map((v) => (
            <div className="setting-line" key={v.id}>
              <span>{v.name}</span>
              <div className="actions">
                <Button
                  onClick={() => {
                    onApply(v);
                    onClose();
                  }}
                >
                  Anwenden
                </Button>
                <Button
                  variant="danger"
                  aria-label={`${v.name} löschen`}
                  onClick={() =>
                    void a.run(async () => {
                      await api('/saved-views/' + v.id, 'DELETE', { version: v.version });
                      onViewsChanged();
                    })
                  }
                >
                  Löschen
                </Button>
              </div>
            </div>
          ))}
          <div className="inline-form">
            <input
              aria-label="Ansichtsname"
              placeholder="Name"
              value={name}
              maxLength={100}
              onChange={(e) => setName(e.target.value)}
            />
            <Button
              disabled={a.busy || !name.trim()}
              onClick={() =>
                void a.run(async () => {
                  await api('/saved-views', 'POST', {
                    name,
                    search: query.search,
                    favorite: query.favorite,
                    filters: query.filters,
                    sort: query.sort,
                    direction: query.direction,
                  });
                  setName('');
                  onViewsChanged();
                })
              }
            >
              Aktuelle Suche speichern
            </Button>
          </div>
        </section>
      )}
      <ErrorMessage error={a.error} />
    </Dialog>
  );
}
