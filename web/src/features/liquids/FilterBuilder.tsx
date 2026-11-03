import type { Field, Filter, Scalar, Tag } from '../../app/types';
import { FieldInput } from '../../components/FieldInput';
import { Button, IconButton } from '../../components/ui';
const labels: Record<string, string> = {
  eq: 'Gleich',
  contains: 'Enthält',
  empty: 'Fehlt',
  notEmpty: 'Vorhanden',
  gte: 'Mindestens',
  lte: 'Höchstens',
  before: 'Vor',
  after: 'Nach',
  between: 'Zwischen',
  any: 'Einer von',
  all: 'Alle',
  none: 'Keiner von',
};
export function allowedOperators(type: string) {
  return [
    'empty',
    'notEmpty',
    ...(type === 'image'
      ? []
      : type === 'tags'
        ? ['any', 'all', 'none']
        : type === 'text' || type === 'longtext'
          ? ['eq', 'contains']
          : type === 'number' || type === 'rating'
            ? ['eq', 'gte', 'lte', 'between']
            : type === 'date'
              ? ['eq', 'before', 'after', 'between']
              : ['eq']),
  ];
}
export function FilterBuilder({
  value,
  onChange,
  fields,
  tags = [],
}: {
  value?: Filter;
  onChange: (v: Filter | undefined) => void;
  fields: Field[];
  tags?: Tag[];
}) {
  const conditions = value?.op ? value.conditions || [] : value ? [value] : [];
  const favorite: Field = {
    id: 'favorite',
    label: 'Favorit',
    type: 'boolean',
    position: -1,
    version: 1,
    required: false,
    archived: false,
    visibility: { form: false, detail: false, list: false, grid: false },
    config: { filterable: true },
  };
  const available = [favorite, ...fields];
  const active = available.filter((f) => !f.archived && f.config.filterable);
  const update = (items: Filter[]) =>
    onChange(items.length ? { op: value?.op || 'and', conditions: items } : undefined);
  return (
    <div className="filter-builder">
      <div className="actions">
        <select
          aria-label="Filter verknüpfen"
          value={value?.op || 'and'}
          onChange={(e) => onChange({ op: e.target.value as 'and' | 'or', conditions })}
        >
          <option value="and">Alle Bedingungen (UND)</option>
          <option value="or">Eine Bedingung (ODER)</option>
        </select>
        <Button
          disabled={conditions.length >= 20 || !active.length}
          onClick={() => {
            const f = active[0];
            update([
              ...conditions,
              {
                fieldId: f.id,
                operator: f.id === 'favorite' ? 'eq' : 'notEmpty',
                ...(f.id === 'favorite' ? { value: true } : {}),
              },
            ]);
          }}
        >
          Bedingung hinzufügen
        </Button>
      </div>
      {conditions.map((c, i) => {
        const field = available.find((f) => f.id === c.fieldId);
        const change = (patch: Partial<Filter>) =>
          update(conditions.map((x, j) => (i === j ? { ...x, ...patch } : x)));
        return (
          <div className="filter-row" key={i}>
            <select
              aria-label={`Filterfeld ${i + 1}`}
              value={c.fieldId}
              onChange={(e) =>
                change({
                  fieldId: e.target.value,
                  operator: e.target.value === 'favorite' ? 'eq' : 'notEmpty',
                  value: e.target.value === 'favorite' ? true : undefined,
                  value2: undefined,
                })
              }
            >
              {!field && <option value={c.fieldId}>Feld nicht verfügbar</option>}
              {active.map((f) => (
                <option key={f.id} value={f.id}>
                  {f.label}
                </option>
              ))}
            </select>
            <select
              aria-label={`Operator ${i + 1}`}
              value={c.operator}
              onChange={(e) =>
                change({ operator: e.target.value, value: undefined, value2: undefined })
              }
            >
              {(c.fieldId === 'favorite' ? ['eq'] : allowedOperators(field?.type || 'image')).map(
                (op) => (
                  <option key={op} value={op}>
                    {labels[op]}
                  </option>
                ),
              )}
            </select>
            {field && !['empty', 'notEmpty'].includes(c.operator || '') && (
              <div className="filter-value">
                {field.type === 'tags' ? (
                  <div className="tag-options">
                    {tags
                      .filter((t) => t.fieldId === field.id)
                      .map((t) => (
                        <label className="check-row" key={t.id}>
                          <input
                            type="checkbox"
                            checked={Array.isArray(c.value) && c.value.includes(t.id)}
                            onChange={(e) => {
                              const selected = Array.isArray(c.value) ? c.value : [];
                              change({
                                value: e.target.checked
                                  ? [...selected, t.id]
                                  : selected.filter((v) => v !== t.id),
                              });
                            }}
                          />
                          {t.name}
                        </label>
                      ))}
                  </div>
                ) : (
                  <FieldInput
                    compact
                    field={{ ...field, label: 'Filterwert', required: false }}
                    value={c.value ?? null}
                    onChange={(v) => change({ value: v })}
                  />
                )}
              </div>
            )}
            {field && c.operator === 'between' && (
              <FieldInput
                compact
                field={{ ...field, label: 'Bis', required: false }}
                value={c.value2 ?? null}
                onChange={(v) => change({ value2: v })}
              />
            )}
            <IconButton
              label={`Bedingung ${i + 1} entfernen`}
              icon="close"
              onClick={() => update(conditions.filter((_, j) => i !== j))}
            />
          </div>
        );
      })}
    </div>
  );
}
