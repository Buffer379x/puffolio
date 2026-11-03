import { useDraftGuard } from '../../components/useDraftGuard';
import { useState, useEffect } from 'react';
import { api, download } from '../../app/api';
import { useApp, useLoad, useAction } from '../../app/state';
import type { Liquid, Page, SavedView, Filter, Field } from '../../app/types';
import { readView, writeView, type ViewState } from './viewState';
import { useTags, normalizeFilter } from './data';
import { LiquidEditor } from './LiquidEditor';
import { LiquidDetail } from './LiquidDetail';
import { FilterBuilder } from './FilterBuilder';
import { FieldValue } from '../../components/FieldValue';
import { TagsInput } from '../../components/FieldInput';
import { saveBlob } from '../fields/values';
import {
  Button,
  IconButton,
  PageHeader,
  SegmentedControl,
  Dialog,
  ErrorMessage,
  Empty,
  ItemCard,
  Icon,
  Confirm,
} from '../../components/ui';
const fieldColumn = (field: Field) =>
  'field-cell cell-' + field.type + (field.type === 'number' ? ' numeric' : '');
export function LiquidsPage({ query }: { query: string }) {
  const { fields, preferences, savePreferences, navigate } = useApp();
  const q = readView(query);
  if (!new URLSearchParams(query).has('view')) q.view = preferences.view;
  const [search, setSearch] = useState(q.search);
  const [editing, setEditing] = useState<{ liquid?: Liquid; duplicate?: boolean }>();
  const [detail, setDetail] = useState<Liquid>();
  const [filters, setFilters] = useState<Filter>();
  const [filterOpen, setFilterOpen] = useState(false);
  const [viewOpen, setViewOpen] = useState(false);
  const [viewDraft, setViewDraft] = useState(preferences);
  const viewState = (p: typeof preferences) =>
    JSON.stringify({ images: p.images, listFields: p.listFields, gridFields: p.gridFields });
  const [viewInitial, setViewInitial] = useState(() => viewState(preferences));
  const viewGuard = useDraftGuard(viewOpen && viewState(viewDraft) !== viewInitial, () =>
    setViewOpen(false),
  );
  const [selected, setSelected] = useState<string[]>([]);
  const [bulk, setBulk] = useState<'trash' | 'tag'>();
  const [tagField, setTagField] = useState('');
  const [tagIDs, setTagIDs] = useState<string[]>([]);
  const [saveView, setSaveView] = useState(false);
  const [viewName, setViewName] = useState('');
  const a = useAction();
  const tags = useTags();
  const views = useLoad(
    (s) => api<{ items: SavedView[] }>('/saved-views', 'GET', undefined, s),
    [],
  );
  const request = writeView(q);
  const list = useLoad(
    (s) => api<Page<Liquid>>('/liquids' + request, 'GET', undefined, s),
    [request],
  );
  const update = (patch: Partial<ViewState>) =>
    navigate('/liquids' + writeView({ ...q, ...patch }));
  useEffect(() => setSearch(q.search), [q.search]);
  useEffect(() => {
    if (search === q.search) return;
    const timer = setTimeout(() => update({ search, page: 1 }), 250);
    return () => clearTimeout(timer);
  }, [search, q.search, request]);
  useEffect(() => {
    setSelected([]);
  }, [request]);
  const reload = () => {
    list.reload();
    setSelected([]);
  };
  const favorite = (l: Liquid) =>
    void a.run(async () => {
      await api('/liquids/' + l.id, 'PATCH', { version: l.version, favorite: !l.favorite });
      reload();
    });
  const shown = fields.filter(
    (f) =>
      !f.archived &&
      f.visibility[q.view] &&
      (preferences.images || f.type !== 'image') &&
      (q.view === 'grid' ? preferences.gridFields : preferences.listFields).includes(f.id),
  );
  const imageField = shown.find((f) => f.id === preferences.roles.image && !f.archived);
  const ratingField = shown.find((f) => f.id === preferences.roles.rating && !f.archived);
  const quantityField = shown.find((f) => f.id === preferences.roles.quantity && !f.archived);
  const colorField = shown.find((f) => f.id === preferences.roles.color && !f.archived);
  const image = (l: Liquid) => {
    const id = imageField && l.values[imageField.id]?.value;
    const color = colorField && l.values[colorField.id]?.value;
    return preferences.images && imageField ? (
      <div className="item-image-wrap">
        {id ? (
          <img className="item-image" src={'/api/v1/media/' + id} alt="" />
        ) : (
          <div
            className="item-placeholder"
            style={typeof color === 'string' ? { backgroundColor: color } : undefined}
          >
            <Icon name="liquids" />
          </div>
        )}
      </div>
    ) : null;
  };
  const choose = (id: string, checked: boolean) =>
    setSelected(checked ? [...selected, id] : selected.filter((x) => x !== id));
  return (
    <>
      <PageHeader
        title="Liquids"
        meta={
          list.data
            ? `${list.data.total} ${list.data.total === 1 ? 'Liquid' : 'Liquids'}`
            : undefined
        }
      >
        <Button variant="primary" onClick={() => setEditing({})}>
          <Icon name="plus" />
          Liquid hinzufügen
        </Button>
      </PageHeader>
      <div className="toolbar">
        <div className="toolbar-start">
          <input
            type="search"
            className="search-box"
            aria-label="Liquids suchen"
            placeholder="Liquids suchen"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            maxLength={1000}
          />
          <Button
            onClick={() => {
              setViewDraft(preferences);
              setViewInitial(viewState(preferences));
              setViewOpen(true);
            }}
          >
            Ansicht
          </Button>
          <Button
            onClick={() => {
              setFilters(q.filters);
              setFilterOpen(true);
            }}
          >
            Filter{q.filters ? ' aktiv' : ''}
          </Button>
        </div>
        <div className="toolbar-end">
          <select
            aria-label="Sortieren nach"
            value={q.sort}
            onChange={(e) => update({ sort: e.target.value, page: 1 })}
          >
            <option value="createdAt">Erstellt</option>
            <option value="updatedAt">Geändert</option>
            <option value="name">Name</option>
            <option value="favorite">Favorit</option>
            {fields
              .filter(
                (f) =>
                  !f.archived && ['text', 'number', 'rating', 'date', 'boolean'].includes(f.type),
              )
              .map((f) => (
                <option key={f.id} value={'field:' + f.id}>
                  {f.label}
                </option>
              ))}
          </select>
          <select
            aria-label="Sortierrichtung"
            value={q.direction}
            onChange={(e) => update({ direction: e.target.value })}
          >
            <option value="desc">Absteigend</option>
            <option value="asc">Aufsteigend</option>
          </select>
          <SegmentedControl
            label="Darstellung"
            value={q.view}
            options={[
              { value: 'grid', label: 'Kacheln' },
              { value: 'list', label: 'Liste' },
            ]}
            onChange={(v) => {
              update({ view: v });
              void a.run(() => savePreferences({ view: v }));
            }}
          />
        </div>
      </div>
      <div className="filter-bar">
        <Button
          aria-pressed={q.favorite}
          onClick={() => update({ favorite: !q.favorite, page: 1 })}
        >
          <Icon name="star" />
          Nur Favoriten
        </Button>
        {q.filters && (
          <Button onClick={() => update({ filters: undefined, page: 1 })}>
            Feldfilter entfernen
          </Button>
        )}
        {(q.favorite || q.filters || q.search) && (
          <Button
            onClick={() => {
              setSearch('');
              update({ favorite: false, filters: undefined, search: '', page: 1 });
            }}
          >
            Filter zurücksetzen
          </Button>
        )}
        <select
          aria-label="Gespeicherte Ansicht"
          value=""
          onChange={(e) => {
            const v = views.data?.items.find((v) => v.id === e.target.value);
            if (v)
              update({
                search: v.search,
                favorite: v.favorite,
                sort: v.sort,
                direction: v.direction,
                filters: v.filters,
                page: 1,
              });
          }}
        >
          <option value="">Gespeicherte Ansichten</option>
          {views.data?.items.map((v) => (
            <option key={v.id} value={v.id}>
              {v.name}
            </option>
          ))}
        </select>
        <Button onClick={() => setSaveView(true)}>Ansicht speichern</Button>
      </div>
      <ErrorMessage error={a.error || list.error || tags.error || views.error} />
      {list.error && <Button onClick={list.reload}>Erneut versuchen</Button>}
      {list.loading && <p role="status">Laden…</p>}
      {selected.length > 0 && (
        <div className="selection-bar">
          <strong>{selected.length} ausgewählt</strong>
          <Button onClick={() => setBulk('trash')}>In Papierkorb</Button>
          <Button
            disabled={!fields.some((f) => f.type === 'tags' && !f.archived)}
            onClick={() => {
              setTagField(fields.find((f) => f.type === 'tags' && !f.archived)?.id || '');
              setBulk('tag');
            }}
          >
            Tags zuweisen
          </Button>
          <Button
            onClick={() =>
              void a.run(async () =>
                saveBlob(await download('/exports', {}), 'puffolio-sammlung.zip'),
              )
            }
          >
            Gesamte Sammlung exportieren
          </Button>
          <Button onClick={() => setSelected([])}>Auswahl aufheben</Button>
        </div>
      )}
      {list.data && !list.loading && list.data.items.length === 0 ? (
        <Empty
          action={
            q.search || q.filters || q.favorite ? (
              <Button
                onClick={() => update({ search: '', favorite: false, filters: undefined, page: 1 })}
              >
                Filter zurücksetzen
              </Button>
            ) : (
              <Button variant="primary" onClick={() => setEditing({})}>
                Liquid hinzufügen
              </Button>
            )
          }
        >
          {q.search || q.filters || q.favorite ? 'Keine Liquids gefunden.' : 'Noch keine Liquids.'}
        </Empty>
      ) : q.view === 'grid' ? (
        <div className="liquid-grid">
          {list.data?.items.map((l) => (
            <ItemCard
              key={l.id}
              image={image(l)}
              title={
                <button className="name-button" onClick={() => setDetail(l)}>
                  {l.name}
                </button>
              }
              actions={
                <>
                  <label className="check-row">
                    <input
                      type="checkbox"
                      aria-label={`${l.name} auswählen`}
                      checked={selected.includes(l.id)}
                      onChange={(e) => choose(l.id, e.target.checked)}
                    />
                    Auswählen
                  </label>
                  <Button onClick={() => setEditing({ liquid: l })}>
                    <Icon name="edit" />
                    Bearbeiten
                  </Button>
                </>
              }
            >
              <div className="card-rating-row">
                {quantityField && (
                  <FieldValue field={quantityField} value={l.values[quantityField.id]?.value} />
                )}
                {ratingField && (
                  <FieldValue field={ratingField} value={l.values[ratingField.id]?.value} />
                )}
                <IconButton
                  label={`${l.favorite ? 'Favorit entfernen' : 'Als Favorit markieren'}: ${l.name}`}
                  icon="star"
                  aria-pressed={l.favorite}
                  onClick={() => favorite(l)}
                />
              </div>
              <dl className="card-fields">
                {shown
                  .filter(
                    (f) =>
                      f.id !== ratingField?.id &&
                      f.id !== quantityField?.id &&
                      f.id !== imageField?.id,
                  )
                  .map((f) => (
                    <div key={f.id}>
                      <dt>{f.label}</dt>
                      <dd>
                        <FieldValue field={f} value={l.values[f.id]?.value} tags={tags.data} />
                      </dd>
                    </div>
                  ))}
              </dl>
            </ItemCard>
          ))}
        </div>
      ) : (
        <div className="table-scroll" tabIndex={0} aria-label="Liquidtabelle">
          <table>
            <thead>
              <tr>
                <th className="checkbox-column">
                  <label className="check-row checkbox-target">
                    <input
                      type="checkbox"
                      aria-label="Alle auf dieser Seite auswählen"
                      checked={
                        !!list.data?.items.length && selected.length === list.data.items.length
                      }
                      onChange={(e) =>
                        setSelected(e.target.checked ? list.data?.items.map((l) => l.id) || [] : [])
                      }
                    />
                  </label>
                </th>
                <th className="name-column">Name</th>
                {imageField && <th>Bild</th>}
                {shown
                  .filter((f) => f.id !== imageField?.id)
                  .map((f) => (
                    <th key={f.id} className={fieldColumn(f)}>
                      {f.label}
                    </th>
                  ))}
                <th className="actions-column">Aktionen</th>
              </tr>
            </thead>
            <tbody>
              {list.data?.items.map((l) => (
                <tr key={l.id}>
                  <td className="checkbox-column">
                    <label className="check-row checkbox-target">
                      <input
                        type="checkbox"
                        aria-label={`${l.name} auswählen`}
                        checked={selected.includes(l.id)}
                        onChange={(e) => choose(l.id, e.target.checked)}
                      />
                    </label>
                  </td>
                  <th className="name-column" scope="row">
                    <button className="name-button" onClick={() => setDetail(l)}>
                      {l.name}
                    </button>
                  </th>
                  {imageField && <td className="table-image">{image(l)}</td>}
                  {shown
                    .filter((f) => f.id !== imageField?.id)
                    .map((f) => (
                      <td key={f.id} className={fieldColumn(f)}>
                        <FieldValue field={f} value={l.values[f.id]?.value} tags={tags.data} />
                      </td>
                    ))}
                  <td className="actions-column">
                    <div className="actions table-actions">
                      <IconButton
                        label={`${l.favorite ? 'Favorit entfernen' : 'Als Favorit markieren'}: ${l.name}`}
                        icon="star"
                        aria-pressed={l.favorite}
                        onClick={() => favorite(l)}
                      />
                      <IconButton
                        label={`${l.name} bearbeiten`}
                        icon="edit"
                        onClick={() => setEditing({ liquid: l })}
                      />
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
      <div className="pagination">
        <Button disabled={q.page <= 1} onClick={() => update({ page: q.page - 1 })}>
          Zurück
        </Button>
        <span>
          Seite {q.page} von {Math.max(1, Math.ceil((list.data?.total || 0) / q.pageSize))}
        </span>
        <Button
          disabled={!list.data || q.page * q.pageSize >= list.data.total}
          onClick={() => update({ page: q.page + 1 })}
        >
          Weiter
        </Button>
        <select
          aria-label="Liquids pro Seite"
          value={q.pageSize}
          onChange={(e) => update({ pageSize: Number(e.target.value), page: 1 })}
        >
          {[24, 48, 96].map((n) => (
            <option key={n} value={n}>
              {n} pro Seite
            </option>
          ))}
        </select>
      </div>
      {editing && (
        <LiquidEditor
          {...editing}
          onClose={() => {
            setEditing(undefined);
            reload();
          }}
          onSaved={() => {
            setEditing(undefined);
            reload();
          }}
        />
      )}
      {detail && (
        <LiquidDetail
          liquid={detail}
          tags={tags.data || []}
          onClose={() => setDetail(undefined)}
          onEdit={() => {
            setEditing({ liquid: detail });
            setDetail(undefined);
          }}
          onDuplicate={() => {
            setEditing({ liquid: detail, duplicate: true });
            setDetail(undefined);
          }}
          onChange={reload}
        />
      )}
      {filterOpen && (
        <Dialog
          title="Filter"
          onClose={() => setFilterOpen(false)}
          footer={
            <>
              <Button onClick={() => setFilterOpen(false)}>Abbrechen</Button>
              <Button
                variant="primary"
                onClick={() =>
                  void a.run(async () => {
                    update({ filters: normalizeFilter(filters, fields), page: 1 });
                    setFilterOpen(false);
                  })
                }
              >
                Übernehmen
              </Button>
            </>
          }
        >
          <FilterBuilder fields={fields} tags={tags.data} value={filters} onChange={setFilters} />
          <ErrorMessage error={a.error} />
        </Dialog>
      )}
      {viewOpen && (
        <Dialog
          title="Ansicht"
          onClose={viewGuard.close}
          footer={
            <>
              <Button onClick={viewGuard.close}>Abbrechen</Button>
              <Button
                variant="primary"
                disabled={a.busy}
                onClick={() =>
                  void a.run(async () => {
                    await savePreferences({
                      images: viewDraft.images,
                      listFields: viewDraft.listFields,
                      gridFields: viewDraft.gridFields,
                    });
                    setViewOpen(false);
                  })
                }
              >
                Speichern
              </Button>
            </>
          }
        >
          {viewGuard.confirmation}
          <label className="check-row">
            <input
              type="checkbox"
              checked={viewDraft.images}
              onChange={(e) => setViewDraft({ ...viewDraft, images: e.target.checked })}
            />
            Bilder anzeigen
          </label>
          {(['list', 'grid'] as const).map((view) => (
            <fieldset key={view}>
              <legend>{view === 'list' ? 'Listenspalten' : 'Kachelfelder'}</legend>
              {fields
                .filter((f) => !f.archived && f.visibility[view])
                .map((f) => {
                  const key = view === 'list' ? 'listFields' : 'gridFields';
                  return (
                    <label className="check-row" key={f.id}>
                      <input
                        type="checkbox"
                        checked={viewDraft[key].includes(f.id)}
                        onChange={(e) =>
                          setViewDraft({
                            ...viewDraft,
                            [key]: e.target.checked
                              ? [...viewDraft[key], f.id]
                              : viewDraft[key].filter((id) => id !== f.id),
                          })
                        }
                      />
                      {f.label}
                    </label>
                  );
                })}
            </fieldset>
          ))}
          <ErrorMessage error={a.error} />
        </Dialog>
      )}
      {saveView && (
        <Dialog
          title="Gespeicherte Ansichten"
          onClose={() => setSaveView(false)}
          footer={
            <>
              <Button onClick={() => setSaveView(false)}>Schließen</Button>
              <Button
                variant="primary"
                disabled={a.busy || !viewName.trim()}
                onClick={() =>
                  void a.run(async () => {
                    await api('/saved-views', 'POST', {
                      name: viewName,
                      search: q.search,
                      favorite: q.favorite,
                      filters: q.filters,
                      sort: q.sort,
                      direction: q.direction,
                    });
                    views.reload();
                    setViewName('');
                    setSaveView(false);
                  })
                }
              >
                Speichern
              </Button>
            </>
          }
        >
          <label>
            Name
            <input
              aria-label="Ansichtsname"
              value={viewName}
              onChange={(e) => setViewName(e.target.value)}
              maxLength={100}
            />
          </label>
          {views.data?.items.map((v) => (
            <div className="setting-line" key={v.id}>
              <span>{v.name}</span>
              <Button
                variant="danger"
                onClick={() =>
                  void a.run(async () => {
                    await api('/saved-views/' + v.id, 'DELETE', { version: v.version });
                    views.reload();
                  })
                }
              >
                Löschen
              </Button>
            </div>
          ))}
          <ErrorMessage error={a.error} />
        </Dialog>
      )}
      {bulk === 'trash' && (
        <Confirm
          title="Ausgewählte Liquids löschen?"
          danger
          onCancel={() => setBulk(undefined)}
          onConfirm={() =>
            void a.run(async () => {
              await api('/liquids/bulk', 'POST', { ids: selected, action: 'trash' });
              setBulk(undefined);
              reload();
            })
          }
        >
          <p>{selected.length} Liquids bleiben 30 Tage im Papierkorb.</p>
          <ErrorMessage error={a.error} />
        </Confirm>
      )}
      {bulk === 'tag' && (
        <Dialog
          title="Tags zuweisen"
          onClose={() => setBulk(undefined)}
          footer={
            <>
              <Button onClick={() => setBulk(undefined)}>Abbrechen</Button>
              <Button
                variant="primary"
                disabled={!tagIDs.length || a.busy}
                onClick={() =>
                  void a.run(async () => {
                    await api('/liquids/bulk', 'POST', {
                      ids: selected,
                      action: 'tag',
                      fieldId: tagField,
                      tagIds: tagIDs,
                    });
                    setBulk(undefined);
                    setTagIDs([]);
                    reload();
                  })
                }
              >
                Zuweisen
              </Button>
            </>
          }
        >
          <select
            aria-label="Tagfeld"
            value={tagField}
            onChange={(e) => {
              setTagField(e.target.value);
              setTagIDs([]);
            }}
          >
            {fields
              .filter((f) => f.type === 'tags' && !f.archived)
              .map((f) => (
                <option key={f.id} value={f.id}>
                  {f.label}
                </option>
              ))}
          </select>
          {fields.find((f) => f.id === tagField) && (
            <TagsInput
              field={fields.find((f) => f.id === tagField)!}
              value={tagIDs}
              onChange={(v) => setTagIDs(v as string[])}
            />
          )}
          <ErrorMessage error={a.error} />
        </Dialog>
      )}
    </>
  );
}
