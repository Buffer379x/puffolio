import { useState } from 'react';
import { useApp, useLoad } from '../../app/state';
import { api } from '../../app/api';
import type { Tag } from '../../app/types';
import { TagDialog } from '../../components/FieldInput';
import { paletteColor } from '../../components/PalettePicker';
import { Button, ErrorMessage, Empty } from '../../components/ui';
export function TagsSettings() {
  const { fields } = useApp();
  const tagFields = fields.filter((f) => f.type === 'tags' && !f.archived);
  const [fieldId, setFieldId] = useState(tagFields[0]?.id || '');
  const [editing, setEditing] = useState<{ tag?: Tag }>();
  const load = useLoad(
    (s) =>
      fieldId
        ? api<{ items: Tag[] }>('/fields/' + fieldId + '/tags', 'GET', undefined, s)
        : Promise.resolve({ items: [] }),
    [fieldId],
  );
  return (
    <>
      <div className="settings-section-heading">
        <h2>Tags</h2>
        <Button variant="primary" disabled={!fieldId} onClick={() => setEditing({})}>
          Tag hinzufügen
        </Button>
      </div>
      <label className="form-field">
        Tagfeld
        <select aria-label="Tagfeld" value={fieldId} onChange={(e) => setFieldId(e.target.value)}>
          {tagFields.map((f) => (
            <option key={f.id} value={f.id}>
              {f.label}
            </option>
          ))}
        </select>
      </label>
      <ErrorMessage error={load.error} />
      {!tagFields.length && <Empty>Erst ein Tagfeld anlegen.</Empty>}
      {load.data?.items.map((t) => (
        <div className="setting-line" key={t.id}>
          <span className="tag" style={{ backgroundColor: paletteColor(t.paletteId) }}>
            {t.name}
          </span>
          <Button onClick={() => setEditing({ tag: t })}>Bearbeiten</Button>
        </div>
      ))}
      {editing && (
        <TagDialog
          tag={editing.tag}
          fieldId={fieldId}
          onClose={() => setEditing(undefined)}
          onSaved={() => {
            setEditing(undefined);
            load.reload();
          }}
        />
      )}
    </>
  );
}
