import { useState, useEffect } from 'react';
import { api, download } from '../../app/api';
import { useApp, useLoad, useAction, useCanEdit } from '../../app/state';
import type { Liquid, Page, SavedView, Filter, Field } from '../../app/types';
import { readView, writeView, type ViewState } from './viewState';
import { useTags, normalizeFilter } from './data';
import { LiquidEditor } from './LiquidEditor';
import { LiquidDetail } from './LiquidDetail';
import { LiquidCard } from './LiquidCard';
import { cardLayout, shelfEnabled, shownFields } from './cardLayout';
import { LiquidTable } from './LiquidTable';
import { ViewDialog } from './ViewDialog';
import { FilterBuilder } from './FilterBuilder';
import { FilterChips } from './FilterChips';
import {
  countConditions,
  quickTags,
  withQuickTags,
  withoutQuickTags,
  withoutUnknownTags,
} from './filters';
import { TagsInput } from '../../components/FieldInput';
import { paletteColor } from '../../components/PalettePicker';
import { useToast } from '../../components/toast';
import { saveBlob } from '../fields/values';
import {
  Button,
  IconButton,
  PageHeader,
  SegmentedControl,
  Dialog,
  ErrorMessage,
  Empty,
  Icon,
  Confirm,
} from '../../components/ui';
/** Sorting by a name reads A to Z; sorting by anything measured leads with the highest. */
const defaultDirection = (sort: string, fields: Field[]) =>
  sort === 'name' ||
  sort === 'shelf' ||
  fields.find((f) => 'field:' + f.id === sort)?.type === 'text'
    ? 'asc'
    : 'desc';
