import { useApp } from '../../app/state';
import { PageHeader } from '../../components/ui';
import { FieldsSettings } from '../fields/FieldsSettings';
import { TagsSettings } from './TagsSettings';
import { ProfileSettings } from './ProfileSettings';
import { DataSettings } from './DataSettings';
export function SettingsPage({ query }: { query: string }) {
  const { navigate } = useApp();
  const section = new URLSearchParams(query).get('section') || 'fields';
  return (
    <>
      <PageHeader title="Einstellungen" />
      <nav className="settings-tabs" aria-label="Einstellungen">
        {[
          ['fields', 'Felder'],
          ['tags', 'Tags'],
          ['profile', 'Profil'],
          ['data', 'Daten'],
        ].map(([id, label]) => (
          <a
            key={id}
            href={'/settings?section=' + id}
            aria-current={section === id ? 'page' : undefined}
            onClick={(e) => {
              e.preventDefault();
              navigate('/settings?section=' + id);
            }}
          >
            {label}
          </a>
        ))}
      </nav>
      {section === 'fields' ? (
        <FieldsSettings />
      ) : section === 'tags' ? (
        <TagsSettings />
      ) : section === 'profile' ? (
        <ProfileSettings />
      ) : section === 'data' ? (
        <DataSettings />
      ) : (
        <p>Bereich nicht gefunden.</p>
      )}
    </>
  );
}
