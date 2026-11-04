import { useApp } from '../app/state';
import type { Liquid, Tag } from '../app/types';
import { palette, paletteColor } from './PalettePicker';
import { Icon } from './ui';
/** Every liquid has a recognisable colour, so a collection without photos still reads well. */
export function liquidColor(liquid: Liquid, colorFieldId: string | undefined, tags: Tag[] = []) {
  const color = colorFieldId && liquid.values[colorFieldId]?.value;
  if (typeof color === 'string' && /^#[0-9a-f]{6}$/i.test(color)) return color;
  for (const value of Object.values(liquid.values)) {
    if (value?.type !== 'tags' || !Array.isArray(value.value)) continue;
    const tag = tags.find((t) => t.id === (value.value as string[])[0]);
    if (tag) return paletteColor(tag.paletteId);
  }
  let hash = 0;
  for (const char of liquid.name) hash = (hash * 31 + char.charCodeAt(0)) >>> 0;
  return palette[hash % palette.length][2];
}
function Bottle({ name, color }: { name: string; color: string }) {
  const body = 'M27 34h30v15c0 7 14 8 14 22v76c0 11-9 17-29 17s-29-6-29-17V71c0-14 14-15 14-22Z';
  return (
    <svg className="bottle" viewBox="0 0 84 170" aria-hidden="true">
      <path d="M29 4h26v23H29z" fill="#161c16" />
      <rect x="26" y="25" width="32" height="10" rx="3" fill="#242d23" />
      <path d={body} fill="#b9c4a7" fillOpacity=".82" />
      <path d={body} fill="#ffffff0c" stroke="#ffffff30" />
      <path d="M20 71v71" stroke="#ffffff33" strokeWidth="3" strokeLinecap="round" />
      <rect x="13" y="72" width="58" height="72" rx="2" fill={color} />
      <path d="M13 130h58v14H13z" fill="#172013" />
      <text
        x="42"
        y="104"
        textAnchor="middle"
        fill="#172013"
        fontFamily="Arial,sans-serif"
        fontSize="16"
        fontWeight="700"
      >
        {name.trim().slice(0, 1).toLocaleUpperCase('de') || '?'}
      </text>
    </svg>
  );
}
/**
 * The one image slot for a liquid: square, filled, on card, dashboard, table and
 * detail alike. `compact` swaps the bottle for a drop where a bottle would be illegible.
 */
export function LiquidImage({
  liquid,
  tags,
  compact = false,
  full = false,
  stripped = false,
}: {
  liquid: Liquid;
  tags?: Tag[];
  compact?: boolean;
  /** Detail view: the full-resolution crop instead of the thumbnail. */
  full?: boolean;
  /** Images switched off in "Ansicht": a slim colour band, no photo. */
  stripped?: boolean;
}) {
  const { preferences, fields } = useApp();
  const role = fields.find((f) => f.id === preferences.roles.image && !f.archived);
  const id = role && liquid.values[role.id]?.value;
  const color = liquidColor(liquid, preferences.roles.color, tags);
  const art = { '--art-color': color } as React.CSSProperties;
  if (stripped)
    return (
      <div className="item-stage stripped" style={art}>
        <Icon name="liquids" />
      </div>
    );
  return (
    <div className={'item-stage ' + (compact ? 'compact' : '')} style={art}>
      {typeof id === 'string' ? (
        <img
          className="item-image"
          loading={full ? undefined : 'lazy'}
          decoding="async"
          src={'/api/v1/media/' + id + (full ? '' : '?variant=thumbnail')}
          alt=""
        />
      ) : compact ? (
        <Icon name="liquids" />
      ) : (
        <Bottle name={liquid.name} color={color} />
      )}
    </div>
  );
}
