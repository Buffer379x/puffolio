import { createUUID } from '../../app/uuid';
import { useDraftGuard } from '../../components/useDraftGuard';
import { FieldValue } from '../../components/FieldValue';
import { useState, useEffect } from 'react';
import { api } from '../../app/api';
import type { Widget, WidgetType, Layout, WidgetResult, Liquid } from '../../app/types';
import { useApp, useLoad, useAction, useDirty } from '../../app/state';
import {
  Button,
  IconButton,
  Dialog,
  ErrorMessage,
  Empty,
  Panel,
  PageHeader,
  Confirm,
} from '../../components/ui';
import { FieldInput } from '../../components/FieldInput';
import { paletteColor } from '../../components/PalettePicker';
import { formatDecimal } from '../fields/values';
import { LiquidEditor } from '../liquids/LiquidEditor';
import { LiquidDetail } from '../liquids/LiquidDetail';
import { FilterBuilder } from '../liquids/FilterBuilder';
import { normalizeFilter, useTags } from '../liquids/data';
const titles: Record<WidgetType, string> = {
  count: 'Anzahl Liquids',
  sum: 'Mengen-Summe',
  favorites: 'Favoriten',
  average: 'Durchschnittsbewertung',
  recent: 'Zuletzt hinzugefügt',
  low: 'Niedriger Bestand',
  tags: 'Tag-Verteilung',
  dates: 'Datumsliste',
};
const fieldType: Partial<Record<WidgetType, string>> = {
  sum: 'number',
  average: 'rating',
  low: 'number',
  tags: 'tags',
  dates: 'date',
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
  const { fields } = useApp();
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
  const guard = useDraftGuard(JSON.stringify(draft) !== initial, onClose);
  const [error, setError] = useState<unknown>();
  const type = fieldType[draft.type];
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
                if (type && !referenced) throw new Error('Feld auswählen.');
                if (w.type === 'low' && !w.threshold?.trim())
                  throw new Error('Grenzwert eingeben.');
                if (w.type === 'dates' && (!w.from || !w.to || w.from > w.to))
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
              patch({ type: t, title: titles[t], fieldId: undefined });
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
        {type && (
          <label className="form-field full">
            Feld
            <select
              aria-label="Widgetfeld"
              value={draft.fieldId || ''}
              onChange={(e) => patch({ fieldId: e.target.value })}
            >
              <option value="">Feld auswählen</option>
              {fields
                .filter((f) => !f.archived && f.type === type)
                .map((f) => (
                  <option key={f.id} value={f.id}>
                    {f.label}
                  </option>
                ))}
            </select>
          </label>
        )}
        {['recent', 'low', 'tags', 'dates'].includes(draft.type) && (
          <label className="form-field">
            Einträge
            <select
              aria-label="Widgeteinträge"
              value={draft.limit || 5}
              onChange={(e) => patch({ limit: Number(e.target.value) as 3 | 5 | 10 })}
            >
              {[3, 5, 10].map((n) => (
                <option key={n}>{n}</option>
              ))}
            </select>
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
            {(['from', 'to'] as const).map((k, i) => (
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
export function DashboardPage() {
  const { fields, navigate } = useApp();
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
    [key],
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
        <Button disabled={!layout.data} onClick={draft ? () => setCancelConfirm(true) : edit}>
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
              })
            }
          >
            Speichern
          </Button>
        ) : (
          <Button variant="primary" onClick={() => setNewLiquid(true)}>
            Liquid hinzufügen
          </Button>
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
            Widget hinzufügen
          </Button>
          <span className="muted">{draft.widgets.length} / 30 Widgets</span>
        </div>
      )}
      {layout.loading && <p role="status">Laden…</p>}
      {!layout.loading && !widgets.length && (
        <Empty action={!draft ? <Button onClick={edit}>Widget hinzufügen</Button> : undefined}>
          Noch keine Widgets.
        </Empty>
      )}
      <div className="dashboard-grid">
        {widgets.map((w, i) => {
          const result = query.data?.items.find((r) => r.id === w.id);
          const field = fields.find((f) => f.id === w.fieldId);
          return (
            <Panel
              key={w.id}
              className={'widget widget-' + w.size}
              title={w.title || titles[w.type]}
              actions={
                draft ? (
                  <div className="actions">
                    <IconButton
                      label={`${w.title} nach oben`}
                      icon="arrow"
                      className="arrow-up"
                      disabled={i === 0}
                      onClick={() => move(i, i - 1)}
                    />
                    <IconButton
                      label={`${w.title} nach unten`}
                      icon="arrow"
                      className="arrow-down"
                      disabled={i === widgets.length - 1}
                      onClick={() => move(i, i + 1)}
                    />
                  </div>
                ) : undefined
              }
            >
              {draft && (
                <div
                  className="widget-edit-actions"
                  draggable
                  onDragStart={() => setDrag(i)}
                  onDragOver={(e) => e.preventDefault()}
                  onDrop={(e) => {
                    e.preventDefault();
                    if (drag !== undefined) move(drag, i);
                    setDrag(undefined);
                  }}
                >
                  <Button onClick={() => setWidgetEdit({ widget: w, index: i })}>Bearbeiten</Button>
                  <IconButton
                    label={`${w.title} entfernen`}
                    icon="trash"
                    onClick={() =>
                      setDraft({
                        ...draft,
                        widgets: draft.widgets.filter((x) => x.id !== w.id),
                      })
                    }
                  />
                  <span className="muted">Verschieben</span>
                </div>
              )}
              {query.loading ? (
                <p role="status">Laden…</p>
              ) : result?.error ? (
                <p className="field-error">{result.error}</p>
              ) : !result ? (
                <p className="muted">Keine Daten.</p>
              ) : ['count', 'favorites', 'sum', 'average'].includes(w.type) ? (
                <div className="stat-body">
                  {w.type === 'favorites' ? (
                    <button
                      className="stat-number name-button"
                      onClick={() => navigate('/liquids?favorite=true')}
                    >
                      {result.count ?? '—'}
                    </button>
                  ) : (
                    <div className="stat-number">
                      {w.type === 'sum' && result.count === 0
                        ? 'Keine Daten'
                        : result.value !== undefined
                          ? formatDecimal(result.value)
                          : result.count !== undefined
                            ? result.count
                            : 'Keine Daten'}
                    </div>
                  )}
                  {field?.config.unit && !(w.type === 'sum' && result.count === 0) && (
                    <span>{field.config.unit}</span>
                  )}
                </div>
              ) : w.type === 'tags' ? (
                result.tags?.length ? (
                  <ul className="tag-chart">
                    {result.tags.map((t) => (
                      <li key={t.id}>
                        <span
                          className="tag"
                          style={{ backgroundColor: paletteColor(t.paletteId) }}
                        >
                          {t.name}
                        </span>
                        <span
                          className="tag-bar"
                          style={{
                            width: `${(t.count / Math.max(...result.tags!.map((t) => t.count))) * 100}%`,
                            backgroundColor: paletteColor(t.paletteId),
                          }}
                        />
                        <strong>{t.count}</strong>
                      </li>
                    ))}
                  </ul>
                ) : (
                  <p className="muted">Keine Daten.</p>
                )
              ) : result.items?.length ? (
                <ul className="widget-liquids">
                  {result.items.map((l) => (
                    <li key={l.id}>
                      <button className="name-button" onClick={() => setDetail(l)}>
                        {l.name}
                      </button>
                      {field && l.values[field.id] && (
                        <FieldValue
                          field={field}
                          value={l.values[field.id]?.value}
                          tags={tags.data}
                        />
                      )}
                    </li>
                  ))}
                </ul>
              ) : (
                <p className="muted">Keine Daten.</p>
              )}
            </Panel>
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
