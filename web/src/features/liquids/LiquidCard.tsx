import type { Field, Liquid, Tag } from '../../app/types';
import { FieldValue } from '../../components/FieldValue';
import { LiquidImage } from '../../components/LiquidImage';
import { ShelfBar } from '../../components/ShelfBar';
import { Icon } from '../../components/ui';
export interface CardFields {
  subtitle?: Field;
  rating?: Field;
  tagFields: Field[];
  rest: Field[];
  stripped: boolean;
}
/** A value-less field takes no room: a card shows what is known, not what is missing. */
const hasValue = (liquid: Liquid, field: Field) => {
  const value = liquid.values[field.id]?.value;
  return value !== undefined && value !== null && !(Array.isArray(value) && !value.length);
};
export function FavoriteButton({ liquid, onToggle }: { liquid: Liquid; onToggle?: () => void }) {
  if (!onToggle)
    return liquid.favorite ? (
      <span className="fav-button" role="img" aria-label="Favorit" data-on="true">
        <Icon name="heart" />
      </span>
    ) : null;
  return (
    <button
      type="button"
      className="fav-button"
      aria-label={`${liquid.favorite ? 'Favorit entfernen' : 'Als Favorit markieren'}: ${liquid.name}`}
      title={liquid.favorite ? 'Favorit entfernen' : 'Als Favorit markieren'}
      aria-pressed={liquid.favorite}
      onClick={onToggle}
    >
      <Icon name="heart" />
    </button>
  );
}
export function LiquidCard({
  liquid,
  fields,
  tags,
  onOpen,
  onEdit,
  onFavorite,
}: {
  liquid: Liquid;
  fields: CardFields;
  tags?: Tag[];
  onOpen: () => void;
  onEdit?: () => void;
  onFavorite?: () => void;
}) {
  const { subtitle, rating, tagFields, rest, stripped } = fields;
  const data = rest.filter((f) => hasValue(liquid, f));
  const assignedTags = tagFields.filter((f) => hasValue(liquid, f));
  return (
    <article className="item-card">
      <LiquidImage liquid={liquid} tags={tags} stripped={stripped} />
      <FavoriteButton liquid={liquid} onToggle={onFavorite} />
      <div className="item-card-content">
        <h2>
          <button type="button" className="name-button" onClick={onOpen}>
            <span>{liquid.name}</span>
          </button>
        </h2>
        {subtitle && (
          <p className="card-subtitle">{String(liquid.values[subtitle.id]?.value ?? '')}</p>
        )}
        {rating && hasValue(liquid, rating) && (
          <div className="card-rating-row">
            <FieldValue field={rating} value={liquid.values[rating.id]?.value} />
          </div>
        )}
        <ShelfBar liquid={liquid} />
        {assignedTags.map((f) => (
          <FieldValue key={f.id} field={f} value={liquid.values[f.id]?.value} tags={tags} />
        ))}
        {data.length > 0 && (
          <dl className="card-data">
            {data.map((f) => (
              <div key={f.id}>
                <dt>{f.label}</dt>
                <dd>
                  <FieldValue field={f} value={liquid.values[f.id]?.value} tags={tags} />
                </dd>
              </div>
            ))}
          </dl>
        )}
        {onEdit && (
          <div className="item-card-actions">
            <button
              type="button"
              className="text-action"
              aria-label={`${liquid.name} bearbeiten`}
              onClick={onEdit}
            >
              <Icon name="edit" />
              Bearbeiten
            </button>
          </div>
        )}
      </div>
    </article>
  );
}
