import type { Liquid } from '../app/types';
import { formatDate } from '../features/fields/values';
import { shelfColor } from '../features/liquids/shelfLife';
export function ShelfBar({ liquid }: { liquid: Liquid }) {
  const life = liquid.shelfLife;
  if (!life) return null;
  const expiresOn = formatDate(life.expiresOn);
  const text = life.expired ? 'Abgelaufen' : `${life.percent} %`;
  return (
    <div
      className={'shelf ' + (life.expired ? 'expired' : '')}
      role="meter"
      aria-label="Haltbarkeit"
      aria-valuemin={0}
      aria-valuemax={100}
      aria-valuenow={life.percent}
      aria-valuetext={`${text}, bis ${expiresOn}`}
      title={`Haltbar bis ${expiresOn}`}
    >
      <div className="shelf-track">
        <div
          className="shelf-fill"
          style={{ width: `${life.percent}%`, backgroundColor: shelfColor(life.percent) }}
        />
      </div>
      <span className="shelf-label">{text}</span>
    </div>
  );
}
