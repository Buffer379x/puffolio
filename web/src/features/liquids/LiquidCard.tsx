import type { Liquid, Scalar, Tag } from '../../app/types';
import { FieldValue } from '../../components/FieldValue';
import { LiquidImage } from '../../components/LiquidImage';
import { ShelfBar } from '../../components/ShelfBar';
import { Icon, IconButton } from '../../components/ui';
import type { CardLayout, CardRow } from './cardLayout';
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
/** Every card shows the same rows, so an unknown value reads "—" instead of moving the rest up. */
function Row({ row, liquid, tags }: { row: CardRow; liquid: Liquid; tags?: Tag[] }) {
  const value = (id: string): Scalar | undefined => {
    const stored = liquid.values[id]?.value;
    return Array.isArray(stored) && !stored.length ? null : stored;
  };
  switch (row.kind) {
    case 'subtitle':
      return (
        <p className="card-subtitle">
          <FieldValue field={row.field} value={value(row.field.id)} />
        </p>
      );
    case 'rating':
      return (
        <div className="card-rating-row">
          <FieldValue field={row.field} value={value(row.field.id)} />
        </div>
      );
    case 'shelf':
      return (
        <div className="card-shelf">
          <ShelfBar liquid={liquid} />
        </div>
      );
    case 'tags':
      return (
        <div className="card-tags">
          <FieldValue field={row.field} value={value(row.field.id)} tags={tags} />
        </div>
      );
    case 'data':
      return (
        <dl className="card-data">
          {row.fields.map((f) => (
            <div key={f.id}>
              <dt>{f.label}</dt>
              <dd>
                <FieldValue field={f} value={value(f.id)} tags={tags} />
              </dd>
            </div>
          ))}
        </dl>
      );
  }
}
export function LiquidCard({
  liquid,
  layout,
  tags,
  onOpen,
  onEdit,
  onFavorite,
}: {
  liquid: Liquid;
  layout: CardLayout;
  tags?: Tag[];
  onOpen: () => void;
  onEdit?: () => void;
  onFavorite?: () => void;
}) {
  // Image and name, the rows, and the edit action share one grid across the cards of a row.
  const cardRows = 2 + layout.rows.length + (onEdit ? 1 : 0);
  return (
    <article
      className="item-card liquid-card"
      style={{ '--card-rows': cardRows } as React.CSSProperties}
    >
      <LiquidImage liquid={liquid} tags={tags} stripped={layout.stripped} />
      <FavoriteButton liquid={liquid} onToggle={onFavorite} />
      <div className="item-card-content">
        <h2>
          <button type="button" className="name-button" onClick={onOpen}>
            <span title={liquid.name}>{liquid.name}</span>
          </button>
        </h2>
        {layout.rows.map((row, i) => (
          <Row key={i} row={row} liquid={liquid} tags={tags} />
        ))}
        {onEdit && (
          <div className="item-card-actions">
            <IconButton icon="edit" label={`${liquid.name} bearbeiten`} onClick={onEdit} />
          </div>
        )}
      </div>
    </article>
  );
}
