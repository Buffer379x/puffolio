import { useState } from 'react';
import { api } from '../../app/api';
import { useApp, useAction, useDirty } from '../../app/state';
import type { Field, FieldType, Scalar } from '../../app/types';
import { FieldInput } from '../../components/FieldInput';
import {
  Button,
  IconButton,
  Dialog,
  ErrorMessage,
  Confirm,
  HelpPopover,
} from '../../components/ui';
import { canonicalNumber, saveBlob } from './values';
import { download } from '../../app/api';
export const fieldTypes: Record<FieldType, string> = {
  text: 'Text',
  longtext: 'Langer Text',
  rating: 'Sternebewertung',
  image: 'Bild',
  tags: 'Tags',
  color: 'Farbe',
  date: 'Datum',
  number: 'Zahl',
  select: 'Auswahl',
  boolean: 'Ja/Nein',
};
const blank = (): Field => ({
  id: '',
  label: '',
  type: 'text',
  position: 0,
  version: 0,
  required: false,
  archived: false,
  visibility: { form: true, detail: true, list: true, grid: true },
  config: { scale: 0, stars: 5, filterable: true, searchable: true },
});
function FieldDialog({
  field,
  onClose,
  onSaved,
}: {
  field?: Field;
  onClose: () => void;
  onSaved: () => void;
}) {
  const [draft, setDraft] = useState<Field>(() => structuredClone(field || blank()));
  const [policy, setPolicy] = useState('new');
  const [backfill, setBackfill] = useState<Scalar>(null);
  const [defaultEnabled, setDefaultEnabled] = useState(
    field?.config.defaultValue !== undefined && field.config.defaultValue !== null,
  );
  const [confirm, setConfirm] = useState(false);
  const a = useAction();
  const dirty =
    JSON.stringify(draft) !== JSON.stringify(field || blank()) ||
    defaultEnabled !==
      (field?.config.defaultValue !== undefined && field.config.defaultValue !== null) ||
    policy !== 'new' ||
    backfill !== null;
  useDirty(dirty);
  const close = () => (dirty ? setConfirm(true) : onClose());
  const config = (patch: Partial<Field['config']>) =>
    setDraft({ ...draft, config: { ...draft.config, ...patch } });
  const scalar = !['image', 'tags'].includes(draft.type);
  const convert = (v: Scalar) =>
    draft.type === 'number' && typeof v === 'string'
      ? canonicalNumber(v, draft.config.scale || 0)
      : v;
  return (
    <>
      <Dialog
        title={field ? 'Feld bearbeiten' : 'Feld hinzufügen'}
        onClose={close}
        footer={
          <>
            <Button onClick={close}>Abbrechen</Button>
            <Button
              variant="primary"
              disabled={a.busy || !draft.label.trim()}
              onClick={() =>
                void a.run(async () => {
                  const d = structuredClone(draft);
                  if (!defaultEnabled) delete d.config.defaultValue;
                  else d.config.defaultValue = convert(d.config.defaultValue ?? null);
                  if (d.type === 'number') {
                    if (d.config.min)
                      d.config.min = canonicalNumber(d.config.min, d.config.scale || 0);
                    if (d.config.max)
                      d.config.max = canonicalNumber(d.config.max, d.config.scale || 0);
                  }
                  await api(field ? '/fields/' + field.id : '/fields', field ? 'PATCH' : 'POST', {
                    ...d,
                    ...(!field && d.required && policy === 'backfill'
                      ? { backfillValue: convert(backfill) }
                      : {}),
                  });
                  onSaved();
                })
              }
            >
              Speichern
            </Button>
          </>
        }
      >
        <div className="form-grid">
          <label className="form-field full">
            Feldname
            <input
              autoFocus
              aria-label="Feldname"
              value={draft.label}
              maxLength={100}
              onChange={(e) => setDraft({ ...draft, label: e.target.value })}
            />
          </label>
          <label className="form-field">
            Typ
            <select
              aria-label="Feldtyp"
              value={draft.type}
              disabled={!!field}
              onChange={(e) => {
                const type = e.target.value as FieldType;
                setDraft({
                  ...draft,
                  type,
                  config: {
                    scale: 0,
                    stars: 5,
                    filterable: true,
                    searchable: ['text', 'longtext', 'tags'].includes(type),
                  },
                });
                setDefaultEnabled(false);
                setBackfill(null);
                setPolicy('new');
              }}
            >
              {Object.entries(fieldTypes).map(([id, label]) => (
                <option key={id} value={id}>
                  {label}
                </option>
              ))}
            </select>
          </label>
          <label className="check-row">
            <input
              type="checkbox"
              checked={draft.required}
              onChange={(e) =>
                setDraft({
                  ...draft,
                  required: e.target.checked,
                  visibility: {
                    ...draft.visibility,
                    form: e.target.checked || draft.visibility.form,
                  },
                })
              }
            />
            Pflichtfeld
          </label>
          <HelpPopover label="Pflichtfelder">
            <p>
              Neue Pflichtfelder gelten für neue Liquids. Vorhandene Einträge bleiben mit „Nur neue
              Liquids“ bearbeitbar. „Vorhandene auffüllen“ schreibt den ausdrücklich gewählten Wert
              in alle bisherigen Einträge.
            </p>
          </HelpPopover>
          {draft.required && !field && (
            <>
              <label className="form-field full">
                Vorhandene Liquids
                <select
                  aria-label="Pflichtfeld für vorhandene Liquids"
                  value={policy}
                  onChange={(e) => setPolicy(e.target.value)}
                >
                  <option value="new">Nur neue Liquids</option>
                  {scalar && <option value="backfill">Vorhandene auffüllen</option>}
                </select>
              </label>
              {policy === 'backfill' && scalar && (
                <FieldInput
                  field={{ ...draft, label: 'Auffüllwert', required: true }}
                  value={backfill}
                  onChange={setBackfill}
                />
              )}
            </>
          )}
          <fieldset className="full">
            <legend>Sichtbarkeit</legend>
            <div className="visibility-grid">
              {(['form', 'detail', 'list', 'grid'] as const).map((key, i) => (
                <label className="check-row" key={key}>
                  <input
                    type="checkbox"
                    checked={draft.visibility[key]}
                    disabled={key === 'form' && draft.required}
                    onChange={(e) =>
                      setDraft({
                        ...draft,
                        visibility: { ...draft.visibility, [key]: e.target.checked },
                      })
                    }
                  />
                  {['Formular', 'Detail', 'Liste', 'Kachel'][i]}
                </label>
              ))}
            </div>
          </fieldset>
          {['text', 'longtext', 'tags'].includes(draft.type) && (
            <label className="check-row">
              <input
                type="checkbox"
                checked={!!draft.config.searchable}
                onChange={(e) => config({ searchable: e.target.checked })}
              />
              Durchsuchbar
            </label>
          )}
          <label className="check-row">
            <input
              type="checkbox"
              checked={!!draft.config.filterable}
              onChange={(e) => config({ filterable: e.target.checked })}
            />
            Filterbar
          </label>
          {draft.type === 'number' && (
            <>
              <label className="form-field">
                Nachkommastellen
                <select
                  aria-label="Nachkommastellen"
                  value={draft.config.scale || 0}
                  onChange={(e) => config({ scale: Number(e.target.value) })}
                >
                  {[0, 1, 2, 3].map((v) => (
                    <option key={v}>{v}</option>
                  ))}
                </select>
              </label>
              <label className="form-field">
                Einheit
                <input
                  aria-label="Einheit"
                  value={draft.config.unit || ''}
                  onChange={(e) => config({ unit: e.target.value })}
                  maxLength={30}
                />
              </label>
            </>
          )}
          {['number', 'date'].includes(draft.type) && (
            <>
              {(['min', 'max'] as const).map((key, i) => (
                <label className="form-field" key={key}>
                  {i ? 'Maximum' : 'Minimum'}
                  <input
                    aria-label={i ? 'Maximum' : 'Minimum'}
                    type={draft.type === 'date' ? 'date' : 'text'}
                    inputMode={draft.type === 'number' ? 'decimal' : undefined}
                    value={draft.config[key] || ''}
                    onChange={(e) => config({ [key]: e.target.value })}
                  />
                </label>
              ))}
            </>
          )}
          {draft.type === 'rating' && (
            <label className="form-field">
              Sterne
              <select
                aria-label="Sterneskala"
                value={draft.config.stars || 5}
                onChange={(e) => config({ stars: Number(e.target.value) })}
              >
                {[3, 5, 10].map((n) => (
                  <option key={n}>{n}</option>
                ))}
              </select>
            </label>
          )}
          {draft.type === 'select' && (
            <fieldset className="full">
              <legend>Optionen</legend>
              {draft.config.options?.map((o, i) => (
                <div className="option-row" key={o.id}>
                  <input
                    aria-label={`Option ${i + 1}`}
                    value={o.label}
                    onChange={(e) =>
                      config({
                        options: draft.config.options?.map((x) =>
                          x.id === o.id ? { ...x, label: e.target.value } : x,
                        ),
                      })
                    }
                  />
                  <label className="check-row">
                    <input
                      type="checkbox"
                      checked={!!o.archived}
                      onChange={(e) =>
                        config({
                          options: draft.config.options?.map((x) =>
                            x.id === o.id ? { ...x, archived: e.target.checked } : x,
                          ),
                        })
                      }
                    />
                    Archiviert
                  </label>
                  <IconButton
                    label={`Option ${i + 1} entfernen`}
                    icon="close"
                    onClick={() =>
                      config({ options: draft.config.options?.filter((x) => x.id !== o.id) })
                    }
                  />
                </div>
              ))}
              <Button
                onClick={() =>
                  config({
                    options: [
                      ...(draft.config.options || []),
                      { id: crypto.randomUUID(), label: '' },
                    ],
                  })
                }
              >
                Option hinzufügen
              </Button>
            </fieldset>
          )}
          {draft.type === 'date' && (
            <label className="check-row full">
              <input
                type="checkbox"
                checked={!!draft.config.defaultToday}
                onChange={(e) => {
                  config({ defaultToday: e.target.checked, defaultValue: null });
                  setDefaultEnabled(false);
                }}
              />
              Heute beim Anlegen
            </label>
          )}
          {(scalar || !!field) && !draft.config.defaultToday && (
            <>
              <label className="check-row full">
                <input
                  type="checkbox"
                  checked={defaultEnabled}
                  onChange={(e) => setDefaultEnabled(e.target.checked)}
                />
                Standardwert für neue Liquids
              </label>
              {defaultEnabled && (
                <FieldInput
                  field={{ ...draft, label: 'Standardwert', required: false }}
                  value={draft.config.defaultValue ?? null}
                  onChange={(v) => config({ defaultValue: v })}
                />
              )}
            </>
          )}
        </div>
        <ErrorMessage error={a.error} />
      </Dialog>
      {confirm && (
        <Confirm
          title="Änderungen verwerfen?"
          onCancel={() => setConfirm(false)}
          onConfirm={onClose}
        >
          <p>Ungespeicherte Änderungen gehen verloren.</p>
        </Confirm>
      )}
    </>
  );
}
export function FieldsSettings() {
  const { fields, schemaVersion, preferences, savePreferences, refresh } = useApp();
  const [edit, setEdit] = useState<{ field?: Field }>();
  const [deletion, setDeletion] = useState<{ field: Field; values: number }>();
  const a = useAction();
  const done = () =>
    void a.run(async () => {
      await refresh();
      setEdit(undefined);
    });
  const move = (i: number, dir: number) =>
    void a.run(async () => {
      const ids = fields.map((f) => f.id);
      [ids[i], ids[i + dir]] = [ids[i + dir], ids[i]];
      await api('/fields/order', 'PUT', { ids, version: schemaVersion });
      await refresh();
    });
  return (
    <>
      <div className="settings-section-heading">
        <h2>Felder</h2>
        <Button variant="primary" onClick={() => setEdit({})}>
          Feld hinzufügen
        </Button>
      </div>
      <ErrorMessage error={a.error} />
      <div className="field-list">
        {fields.map((f, i) => (
          <div className="setting-line" key={f.id}>
            <div>
              <strong>{f.label}</strong>
              <span className="muted">
                {fieldTypes[f.type]}
                {f.archived ? ' · Archiviert' : ''}
                {f.required ? ' · Pflichtfeld' : ''}
              </span>
            </div>
            <div className="actions">
              <IconButton
                icon="arrow"
                className="arrow-up"
                label={`${f.label} nach oben`}
                disabled={i === 0}
                onClick={() => move(i, -1)}
              />
              <IconButton
                icon="arrow"
                className="arrow-down"
                label={`${f.label} nach unten`}
                disabled={i === fields.length - 1}
                onClick={() => move(i, 1)}
              />
              <Button onClick={() => setEdit({ field: f })}>Bearbeiten</Button>
              <Button
                onClick={() =>
                  void a.run(async () => {
                    await api('/fields/' + f.id + '/archive', 'POST', {
                      version: f.version,
                      archived: !f.archived,
                    });
                    await refresh();
                  })
                }
              >
                {f.archived ? 'Aktivieren' : 'Archivieren'}
              </Button>
              <IconButton
                label={`${f.label} löschen`}
                icon="trash"
                onClick={() =>
                  void a.run(async () => {
                    const usage = await api<{ values: number }>('/fields/' + f.id + '/usage');
                    setDeletion({ field: f, values: usage.values });
                  })
                }
              />
            </div>
          </div>
        ))}
      </div>
      <section className="section-card">
        <h2>Feldzuordnungen</h2>
        <div className="form-grid">
          {(
            [
              ['image', 'Vorschaubild', 'image'],
              ['rating', 'Hauptbewertung', 'rating'],
              ['quantity', 'Menge', 'number'],
              ['color', 'Kachelfarbe', 'color'],
            ] as const
          ).map(([role, label, type]) => (
            <label className="form-field" key={role}>
              {label}
              <select
                aria-label={label}
                value={preferences.roles[role] || ''}
                onChange={(e) =>
                  void a.run(() =>
                    savePreferences({ roles: { ...preferences.roles, [role]: e.target.value } }),
                  )
                }
              >
                <option value="">Nicht zugeordnet</option>
                {fields
                  .filter((f) => !f.archived && f.type === type)
                  .map((f) => (
                    <option key={f.id} value={f.id}>
                      {f.label}
                    </option>
                  ))}
              </select>
            </label>
          ))}
        </div>
      </section>
      {edit && <FieldDialog field={edit.field} onClose={() => setEdit(undefined)} onSaved={done} />}
      {deletion && (
        <Confirm
          title="Feld endgültig löschen?"
          danger
          onCancel={() => setDeletion(undefined)}
          onConfirm={() =>
            void a.run(async () => {
              await api('/fields/' + deletion.field.id, 'DELETE', {
                version: deletion.field.version,
                confirm: true,
              });
              setDeletion(undefined);
              await refresh();
            })
          }
        >
          <p>
            {deletion.field.label}: {deletion.values} Liquids mit Werten. Diese Werte werden
            endgültig gelöscht.
          </p>
          <Button
            onClick={() =>
              void a.run(async () =>
                saveBlob(await download('/exports', {}), 'puffolio-sammlung.zip'),
              )
            }
          >
            Vorher Sammlung exportieren
          </Button>
          <ErrorMessage error={a.error} />
        </Confirm>
      )}
    </>
  );
}
