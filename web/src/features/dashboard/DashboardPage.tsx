import { createUUID } from '../../app/uuid';
import { useDraftGuard } from '../../components/useDraftGuard';
import { FieldValue } from '../../components/FieldValue';
import { LiquidImage } from '../../components/LiquidImage';
import { ShelfBar } from '../../components/ShelfBar';
import { useState } from 'react';
import { api } from '../../app/api';
import type {
  Field,
  Filter,
  Layout,
  Liquid,
  Tag,
  Widget,
  WidgetResult,
  WidgetType,
} from '../../app/types';
import { useApp, useLoad, useAction, useDirty, useCanEdit } from '../../app/state';
import {
  Button,
  IconButton,
  Dialog,
  ErrorMessage,
  Empty,
  Icon,
  Panel,
  PageHeader,
  Confirm,
  Stars,
} from '../../components/ui';
import { FieldInput } from '../../components/FieldInput';
import { paletteColor } from '../../components/PalettePicker';
import { formatNumber } from '../fields/values';
import { LiquidEditor } from '../liquids/LiquidEditor';
import { LiquidDetail } from '../liquids/LiquidDetail';
import { FilterBuilder } from '../liquids/FilterBuilder';
import { normalizeFilter, useTags } from '../liquids/data';
import { useToast } from '../../components/toast';
import { ColumnChart, DonutChart, ShelfStatusChart } from './charts';
import { parseWidgetLimit } from './widgetLimit';
const titles: Record<WidgetType, string> = {
  count: 'Anzahl Liquids',
  favorites: 'Favoriten',
  average: 'Durchschnittsbewertung',
  expired: 'Abgelaufen',
  expiring: 'Läuft bald ab',
  sum: 'Summe',
  recent: 'Zuletzt hinzugefügt',
  low: 'Unter Grenzwert',
  tags: 'Tag-Verteilung',
  donut: 'Kreisdiagramm',
  ratings: 'Bewertungsverteilung',
  shelfStatus: 'Haltbarkeit im Überblick',
  timeline: 'Pro Monat',
  top: 'Bestbewertet',
  dates: 'Datumsliste',
};
const fieldTypes: Partial<Record<WidgetType, string[]>> = {
  sum: ['number'],
  average: ['rating'],
  low: ['number'],
  tags: ['tags'],
  dates: ['date'],
  donut: ['tags', 'select', 'text', 'rating', 'boolean'],
  ratings: ['rating'],
  timeline: ['date'],
  top: ['rating', 'number'],
};
const limitedTypes: WidgetType[] = ['recent', 'low', 'tags', 'dates', 'expiring', 'donut', 'top'];
const statIcons: Partial<Record<WidgetType, string>> = {
  count: 'liquids',
  favorites: 'heart',
  average: 'star',
  sum: 'box',
  expired: 'calendar',
};
function WidgetEditor({
  widget,
  onClose,
  onAccept,
}: {
  widget?: Widget;
  onClose: () => void;
  onAccept: (w: Widget) => void;
}) {
  const { fields, preferences } = useApp();
  const tags = useTags();
  const [draft, setDraft] = useState<Widget>(() =>
    widget
      ? structuredClone(widget)
      : {
          id: createUUID(),
          type: 'count',
          title: titles.count,
          size: 'small',
          limit: 5,
        },
  );
  const [initial] = useState(() => JSON.stringify(draft));
  const [initialLimit] = useState(() => String(draft.limit || 5));
  const [limitText, setLimitText] = useState(initialLimit);
  const guard = useDraftGuard(
    JSON.stringify(draft) !== initial || limitText !== initialLimit,
    onClose,
  );
  const [error, setError] = useState<unknown>();
  const types = fieldTypes[draft.type];
  const referenced = fields.find((f) => f.id === draft.fieldId && !f.archived);
  const patch = (p: Partial<Widget>) => setDraft({ ...draft, ...p });
  return (
    <Dialog
      title={widget ? 'Widget bearbeiten' : 'Widget hinzufügen'}
      onClose={guard.close}
      footer={
        <>
          <Button onClick={guard.close}>Abbrechen</Button>
          <Button
            variant="primary"
            onClick={() => {
              try {
                const w = {
                  ...draft,
                  filters: normalizeFilter(draft.filters, fields),
                };
                if (types && !referenced) throw new Error('Feld auswählen.');
                if (limitedTypes.includes(w.type)) {
                  const limit = parseWidgetLimit(limitText);
                  if (limit === null) throw new Error('Einträge: ganze Zahl von 1 bis 10 eingeben.');
                  w.limit = limit;
                }
                if (w.type === 'low' && !w.threshold?.trim())
                  throw new Error('Grenzwert eingeben.');
                if (w.type === 'dates' && !w.window && (!w.from || !w.to || w.from > w.to))
                  throw new Error('Gültiges Datumsfenster wählen.');
                if (w.type === 'low' && referenced && w.threshold !== undefined)
                  w.threshold = String(
                    normalizeFilter(
                      {
                        fieldId: referenced.id,
                        operator: 'eq',
                        value: w.threshold,
                      },
                      fields,
                    )?.value,
                  );
                onAccept(w);
              } catch (e) {
                setError(e);
              }
            }}
          >
            Übernehmen
          </Button>
        </>
      }
    >
      <div className="form-grid">
        <label className="form-field">
          Widgettyp
          <select
            aria-label="Widgettyp"
            value={draft.type}
            onChange={(e) => {
              const t = e.target.value as WidgetType;
              patch({
                type: t,
                title: titles[t],
                fieldId:
                  t === 'dates' || t === 'timeline'
                    ? preferences.roles.purchase
                    : t === 'ratings' || t === 'top' || t === 'average'
                      ? preferences.roles.rating
                      : undefined,
                window: t === 'dates' ? 'past' : undefined,
              });
            }}
          >
            {Object.entries(titles).map(([id, title]) => (
              <option key={id} value={id}>
                {title}
              </option>
            ))}
          </select>
        </label>
        <label className="form-field">
          Größe
          <select
            aria-label="Widgetgröße"
            value={draft.size}
            onChange={(e) => patch({ size: e.target.value as Widget['size'] })}
          >
            <option value="small">Klein</option>
            <option value="medium">Mittel</option>
            <option value="wide">Breit</option>
          </select>
        </label>
        <label className="form-field full">
          Titel
          <input
            aria-label="Widgettitel"
            value={draft.title}
            maxLength={100}
            onChange={(e) => patch({ title: e.target.value })}
          />
        </label>
        {types && (
          <label className="form-field full">
            Feld
            <select
              aria-label="Widgetfeld"
              value={draft.fieldId || ''}
              onChange={(e) => patch({ fieldId: e.target.value })}
            >
              <option value="">Feld auswählen</option>
              {fields
                .filter((f) => !f.archived && types.includes(f.type))
                .map((f) => (
                  <option key={f.id} value={f.id}>
                    {f.label}
                  </option>
                ))}
            </select>
          </label>
        )}
        {limitedTypes.includes(draft.type) && (
          <label className="form-field">
            Einträge
            <input
              aria-label="Widgeteinträge"
              inputMode="numeric"
              value={limitText}
              aria-invalid={parseWidgetLimit(limitText) === null}
              onChange={(e) => setLimitText(e.target.value)}
            />
          </label>
        )}
        {draft.type === 'low' && referenced && (
          <>
            <FieldInput
              field={{ ...referenced, label: 'Grenzwert', required: true }}
              value={draft.threshold ?? null}
              onChange={(v) => patch({ threshold: String(v ?? '') })}
            />
            <label className="check-row full">
              <input
                type="checkbox"
                checked={!!draft.inclusive}
                onChange={(e) => patch({ inclusive: e.target.checked })}
              />
              Grenzwert einschließen
            </label>
          </>
        )}
        {draft.type === 'dates' && (
          <>
            <label className="form-field">
              Zeitraum
              <select
                aria-label="Widgetzeitraum"
                value={draft.window || 'custom'}
                onChange={(e) =>
                  patch({
                    window: e.target.value === 'custom' ? undefined : (e.target.value as 'past'),
                  })
                }
              >
                <option value="past">Vergangen, neueste zuerst</option>
                <option value="upcoming">Bevorstehend</option>
                <option value="custom">Fester Zeitraum</option>
              </select>
            </label>
            {!draft.window &&
              (['from', 'to'] as const).map((k, i) => (
                <label className="form-field" key={k}>
                  {i ? 'Bis' : 'Von'}
                  <input
                    type="date"
                    aria-label={i ? 'Widget bis' : 'Widget von'}
                    value={draft[k] || ''}
                    onChange={(e) => patch({ [k]: e.target.value })}
                  />
                </label>
              ))}
          </>
        )}
      </div>
      <fieldset>
        <legend>Filter</legend>
        <FilterBuilder
          fields={fields}
          tags={tags.data}
          value={draft.filters}
          onChange={(filters) => patch({ filters })}
        />
      </fieldset>
      <ErrorMessage error={error} />
      {guard.confirmation}
    </Dialog>
  );
}
/** The first text field reads as a card's subtitle, whatever it is named. */
const subtitleOf = (liquid: Liquid, fields: Field[]) => {
  const field = fields.find((f) => f.type === 'text' && !f.archived);
  const value = field && liquid.values[field.id]?.value;
  return typeof value === 'string' ? value : '';
};
function StatBody({
  widget,
  result,
  field,
}: {
  widget: Widget;
  result: WidgetResult;
  field?: Field;
}) {
  const { navigate, preferences } = useApp();
  const unit = field?.config.unit;
  const count = result.count ?? 0;
  if (widget.type === 'favorites')
    return (
      <>
        <button
          type="button"
          className="stat-value stat-link name-button"
          onClick={() => navigate('/liquids?favorite=true')}
        >
          {count}
        </button>
        <p className="stat-note">Markiert</p>
      </>
    );
  if (widget.type === 'count')
    return (
      <>
        <button
          type="button"
          className="stat-value stat-link name-button"
          onClick={() => navigate('/liquids')}
        >
          {count}
        </button>
        <p className="stat-note">{count === 1 ? 'Eintrag' : 'Einträge'}</p>
      </>
    );
  if (widget.type === 'expired')
    return (
      <>
        <div className="stat-value">{preferences.shelfLifeMonths > 0 ? count : '—'}</div>
        <p className="stat-note">
          {preferences.shelfLifeMonths > 0
            ? `Haltedauer ${
                preferences.shelfLifeMonths % 12
                  ? preferences.shelfLifeMonths + ' Monate'
                  : preferences.shelfLifeMonths / 12 + ' Jahre'
              }`
            : 'Keine Haltedauer festgelegt'}
        </p>
      </>
    );
  if (!result.value) return <div className="stat-note">Keine Daten</div>;
  return (
    <>
      <div className="stat-value">
        {formatNumber(result.value)}
        {unit && <small>{unit}</small>}
      </div>
      <p className="stat-note">
        {widget.type === 'average'
          ? `${count} ${count === 1 ? 'Bewertung' : 'Bewertungen'} · von ${field?.config.stars || 5}`
          : `${count} ${count === 1 ? 'Liquid' : 'Liquids'}`}
      </p>
    </>
  );
}
function RecentGrid({
  items,
  tags,
  onOpen,
}: {
  items: Liquid[];
  tags?: Tag[];
  onOpen: (l: Liquid) => void;
}) {
  const { fields, preferences } = useApp();
  const rating = fields.find((f) => f.id === preferences.roles.rating && !f.archived);
  const hasSubtitle = fields.some((f) => f.type === 'text' && !f.archived);
  return (
    <div className="recent-grid">
      {items.map((l) => {
        const stars = rating && l.values[rating.id]?.value;
        return (
          <div className="mini-card" key={l.id}>
            <LiquidImage liquid={l} tags={tags} />
            <button type="button" className="name-button" onClick={() => onOpen(l)}>
              <span title={l.name}>{l.name}</span>
            </button>
            <p className="card-subtitle">
              {subtitleOf(l, fields) || (hasSubtitle && <span className="muted">—</span>)}
            </p>
            <div className="card-rating-row">
              {typeof stars === 'number' ? (
                <Stars value={stars} max={rating?.config.stars || 5} />
              ) : (
                rating && <span className="muted">—</span>
              )}
            </div>
            <ShelfBar liquid={l} />
          </div>
        );
      })}
    </div>
  );
}
function RowList({
  widget,
  items,
  field,
  tags,
  onOpen,
}: {
  widget: Widget;
  items: Liquid[];
  field?: Field;
  tags?: Tag[];
  onOpen: (l: Liquid) => void;
}) {
  const { fields } = useApp();
  const shelfRows = widget.type === 'expiring';
  return (
    <ul>
      {items.map((l) => (
        <li className="compact-row" key={l.id}>
          <LiquidImage liquid={l} tags={tags} compact />
          <div className="compact-info">
            <button type="button" className="name-button" onClick={() => onOpen(l)}>
              <span>{l.name}</span>
            </button>
            {shelfRows ? <ShelfBar liquid={l} /> : <span>{subtitleOf(l, fields)}</span>}
          </div>
          {field && !shelfRows && l.values[field.id] && (
            <span className={widget.type === 'low' ? 'compact-value' : 'compact-date'}>
              <FieldValue field={field} value={l.values[field.id]?.value} tags={tags} />
            </span>
          )}
        </li>
      ))}
    </ul>
  );
}
function openTag(navigate: (url: string) => void, fieldId: string | undefined, id: string) {
  const filter: Filter = {
    op: 'and',
    conditions: [{ fieldId, operator: 'any', value: [id] }],
  };
  navigate('/liquids?filters=' + encodeURIComponent(JSON.stringify(filter)));
}
function TagBars({ result, fieldId }: { result: WidgetResult; fieldId?: string }) {
  const { navigate } = useApp();
  const max = Math.max(...result.tags!.map((t) => t.count));
  const open = (id: string) => openTag(navigate, fieldId, id);
  return (
    <ul className="tag-chart">
      {result.tags!.map((t) => (
        <li key={t.id}>
          <button type="button" className="chart-button" onClick={() => open(t.id)}>
            <div className="chart-label">
              <span>{t.name}</span>
              <strong>{t.count}</strong>
            </div>
            <div className="bar-track">
              <div
                className="bar-fill"
                style={{
                  width: `${(t.count / max) * 100}%`,
                  backgroundColor: paletteColor(t.paletteId),
                }}
              />
            </div>
          </button>
        </li>
      ))}
    </ul>
  );
}
export function DashboardPage() {
  const { fields, preferences, navigate } = useApp();
  const canEdit = useCanEdit();
  const toast = useToast();
  const layout = useLoad((s) => api<Layout>('/dashboard', 'GET', undefined, s), []);
  const [draft, setDraft] = useState<Layout>();
  const [widgetEdit, setWidgetEdit] = useState<{
    widget?: Widget;
    index?: number;
  }>();
  const [newLiquid, setNewLiquid] = useState(false);
  const [detail, setDetail] = useState<Liquid>();
  const [editLiquid, setEditLiquid] = useState<{
    liquid: Liquid;
    duplicate?: boolean;
  }>();
  const [cancelConfirm, setCancelConfirm] = useState(false);
  const [drag, setDrag] = useState<number>();
  const tags = useTags();
  const a = useAction();
  const widgets = draft?.widgets || layout.data?.widgets || [];
  const key = JSON.stringify(widgets);
  const query = useLoad(
    (s) =>
      widgets.length
        ? api<{ items: WidgetResult[] }>('/dashboard/query', 'POST', { widgets }, s)
        : Promise.resolve({ items: [] }),
    [key, preferences.shelfLifeMonths],
  );
  useDirty(!!draft && JSON.stringify(draft) !== JSON.stringify(layout.data));
  const edit = () => {
    if (layout.data) setDraft(structuredClone(layout.data));
  };
  const move = (from: number, to: number) => {
    if (!draft || to < 0 || to >= draft.widgets.length) return;
    const next = [...draft.widgets];
    const [w] = next.splice(from, 1);
    next.splice(to, 0, w);
    setDraft({ ...draft, widgets: next });
  };
  return (
    <>
      <PageHeader title="Dashboard">
        {canEdit && (
          <>
            <Button disabled={!layout.data} onClick={draft ? () => setCancelConfirm(true) : edit}>
              {!draft && <Icon name="sliders" />}
              {draft ? 'Abbrechen' : 'Dashboard anpassen'}
            </Button>
            {draft ? (
              <Button
                variant="primary"
                disabled={a.busy}
                onClick={() =>
                  void a.run(async () => {
                    const saved = await api<Layout>('/dashboard', 'PUT', draft);
                    layout.setData(saved);
                    setDraft(undefined);
                    toast('Dashboard gespeichert');
                  })
                }
              >
                Speichern
              </Button>
            ) : (
              <Button variant="primary" onClick={() => setNewLiquid(true)}>
                <Icon name="plus" />
                Liquid hinzufügen
              </Button>
            )}
          </>
        )}
      </PageHeader>
      <ErrorMessage error={a.error || layout.error || query.error} />
      {layout.error && <Button onClick={layout.reload}>Erneut versuchen</Button>}
      {draft && (
        <div className="dashboard-edit-bar">
          <Button
            variant="primary"
            disabled={draft.widgets.length >= 30}
            onClick={() => setWidgetEdit({})}
          >
            <Icon name="plus" />
            Widget hinzufügen
          </Button>
          <span className="muted">{draft.widgets.length} / 30 Widgets</span>
        </div>
      )}
      {layout.loading && !layout.data && <p role="status">Laden…</p>}
      {!layout.loading && !widgets.length && (
        <Empty
          action={canEdit && !draft ? <Button onClick={edit}>Widget hinzufügen</Button> : undefined}
        >
          Noch keine Widgets.
        </Empty>
      )}
      <div className="dashboard-grid">
        {widgets.map((w, i) => {
          const result = query.data?.items.find((r) => r.id === w.id);
          const field = fields.find((f) => f.id === w.fieldId);
          const title = w.title || titles[w.type];
          const isStat = ['count', 'favorites', 'sum', 'average', 'expired'].includes(w.type);
          const dragProps = draft
            ? {
                draggable: true,
                onDragStart: () => setDrag(i),
                onDragOver: (e: React.DragEvent) => e.preventDefault(),
                onDrop: (e: React.DragEvent) => {
                  e.preventDefault();
                  if (drag !== undefined) move(drag, i);
                  setDrag(undefined);
                },
              }
            : {};
          return (
            <div
              key={w.id}
              className={'widget-slot widget-' + w.size + (draft ? ' editing' : '')}
              {...dragProps}
            >
              <Panel
                className={
                  'widget ' +
                  (isStat ? 'stat-card' : '') +
                  (w.type === 'donut' ? ' widget-fill' : '') +
                  (draft ? ' editing' : '')
                }
                title={title}
                subtitle={
                  w.type === 'low' && w.threshold
                    ? `Unter ${formatNumber(w.threshold)}${field?.config.unit ? ' ' + field.config.unit : ''}`
                    : undefined
                }
                actions={
                  isStat ? (
                    <span className="stat-icon" aria-hidden="true">
                      <Icon name={statIcons[w.type] || 'box'} />
                    </span>
                  ) : w.type === 'low' && result?.count ? (
                    <span className="count-badge">{result.count}</span>
                  ) : undefined
                }
              >
                {query.loading && !result ? (
                  <p role="status">Laden…</p>
                ) : result?.error ? (
                  <p className="field-error">{result.error}</p>
                ) : !result ? (
                  <p className="muted">Keine Daten.</p>
                ) : isStat ? (
                  <StatBody widget={w} result={result} field={field} />
                ) : w.type === 'donut' ? (
                  result.segments?.some((x) => x.count) ? (
                    <DonutChart
                      title={title}
                      segments={result.segments.filter((x) => x.count)}
                      totalLabel={field?.type === 'tags' ? 'Zuordnungen' : 'Liquids'}
                      onSelect={
                        field?.type === 'tags'
                          ? (segment) => openTag(navigate, w.fieldId, segment.id)
                          : undefined
                      }
                    />
                  ) : (
                    <p className="muted">Keine Daten.</p>
                  )
                ) : w.type === 'ratings' ? (
                  result.count ? (
                    <ColumnChart
                      title={title}
                      segments={result.segments || []}
                      showValues
                      describe={(x) =>
                        `${x.label} ${x.label === '1' ? 'Stern' : 'Sterne'}: ${x.count}`
                      }
                    />
                  ) : (
                    <p className="muted">Keine Daten.</p>
                  )
                ) : w.type === 'timeline' ? (
                  result.count ? (
                    <ColumnChart
                      title={title}
                      segments={result.segments || []}
                      labelEvery={3}
                      describe={(x) => `${x.label}: ${x.count}`}
                    />
                  ) : (
                    <p className="muted">Keine Daten in den letzten 12 Monaten.</p>
                  )
                ) : w.type === 'shelfStatus' ? (
                  result.count ? (
                    <ShelfStatusChart title={title} segments={result.segments || []} />
                  ) : (
                    <p className="muted">Keine Daten.</p>
                  )
                ) : w.type === 'tags' ? (
                  result.tags?.length ? (
                    <TagBars result={result} fieldId={w.fieldId} />
                  ) : (
                    <p className="muted">Keine Daten.</p>
                  )
                ) : result.items?.length ? (
                  w.type === 'recent' ? (
                    <RecentGrid items={result.items} tags={tags.data} onOpen={setDetail} />
                  ) : (
                    <RowList
                      widget={w}
                      items={result.items}
                      field={field}
                      tags={tags.data}
                      onOpen={setDetail}
                    />
                  )
                ) : (
                  <p className="muted">Keine Daten.</p>
                )}
                {draft && (
                  <div className="widget-edit-actions">
                    <IconButton
                      label={`${title} nach oben`}
                      icon="up"
                      disabled={i === 0}
                      onClick={() => move(i, i - 1)}
                    />
                    <IconButton
                      label={`${title} nach unten`}
                      icon="down"
                      disabled={i === widgets.length - 1}
                      onClick={() => move(i, i + 1)}
                    />
                    <IconButton
                      label={`${title} bearbeiten`}
                      icon="edit"
                      onClick={() => setWidgetEdit({ widget: w, index: i })}
                    />
                    <IconButton
                      label={`${title} entfernen`}
                      icon="trash"
                      onClick={() =>
                        setDraft({
                          ...draft,
                          widgets: draft.widgets.filter((x) => x.id !== w.id),
                        })
                      }
                    />
                  </div>
                )}
              </Panel>
            </div>
          );
        })}
      </div>
      {widgetEdit && (
        <WidgetEditor
          widget={widgetEdit.widget}
          onClose={() => setWidgetEdit(undefined)}
          onAccept={(w) => {
            if (!draft) return;
            setDraft({
              ...draft,
              widgets:
                widgetEdit.index === undefined
                  ? [...draft.widgets, w]
                  : draft.widgets.map((x, i) => (i === widgetEdit.index ? w : x)),
            });
            setWidgetEdit(undefined);
          }}
        />
      )}
      {cancelConfirm && (
        <Confirm
          title="Dashboard-Entwurf verwerfen?"
          onCancel={() => setCancelConfirm(false)}
          onConfirm={() => {
            setDraft(undefined);
            setCancelConfirm(false);
          }}
        >
          <p>Die gespeicherte Anordnung bleibt erhalten.</p>
        </Confirm>
      )}
      {newLiquid && (
        <LiquidEditor
          onClose={() => setNewLiquid(false)}
          onSaved={() => {
            setNewLiquid(false);
            query.reload();
            toast('Gespeichert');
          }}
        />
      )}
      {editLiquid && (
        <LiquidEditor
          {...editLiquid}
          onClose={() => {
            setEditLiquid(undefined);
            query.reload();
          }}
          onSaved={() => {
            setEditLiquid(undefined);
            query.reload();
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
            setEditLiquid({ liquid: detail });
            setDetail(undefined);
          }}
          onDuplicate={() => {
            setEditLiquid({ liquid: detail, duplicate: true });
            setDetail(undefined);
          }}
          onChange={query.reload}
        />
      )}
    </>
  );
}