export function LiquidsPage({ query }: { query: string }) {
  const { fields, preferences, savePreferences, navigate } = useApp();
  const toast = useToast();
  const canEdit = useCanEdit();
  const q = readView(query);
  if (!new URLSearchParams(query).has('view')) q.view = preferences.view;
  const [search, setSearch] = useState(q.search);
  const [editing, setEditing] = useState<{ liquid?: Liquid; duplicate?: boolean }>();
  const [detail, setDetail] = useState<Liquid>();
  const [filters, setFilters] = useState<Filter>();
  const [filterOpen, setFilterOpen] = useState(false);
  const [viewOpen, setViewOpen] = useState(false);
  const [selected, setSelected] = useState<string[]>([]);
  const [bulk, setBulk] = useState<'trash' | 'tag'>();
  const [tagField, setTagField] = useState('');
  const [tagIDs, setTagIDs] = useState<string[]>([]);
  const a = useAction();
  const tags = useTags();
  const views = useLoad(
    (s) =>
      canEdit
        ? api<{ items: SavedView[] }>('/saved-views', 'GET', undefined, s)
        : Promise.resolve({ items: [] as SavedView[] }),
    [canEdit],
  );
  const request = writeView(q);
  const list = useLoad(
    (s) => api<Page<Liquid>>('/liquids' + request, 'GET', undefined, s),
    [request],
  );
  const update = (patch: Partial<ViewState>) =>
    navigate('/liquids' + writeView({ ...q, ...patch }));
  useEffect(() => setSearch(q.search), [q.search]);
  // Replaces the address, so going back does not land on the stale link again.
  useEffect(() => {
    if (tags.loading || tags.error || !tags.data) return;
    const clean = withoutUnknownTags(q.filters, tags.data, fields);
    if (clean === q.filters) return;
    navigate('/liquids' + writeView({ ...q, filters: clean, page: 1 }), { replace: true });
    toast('Gelöschter Tag aus dem Filter entfernt');
  }, [tags.loading, tags.data, tags.error, request]);
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
  const replaceItem = (next: Liquid) => {
    if (list.data)
      list.setData({
        ...list.data,
        items: list.data.items.map((l) => (l.id === next.id ? next : l)),
      });
  };
  const favorite = (l: Liquid) =>
    void a.run(async () => {
      replaceItem({ ...l, favorite: !l.favorite });
      try {
        replaceItem(
          await api<Liquid>('/liquids/' + l.id, 'PATCH', {
            version: l.version,
            favorite: !l.favorite,
          }),
        );
      } catch (e) {
        replaceItem(l);
        throw e;
      }
      if (q.favorite) list.reload();
    });
  const trashed = (l: Liquid) =>
    toast({
      message: `„${l.name}“ im Papierkorb`,
      action: {
        label: 'Rückgängig',
        run: () =>
          void a.run(async () => {
            await api('/liquids/' + l.id + '/restore', 'POST', { version: l.version });
            reload();
          }),
      },
    });
  // Quick tag chips work on the first tag field; other tag fields use the filter dialog.
  const primaryTags = fields.find((f) => f.type === 'tags' && !f.archived);
  const activeTags = quickTags(q.filters, primaryTags?.id);
  const otherFilters = withoutQuickTags(q.filters, primaryTags?.id);
  const filterCount = countConditions(otherFilters);
  const narrowed = !!(q.search || q.filters || q.favorite);
  const reset = () => {
    setSearch('');
    update({ favorite: false, filters: undefined, search: '', page: 1 });
  };
  const shown = shownFields(fields, preferences, q.view);
  const imageRole = fields.find((f) => f.id === preferences.roles.image && !f.archived);
  const showImage = preferences.images && (!imageRole || shown.includes(imageRole));
  const layout = cardLayout(fields, preferences);
  const shelfOn = shelfEnabled(preferences);
  const items = list.data?.items || [];
  const sortOptions = fields.filter(
    (f) => !f.archived && ['text', 'number', 'rating', 'date', 'boolean'].includes(f.type),
  );
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
        {canEdit && (
          <Button variant="primary" onClick={() => setEditing({})}>
            <Icon name="plus" />
            Liquid hinzufügen
          </Button>
        )}
      </PageHeader>
      <div className="toolbar">
        <div className="toolbar-start">
          <label className="search-box">
            <Icon name="search" />
            <input
              type="search"
              aria-label="Liquids suchen"
              placeholder="Liquids suchen"
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              maxLength={1000}
            />
          </label>
          <Button onClick={() => setViewOpen(true)}>
            <Icon name="sliders" />
            Ansicht
          </Button>
          <Button
            onClick={() => {
              setFilters(otherFilters);
              setFilterOpen(true);
            }}
          >
            <Icon name="filter" />
            Filter{filterCount ? ` (${filterCount})` : ''}
          </Button>
        </div>
        <div className="toolbar-end">
          <select
            aria-label="Sortieren nach"
            value={q.sort}
            onChange={(e) =>
              update({
                sort: e.target.value,
                direction: defaultDirection(e.target.value, fields),
                page: 1,
              })
            }
          >
            <option value="createdAt">Zuletzt hinzugefügt</option>
            <option value="updatedAt">Zuletzt geändert</option>
            <option value="name">Name</option>
            <option value="favorite">Favorit</option>
            {shelfOn && <option value="shelf">Haltbarkeit</option>}
            {sortOptions.map((f) => (
              <option key={f.id} value={'field:' + f.id}>
                {f.label}
              </option>
            ))}
            {q.sort.startsWith('field:') &&
              !sortOptions.some((f) => 'field:' + f.id === q.sort) && (
                <option value={q.sort}>Feld nicht verfügbar</option>
              )}
          </select>
          <IconButton
            className="bordered"
            label={q.direction === 'asc' ? 'Aufsteigend sortiert' : 'Absteigend sortiert'}
            icon={q.direction === 'asc' ? 'sortAsc' : 'sortDesc'}
            onClick={() => update({ direction: q.direction === 'asc' ? 'desc' : 'asc' })}
          />
          <SegmentedControl
            label="Darstellung"
            value={q.view}
            options={[
              { value: 'grid', label: 'Kacheln', icon: 'grid' },
              { value: 'list', label: 'Liste', icon: 'list' },
            ]}
            onChange={(v) => {
              update({ view: v });
              void a.run(() => savePreferences({ view: v }));
            }}
          />
        </div>
      </div>
      <div className="filter-chips">
        <Button
          className="filter-chip"
          aria-pressed={q.favorite}
          onClick={() => update({ favorite: !q.favorite, page: 1 })}
        >
          <Icon name="heart" />
          Nur Favoriten
        </Button>
        {(tags.data || [])
          .filter((t) => t.fieldId === primaryTags?.id)
          .map((t) => (
            <Button
              key={t.id}
              className="filter-chip"
              aria-pressed={activeTags.includes(t.id)}
              onClick={() =>
                update({
                  filters: withQuickTags(
                    q.filters,
                    primaryTags?.id,
                    activeTags.includes(t.id)
                      ? activeTags.filter((id) => id !== t.id)
                      : [...activeTags, t.id],
                  ),
                  page: 1,
                })
              }
            >
              <span
                className="chip-dot"
                style={{ '--chip-color': paletteColor(t.paletteId) } as React.CSSProperties}
              />
              {t.name}
            </Button>
          ))}
        {otherFilters && (
          <FilterChips
            value={otherFilters}
            fields={fields}
            tags={tags.data || []}
            onChange={(next) =>
              update({ filters: withQuickTags(next, primaryTags?.id, activeTags), page: 1 })
            }
          />
        )}
        {narrowed && (
          <button type="button" className="text-action" onClick={reset}>
            Filter zurücksetzen
          </button>
        )}
      </div>
      <ErrorMessage error={a.error || list.error || tags.error || views.error} />
      {list.error && <Button onClick={list.reload}>Erneut versuchen</Button>}
      {selected.length > 0 && (
        <div className="selection-bar">
          <strong>{selected.length} ausgewählt</strong>
          <Button
            disabled={!primaryTags}
            onClick={() => {
              setTagField(primaryTags?.id || '');
              setBulk('tag');
            }}
          >
            Tags zuweisen
          </Button>
          <Button
            onClick={() =>
              void a.run(async () =>
                saveBlob(await download('/exports', { ids: selected }), 'puffolio-auswahl.zip'),
              )
            }
          >
            Auswahl exportieren
          </Button>
          <Button variant="danger" onClick={() => setBulk('trash')}>
            In Papierkorb
          </Button>
          <Button onClick={() => setSelected([])}>Auswahl aufheben</Button>
        </div>
      )}
      <div className={'collection-body ' + (list.loading && list.data ? 'refreshing' : '')}>
        {!list.data && list.loading ? (
          <div className="liquid-grid" role="status" aria-label="Laden…">
            {Array.from({ length: 8 }, (_, i) => (
              <div className="skeleton-card" key={i} />
            ))}
          </div>
        ) : list.data && items.length === 0 ? (
          <Empty
            action={
              narrowed ? (
                <Button onClick={reset}>Filter zurücksetzen</Button>
              ) : canEdit ? (
                <Button variant="primary" onClick={() => setEditing({})}>
                  Liquid hinzufügen
                </Button>
              ) : undefined
            }
          >
            {narrowed ? 'Keine Liquids gefunden.' : 'Noch keine Liquids.'}
          </Empty>
        ) : q.view === 'grid' ? (
          <div className="liquid-grid">
            {items.map((l) => (
              <LiquidCard
                key={l.id}
                liquid={l}
                layout={layout}
                tags={tags.data}
                onOpen={() => setDetail(l)}
                onEdit={canEdit ? () => setEditing({ liquid: l }) : undefined}
                onFavorite={canEdit ? () => favorite(l) : undefined}
              />
            ))}
          </div>
        ) : (
          <LiquidTable
            items={items}
            columns={shown.filter((f) => f !== imageRole)}
            showImage={showImage}
            showShelf={shelfOn}
            tags={tags.data}
            sort={q.sort}
            direction={q.direction}
            selected={selected}
            onSelect={(id, checked) =>
              setSelected(checked ? [...selected, id] : selected.filter((x) => x !== id))
            }
            onSelectAll={(checked) => setSelected(checked ? items.map((l) => l.id) : [])}
            onSort={(key) =>
              update(
                q.sort === key
                  ? { direction: q.direction === 'asc' ? 'desc' : 'asc', page: 1 }
                  : { sort: key, direction: defaultDirection(key, fields), page: 1 },
              )
            }
            onOpen={setDetail}
            onEdit={canEdit ? (l) => setEditing({ liquid: l }) : undefined}
            onFavorite={canEdit ? favorite : undefined}
          />
        )}
      </div>
      {list.data && list.data.total > 24 && (
        <div className="pagination">
          <Button disabled={q.page <= 1} onClick={() => update({ page: q.page - 1 })}>
            Zurück
          </Button>
          <span>
            Seite {q.page} von {Math.max(1, Math.ceil(list.data.total / q.pageSize))}
          </span>
          <Button
            disabled={q.page * q.pageSize >= list.data.total}
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
      )}
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
            toast('Gespeichert');
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
          onChange={(next) => (next ? replaceItem(next) : reload())}
          onTrashed={trashed}
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
                    update({
                      filters: withQuickTags(
                        normalizeFilter(filters, fields),
                        primaryTags?.id,
                        activeTags,
                      ),
                      page: 1,
                    });
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
        <ViewDialog
          query={q}
          views={views.data?.items || []}
          onViewsChanged={views.reload}
          onApply={(v) =>
            update({
              search: v.search,
              favorite: v.favorite,
              sort: v.sort,
              direction: v.direction,
              filters: v.filters,
              page: 1,
            })
          }
          onClose={() => setViewOpen(false)}
        />
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
              toast(`${selected.length} im Papierkorb`);
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
                    toast('Tags zugewiesen');
                    reload();
                  })
                }
              >
                Zuweisen
              </Button>
            </>
          }
        >
          {fields.filter((f) => f.type === 'tags' && !f.archived).length > 1 && (
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
          )}
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
