import type { Field, Scalar, Tag } from '../app/types';
import { formatDate, formatDecimal } from '../features/fields/values';
import { paletteColor } from './PalettePicker';
export function FieldValue({
  field,
  value,
  tags = [],
}: {
  field: Field;
  value: Scalar | undefined;
  tags?: Tag[];
}) {
  if (value === null || value === undefined) return <span className="muted">—</span>;
  switch (field.type) {
    case 'number':
      return (
        <span className="tabular">
          {formatDecimal(String(value))}
          {field.config.unit ? ' ' + field.config.unit : ''}
        </span>
      );
    case 'date':
      return <time dateTime={String(value)}>{formatDate(String(value))}</time>;
    case 'boolean':
      return <>{value ? 'Ja' : 'Nein'}</>;
    case 'rating':
      return (
        <span aria-label={`${value} von ${field.config.stars || 5}`}>
          {'★'.repeat(Number(value))}
          <span className="muted">
            {'☆'.repeat(Math.max(0, (field.config.stars || 5) - Number(value)))}
          </span>
        </span>
      );
    case 'image':
      return <img className="item-image" src={'/api/v1/media/' + value} alt={field.label} />;
    case 'color':
      return (
        <span className="color-value">
          <span style={{ backgroundColor: String(value) }} />
          {String(value)}
        </span>
      );
    case 'select':
      return (
        <>{field.config.options?.find((o) => o.id === value)?.label || 'Option nicht verfügbar'}</>
      );
    case 'tags':
      return (
        <span className="tags">
          {(value as string[]).map((id) => {
            const t = tags.find((t) => t.id === id);
            return (
              <span
                className="tag"
                key={id}
                style={{ backgroundColor: paletteColor(t?.paletteId || 'slate') }}
              >
                {t?.name || 'Tag nicht verfügbar'}
              </span>
            );
          })}
        </span>
      );
    default:
      return <span className="text-value">{String(value)}</span>;
  }
}
