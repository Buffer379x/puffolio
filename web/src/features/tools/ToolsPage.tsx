import { PageHeader, ItemCard } from '../../components/ui';
import { useApp } from '../../app/state';
export function ToolsPage() {
  const { navigate } = useApp();
  return (
    <>
      <PageHeader title="Tools" />
      <div className="liquid-grid">
        <a
          className="tool-card-link"
          href="/tools/nicotine"
          onClick={(e) => {
            e.preventDefault();
            navigate('/tools/nicotine');
          }}
        >
          <ItemCard
            image={<img className="item-image" src="/tools/nicotine.svg" alt="" />}
            title="Nikotinrechner"
          />
        </a>
      </div>
    </>
  );
}
