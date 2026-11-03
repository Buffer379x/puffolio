import type { Field, Filter, Scalar, Tag } from '../../app/types';
import { Button } from '../../components/ui';
import { formatDate, formatDecimal } from '../fields/values';
import { filterOperatorLabels } from './FilterBuilder';

function removeCondition(filter: Filter, path: number[]): Filter | undefined {
  if (!path.length) return undefined;
  const [index, ...rest] = path;
  const conditions = (filter.conditions || []).flatMap((child, i) => {
    const remaining = i === index ? removeCondition(child, rest) : child;
    return remaining ? [remaining] : [];
  });
  return conditions.length ? { ...filter, conditions } : undefined;
}
export function FilterChips({
  value,
  fields,
  tags,
  onChange,
}: {
  value: Filter;
  fields: Field[];
  tags: Tag[];
  onChange: (value: Filter | undefined) => void;
}) {
  const render = (condition: Filter, path: number[]): React.ReactNode => {
    if (condition.op) {
      const label = condition.op === 'and' ? 'Alle Bedingungen (UND)' : 'Eine Bedingung (ODER)';
      return (
        <div className="filter-chip-group" role="group" aria-label={label} key={path.join('.')}>
          <span className="filter-group-label">{label}</span>
          {condition.conditions?.map((child, i) => render(child, [...path, i]))}
        </div>
      );
    }
    const field = fields.find((f) => f.id === condition.fieldId);
    const format = (value: Scalar | undefined): string => {
      if (value === undefined || value === null) return '';
      if (typeof value === 'boolean') return value ? 'Ja' : 'Nein';
      if (Array.isArray(value))
        return value
          .map(
            (id) =>
              tags.find((t) => t.id === id)?.name ||
              field?.config.options?.find((o) => o.id === id)?.label ||
              id,
          )
          .join(', ');
      if (field?.type === 'number')
        return formatDecimal(String(value)) + (field.config.unit ? ' ' + field.config.unit : '');
      if (field?.type === 'date') return formatDate(String(value));
      if (field?.type === 'select')
        return field.config.options?.find((o) => o.id === value)?.label || String(value);
      return String(value);
    };
    const label = [
      condition.fieldId === 'favorite' ? 'Favorit' : field?.label || 'Feld nicht verfügbar',
      filterOperatorLabels[condition.operator || ''] || condition.operator,
      format(condition.value),
      condition.value2 !== undefined ? 'bis ' + format(condition.value2) : '',
    ]
      .filter(Boolean)
      .join(' ');
    return (
      <Button
        key={path.join('.')}
        className="filter-chip"
        aria-label={'Filter entfernen: ' + label}
        onClick={() => onChange(removeCondition(value, path))}
      >
        {label}
        <span aria-hidden="true"> ×</span>
      </Button>
    );
  };
  return <div className="filter-chips">{render(value, [])}</div>;
}
