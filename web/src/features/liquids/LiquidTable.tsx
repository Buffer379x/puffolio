import type { Field, Liquid, Tag } from '../../app/types';
import { FieldValue } from '../../components/FieldValue';
import { LiquidImage } from '../../components/LiquidImage';
import { ShelfBar } from '../../components/ShelfBar';
import { Icon, IconButton } from '../../components/ui';
const sortableTypes = ['text', 'number', 'rating', 'date', 'boolean'];
const fieldColumn = (field: Field) =>
  'field-cell cell-' + field.type + (field.type === 'number' ? ' numeric' : '');
export function LiquidTable({
  items,
  columns,
  showImage,
  showShelf,
  tags,
  sort,
  direction,
  selected,
  onSelect,
  onSelectAll,
  onSort,
  onOpen,
  onEdit,
  onFavorite,
}: {
  items: Liquid[];
  columns: Field[];
  showImage: boolean;
  showShelf: boolean;
  tags?: Tag[];
  sort: string;
  direction: string;
  selected: string[];
  onSelect?: (id: string, checked: boolean) => void;
  onSelectAll?: (checked: boolean) => void;
  onSort: (key: string) => void;
  onOpen: (liquid: Liquid) => void;
  onEdit?: (liquid: Liquid) => void;
  onFavorite?: (liquid: Liquid) => void;
}) {
  const editable = !!onEdit;
  const header = (key: string, label: string) => (
    <button type="button" className="th-sort" onClick={() => onSort(key)}>
      {label}
      {sort === key && <Icon name={direction === 'asc' ? 'sortAsc' : 'sortDesc'} />}
    </button>
  );
  const ariaSort = (key: string) =>
    sort === key ? (direction === 'asc' ? 'ascending' : 'descending') : undefined;
  return (
    <div className="table-scroll" tabIndex={0} aria-label="Liquidtabelle">
      <table>
        <thead>
          <tr>
            {editable && (
              <th className="checkbox-column">
                <label className="check-row checkbox-target">
                  <input
                    type="checkbox"
                    aria-label="Alle auf dieser Seite auswählen"
                    checked={!!items.length && selected.length === items.length}
                    onChange={(e) => onSelectAll?.(e.target.checked)}
                  />
                </label>
              </th>
            )}
            <th className="name-column" aria-sort={ariaSort('name')}>
              {header('name', 'Name')}
            </th>
            {columns.map((f) => (
              <th key={f.id} className={fieldColumn(f)} aria-sort={ariaSort('field:' + f.id)}>
                {sortableTypes.includes(f.type) ? header('field:' + f.id, f.label) : f.label}
              </th>
            ))}
            {showShelf && (
              <th className="cell-shelf" aria-sort={ariaSort('shelf')}>
                {header('shelf', 'Haltbarkeit')}
              </th>
            )}
            {editable && <th className="actions-column">Aktionen</th>}
          </tr>
        </thead>
        <tbody>
          {items.map((l) => (
            <tr key={l.id}>
              {editable && (
                <td className="checkbox-column">
                  <label className="check-row checkbox-target">
                    <input
                      type="checkbox"
                      aria-label={`${l.name} auswählen`}
                      checked={selected.includes(l.id)}
                      onChange={(e) => onSelect?.(l.id, e.target.checked)}
                    />
                  </label>
                </td>
              )}
              <th className="name-column" scope="row">
                <div className="table-name">
                  {showImage && <LiquidImage liquid={l} tags={tags} compact />}
                  <button type="button" className="name-button" onClick={() => onOpen(l)}>
                    {l.name}
                  </button>
                  {!editable && l.favorite && (
                    <span className="table-favorite" role="img" aria-label="Favorit">
                      <Icon name="heart" />
                    </span>
                  )}
                </div>
              </th>
              {columns.map((f) => (
                <td key={f.id} className={fieldColumn(f)}>
                  <FieldValue field={f} value={l.values[f.id]?.value} tags={tags} />
                </td>
              ))}
              {showShelf && (
                <td className="cell-shelf">
                  <ShelfBar liquid={l} />
                </td>
              )}
              {editable && (
                <td className="actions-column">
                  <div className="actions table-actions">
                    <IconButton
                      label={`${l.favorite ? 'Favorit entfernen' : 'Als Favorit markieren'}: ${l.name}`}
                      icon="heart"
                      aria-pressed={l.favorite}
                      onClick={() => onFavorite?.(l)}
                    />
                    <IconButton
                      label={`${l.name} bearbeiten`}
                      icon="edit"
                      onClick={() => onEdit?.(l)}
                    />
                  </div>
                </td>
              )}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
