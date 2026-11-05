import { useApp } from '../../app/state';
import { cardLayout } from '../liquids/cardLayout';
import { LiquidCard } from '../liquids/LiquidCard';
import { cardOrder, previewLiquid } from './previewLiquid';
const ignore = () => undefined;
/** The real card with sample values. Field changes save at once, so it follows by itself. */
export function CardPreview() {
  const { fields, preferences } = useApp();
  const layout = cardLayout(fields, preferences);
  const today = new Date().toLocaleDateString('sv');
  const { liquid, tags } = previewLiquid(fields, preferences, today);
  const hidden = fields.filter(
    (f) => !f.archived && f.visibility.grid && !preferences.gridFields.includes(f.id),
  );
  return (
    <section className="section-card" aria-labelledby="card-preview-title">
      <h2 id="card-preview-title">Kachelvorschau</h2>
      <div className="card-preview">
        <div className="liquid-grid" inert>
          <LiquidCard
            liquid={liquid}
            layout={layout}
            tags={tags}
            onOpen={ignore}
            onEdit={ignore}
            onFavorite={ignore}
          />
        </div>
        <div>
          <h3 id="card-preview-order">Reihenfolge</h3>
          <ol aria-labelledby="card-preview-order">
            {cardOrder(layout).map((line, i) => (
              <li key={i}>{line}</li>
            ))}
          </ol>
          {hidden.length > 0 && (
            <p className="muted">
              In „Ansicht“ ausgeblendet: {hidden.map((f) => f.label).join(', ')}
            </p>
          )}
        </div>
      </div>
    </section>
  );
}
