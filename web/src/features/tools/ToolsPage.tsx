import { useId } from 'react';
import { PageHeader, ItemCard } from '../../components/ui';
import { useApp } from '../../app/state';
export function ToolsPage() {
  const { navigate } = useApp();
  const titleId = useId();
  return (
    <>
      <PageHeader title="Tools" />
      <div className="liquid-grid">
        <a
          className="tool-card-link"
          aria-labelledby={titleId}
          href="/tools/nicotine"
          onClick={(e) => {
            e.preventDefault();
            navigate('/tools/nicotine');
          }}
        >
          <ItemCard
            image={<img className="item-image" src="/tools/nicotine.svg" alt="" />}
            title={<span id={titleId}>Nikotinrechner</span>}
          />
        </a>
      </div>
    </>
  );
}
