import { createUUID } from '../../app/uuid';
import { useRef, useState } from 'react';
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
  Icon,
} from '../../components/ui';
import { useToast } from '../../components/toast';
import { CardPreview } from './CardPreview';
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
  onDelete,
}: {
  field?: Field;
  onClose: () => void;
  onSaved: () => void;
  onDelete: (field: Field, values: number) => void;
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
            {field && (
              <div className="push-left modal-footer-side">
                <Button
                  disabled={a.busy || dirty}
                  onClick={() =>
                    void a.run(async () => {
                      await api('/fields/' + field.id + '/archive', 'POST', {
                        version: field.version,
                        archived: !field.archived,
                      });
                      onSaved();
                    })
                  }
                >
                  {field.archived ? 'Aktivieren' : 'Archivieren'}
                </Button>
                <Button
                  variant="danger"
                  disabled={a.busy || dirty}
                  onClick={() =>
                    void a.run(async () => {
                      const usage = await api<{ values: number }>(
                        '/fields/' + field.id + '/usage',
                      );
                      onDelete(field, usage.values);
                    })
                  }
                >
                  Löschen
                </Button>
              </div>
            )}
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
                        visibility: {
                          ...draft.visibility,
                          [key]: e.target.checked,
                        },
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
                      config({
                        options: draft.config.options?.filter((x) => x.id !== o.id),
                      })
                    }
                  />
                </div>
              ))}
              <Button
                onClick={() =>
                  config({
                    options: [...(draft.config.options || []), { id: createUUID(), label: '' }],
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
                  config({
                    defaultToday: e.target.checked,
                    defaultValue: null,
                  });
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
        {field && dirty && (
          <p className="muted">Erst speichern oder verwerfen, dann archivieren oder löschen.</p>
        )}
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
const typeIcons: Record<FieldType, string> = {
  text: 'list',
  longtext: 'list',
  rating: 'star',
  image: 'image',
  tags: 'tag',
  color: 'liquids',
  date: 'calendar',
  number: 'box',
  select: 'filter',
  boolean: 'check',
};
const contexts = [
  ['form', 'Formular'],
  ['detail', 'Detail'],
  ['list', 'Liste'],
  ['grid', 'Kachel'],
] as const;
function ShelfLifeSetting() {
  const { fields, preferences, savePreferences } = useApp();
  const toast = useToast();
  const stored = preferences.shelfLifeMonths || 0;
  const yearly = stored % 12 === 0;
  const [amount, setAmount] = useState(stored ? String(yearly ? stored / 12 : stored) : '');
  const [unit, setUnit] = useState(yearly ? 'years' : 'months');
  const a = useAction();
  const months = Number(amount || 0) * (unit === 'years' ? 12 : 1);
  const valid = Number.isInteger(months) && months >= 0 && months <= 1200;
  const role = (key: 'purchase' | 'shelfOverride', id: string) =>
    void a.run(() => savePreferences({ roles: { ...preferences.roles, [key]: id } }));
  const statusOptions = fields
    .filter((f) => !f.archived && f.type === 'select')
    .flatMap((f) =>
      (f.config.options || []).map((o) => ({
        value: f.id + '|' + o.id,
        label: `${f.label}: ${o.label}`,
      })),
    );
  const consumed = preferences.consumedOption
    ? preferences.roles.status + '|' + preferences.consumedOption
    : '';
  return (
    <section className="section-card">
      <h2>Haltbarkeit</h2>
      <form
        className="shelf-form"
        onSubmit={(e) => {
          e.preventDefault();
          void a.run(async () => {
            await savePreferences({ shelfLifeMonths: months });
            toast('Gespeichert');
          });
        }}
      >
        <label className="form-field">
          Standardhaltedauer
          <input
            aria-label="Standardhaltedauer"
            inputMode="numeric"
            value={amount}
            aria-invalid={!valid}
            onChange={(e) => setAmount(e.target.value)}
          />
        </label>
        <label className="form-field">
          Einheit
          <select
            aria-label="Einheit der Haltedauer"
            value={unit}
            onChange={(e) => setUnit(e.target.value)}
          >
            <option value="years">Jahre</option>
            <option value="months">Monate</option>
          </select>
        </label>
        <Button type="submit" variant="primary" disabled={a.busy || !valid || months === stored}>
          Haltedauer speichern
        </Button>
      </form>
      <div className="form-grid shelf-roles">
        <label className="form-field">
          Läuft ab
          <select
            aria-label="Datumsfeld der Haltedauer"
            value={preferences.roles.purchase || ''}
            onChange={(e) => role('purchase', e.target.value)}
          >
            <option value="">Nicht zugeordnet</option>
            {fields
              .filter((f) => !f.archived && f.type === 'date')
              .map((f) => (
                <option key={f.id} value={f.id}>
                  {f.label}
                </option>
              ))}
          </select>
        </label>
        <label className="form-field">
          Eigene Haltedauer je Liquid
          <select
            aria-label="Zahlenfeld für eigene Haltedauer"
            value={preferences.roles.shelfOverride || ''}
            onChange={(e) => role('shelfOverride', e.target.value)}
          >
            <option value="">Nicht zugeordnet</option>
            {fields
              .filter((f) => !f.archived && f.type === 'number')
              .map((f) => (
                <option key={f.id} value={f.id}>
                  {f.label}
                  {f.config.unit ? ` (${f.config.unit})` : ''}
                </option>
              ))}
          </select>
        </label>
        <label className="form-field">
          Gilt als aufgebraucht
          <select
            aria-label="Status für aufgebraucht"
            value={consumed}
            onChange={(e) => {
              const [status, option] = e.target.value.split('|');
              void a.run(() =>
                savePreferences({
                  roles: { ...preferences.roles, status: status || '' },
                  consumedOption: option || '',
                }),
              );
            }}
          >
            <option value="">Nicht zugeordnet</option>
            {statusOptions.map((o) => (
              <option key={o.value} value={o.value}>
                {o.label}
              </option>
            ))}
          </select>
        </label>
      </div>
      <ErrorMessage error={a.error} />
    </section>
  );
}
export function FieldsSettings() {
  const { fields, schemaVersion, preferences, savePreferences, refresh } = useApp();
  const [edit, setEdit] = useState<{ field?: Field }>();
  const [deletion, setDeletion] = useState<{ field: Field; values: number }>();
  const a = useAction();
  const addButton = useRef<HTMLButtonElement>(null);
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
        <Button ref={addButton} variant="primary" onClick={() => setEdit({})}>
          <Icon name="plus" />
          Feld hinzufügen
        </Button>
      </div>
      <ErrorMessage error={a.error} />
      <div className="fields-workspace">
        <div className="fields-workspace-grid">
          <div className="table-scroll" tabIndex={0} aria-label="Felder">
            <table className="field-table">
              <thead>
                <tr>
                  <th>Feld</th>
                  {contexts.map(([key, label]) => (
                    <th className="check-col" key={key}>
                      {label}
                    </th>
                  ))}
                  <th className="actions-column">Aktionen</th>
                </tr>
              </thead>
              <tbody>
                {fields.map((f, i) => (
                  <tr key={f.id} className={f.archived ? 'archived' : ''}>
                    <th scope="row">
                      <div className="field-name">
                        <span className="field-type-icon">
                          <Icon name={typeIcons[f.type]} />
                        </span>
                        <div>
                          <strong>{f.label}</strong>
                          <span className="muted">
                            {fieldTypes[f.type]}
                            {f.archived ? ' · Archiviert' : ''}
                            {f.required ? ' · Pflichtfeld' : ''}
                          </span>
                        </div>
                      </div>
                    </th>
                    {contexts.map(([key, label]) => (
                      <td className="check-col" key={key}>
                        <label className="check-row">
                          <input
                            type="checkbox"
                            aria-label={`${f.label}: ${label}`}
                            checked={f.visibility[key]}
                            disabled={a.busy || f.archived || (key === 'form' && f.required)}
                            onChange={(e) =>
                              void a.run(async () => {
                                await api('/fields/' + f.id, 'PATCH', {
                                  version: f.version,
                                  visibility: { ...f.visibility, [key]: e.target.checked },
                                });
                                await refresh();
                              })
                            }
                          />
                        </label>
                      </td>
                    ))}
                    <td className="actions-column">
                      <div className="actions table-actions">
                        <IconButton
                          icon="up"
                          label={`${f.label} nach oben`}
                          disabled={i === 0}
                          onClick={() => move(i, -1)}
                        />
                        <IconButton
                          icon="down"
                          label={`${f.label} nach unten`}
                          disabled={i === fields.length - 1}
                          onClick={() => move(i, 1)}
                        />
                        <IconButton
                          icon="edit"
                          label={`${f.label} bearbeiten`}
                          onClick={() => setEdit({ field: f })}
                        />
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          <CardPreview />
        </div>
      </div>
      <ShelfLifeSetting key={preferences.shelfLifeMonths} />
      <section className="section-card">
        <h2>Feldzuordnungen</h2>
        <div className="form-grid">
          {(
            [
              ['image', 'Vorschaubild', 'image'],
              ['rating', 'Hauptbewertung', 'rating'],
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
                    savePreferences({
                      roles: { ...preferences.roles, [role]: e.target.value },
                    }),
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
      {edit && (
        <FieldDialog
          field={edit.field}
          onClose={() => setEdit(undefined)}
          onSaved={done}
          onDelete={(field, values) => {
            setEdit(undefined);
            setDeletion({ field, values });
          }}
        />
      )}
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
              // The row that held the opener is gone.
              addButton.current?.focus();
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
