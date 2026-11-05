import { useState, type ReactNode } from 'react';
import type { Segment } from '../../app/types';
import { paletteColor } from '../../components/PalettePicker';
import { shelfColor } from '../liquids/shelfLife';
/** Fixed order, so a part keeps its colour however the others change. */
const seriesColor = (segment: Segment, index: number) => {
  if (segment.paletteId) return paletteColor(segment.paletteId);
  if (segment.id === 'none') return 'var(--surface-3)';
  if (segment.id === 'other') return 'var(--faint)';
  return `var(--series-${(index % 6) + 1})`;
};
function ChartTable({ caption, segments }: { caption: string; segments: Segment[] }) {
  return (
    // A table ignores height and overflow itself, so the wrapper does the hiding.
    <div className="sr-only">
      <table>
        <caption>{caption}</caption>
        <tbody>
          {segments.map((s) => (
            <tr key={s.id}>
              <th scope="row">{s.label}</th>
              <td>{s.count}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
function Legend({
  items,
  active,
  onActive,
  onSelect,
}: {
  items: { segment: Segment; color: string }[];
  active?: string;
  onActive: (id?: string) => void;
  onSelect?: (segment: Segment) => boolean | void;
}) {
  return (
    <ul className="chart-legend">
      {items.map(({ segment, color }) => {
        const body: ReactNode = (
          <>
            <span className="legend-swatch" style={{ backgroundColor: color }} />
            <span className="legend-name" title={segment.label}>
              {segment.label}
            </span>
            <strong>{segment.count}</strong>
          </>
        );
        const canSelect = onSelect && segment.id !== 'none' && segment.id !== 'other';
        return (
          <li
            key={segment.id}
            className={active === segment.id ? 'active' : ''}
            onPointerEnter={() => onActive(segment.id)}
            onPointerLeave={() => onActive(undefined)}
          >
            {canSelect ? (
              <button type="button" onClick={() => onSelect(segment)}>
                {body}
              </button>
            ) : (
              body
            )}
          </li>
        );
      })}
    </ul>
  );
}
/** Pie with the total in its centre; the legend carries names and exact numbers. */
export function DonutChart({
  title,
  segments,
  totalLabel,
  onSelect,
}: {
  title: string;
  segments: Segment[];
  totalLabel: string;
  onSelect?: (segment: Segment) => void;
}) {
  const [active, setActive] = useState<string>();
  const total = segments.reduce((sum, s) => sum + s.count, 0);
  const shown = segments.find((s) => s.id === active);
  const radius = 42;
  const circumference = 2 * Math.PI * radius;
  // 2px of surface between neighbours; a single part stays a closed ring.
  const gap = segments.length > 1 ? 2 : 0;
  let offset = 0;
  return (
    <div className={'donut-chart ' + (active ? 'hovering' : '')}>
      <div className="donut">
        <svg viewBox="0 0 100 100" aria-hidden="true">
          <g transform="rotate(-90 50 50)">
            {segments.map((s, i) => {
              const length = (s.count / total) * circumference;
              const start = offset;
              offset += length;
              return (
                <circle
                  key={s.id}
                  className={active === s.id ? 'active' : ''}
                  cx="50"
                  cy="50"
                  r={radius}
                  fill="none"
                  stroke={seriesColor(s, i)}
                  strokeWidth="14"
                  strokeDasharray={`${Math.max(length - gap, 0.5)} ${circumference}`}
                  strokeDashoffset={-start}
                  onPointerEnter={() => setActive(s.id)}
                  onPointerLeave={() => setActive(undefined)}
                />
              );
            })}
          </g>
        </svg>
        <div className="donut-center">
          <strong>{shown ? shown.count : total}</strong>
          <span>
            {shown ? `${shown.label} · ${Math.round((shown.count / total) * 100)} %` : totalLabel}
          </span>
        </div>
      </div>
      <Legend
        items={segments.map((segment, i) => ({ segment, color: seriesColor(segment, i) }))}
        active={active}
        onActive={setActive}
        onSelect={onSelect}
      />
      <ChartTable caption={title} segments={segments} />
    </div>
  );
}
/** One measure over ordered steps: single hue, rounded tops on one baseline. */
export function ColumnChart({
  title,
  segments,
  showValues = false,
  labelEvery = 1,
  describe,
}: {
  title: string;
  segments: Segment[];
  showValues?: boolean;
  labelEvery?: number;
  describe: (segment: Segment) => string;
}) {
  const [active, setActive] = useState<string>();
  const max = Math.max(1, ...segments.map((s) => s.count));
  const shown = segments.find((s) => s.id === active);
  return (
    <div>
      <div className="column-caption">{shown ? describe(shown) : '\u00a0'}</div>
      <div className={'column-chart ' + (active ? 'hovering' : '')} aria-hidden="true">
        {segments.map((s) => (
          <div
            key={s.id}
            className={'column ' + (active === s.id ? 'active' : '')}
            onPointerEnter={() => setActive(s.id)}
            onPointerLeave={() => setActive(undefined)}
          >
            {showValues && <span className="column-value">{s.count}</span>}
            {s.count > 0 && (
              <div className="column-bar" style={{ height: `${(s.count / max) * 78}%` }} />
            )}
          </div>
        ))}
      </div>
      <div className="column-axis" aria-hidden="true">
        {segments.map((s, i) => (
          <span key={s.id}>{(segments.length - 1 - i) % labelEvery === 0 ? s.label : ''}</span>
        ))}
      </div>
      <ChartTable caption={title} segments={segments} />
    </div>
  );
}
const shelfShare: Record<string, number> = { fresh: 90, half: 50, soon: 15, expired: 0 };
/** Status of the whole collection, in the colours of the shelf-life bar. */
export function ShelfStatusChart({ title, segments }: { title: string; segments: Segment[] }) {
  const [active, setActive] = useState<string>();
  const items = segments.map((segment) => ({
    segment,
    color: shelfColor(shelfShare[segment.id] ?? 50),
  }));
  return (
    <div>
      <div className="status-bar" aria-hidden="true">
        {items
          .filter(({ segment }) => segment.count > 0)
          .map(({ segment, color }) => (
            <div
              key={segment.id}
              style={{ flexGrow: segment.count, backgroundColor: color }}
              onPointerEnter={() => setActive(segment.id)}
              onPointerLeave={() => setActive(undefined)}
            />
          ))}
      </div>
      <Legend items={items} active={active} onActive={setActive} />
      <ChartTable caption={title} segments={segments} />
    </div>
  );
}
