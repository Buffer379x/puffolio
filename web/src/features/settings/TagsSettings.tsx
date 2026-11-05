import { useEffect, useRef, useState } from 'react';
import { useApp, useAction, useLoad } from '../../app/state';
import { api, download } from '../../app/api';
import { invalidateTags } from '../../app/tagEvents';
import type { Field, Tag, TagUsage } from '../../app/types';
import { TagDialog } from '../../components/FieldInput';
import { paletteColor } from '../../components/PalettePicker';
import { Button, Confirm, ErrorMessage, Empty, IconButton } from '../../components/ui';
import { saveBlob } from '../fields/values';
function TagUsageWarning({ usage, field }: { usage: TagUsage; field?: Field }) {
  const { liquids, trashed, isDefault, widgets, views, orphaned } = usage;
  const irreversible = 'Das lässt sich nicht rückgängig machen.';
  return (
    <>
      {liquids ? (
        <p>
          Dieser Tag ist {liquids} {liquids === 1 ? 'Liquid' : 'Liquids'} zugeordnet
          {trashed ? `, davon ${trashed} im Papierkorb` : ''}. Er wird dort entfernt. {irreversible}
        </p>
      ) : (
        <p>Er ist keinem Liquid zugeordnet. {irreversible}</p>
      )}
      {!!orphaned && (
        <p>
          „{field?.label}“ ist ein Pflichtfeld: {orphaned === 1 ? '1 Liquid hätte' : `${orphaned} Liquids hätten`}{' '}
          danach keinen Tag mehr. Die Pflicht gilt dann nur noch für neu angelegte Liquids.
        </p>
      )}
      {isDefault && <p>Der Tag ist Standardwert dieses Felds. Neue Liquids erhalten ihn nicht mehr.</p>}
      {!!widgets && (
        <p>
          {widgets === 1 ? '1 Dashboard-Widget filtert' : `${widgets} Dashboard-Widgets filtern`}{' '}
          nach diesem Tag. Der Filter wird entfernt.
        </p>
      )}
      {!!views && (
        <p>
          {views === 1 ? '1 gespeicherte Ansicht filtert' : `${views} gespeicherte Ansichten filtern`}{' '}
          nach diesem Tag. Der Filter wird entfernt.
        </p>
      )}
    </>
  );
}
export function TagsSettings() {
  const { fields, refresh } = useApp();
  const tagFields = fields.filter((f) => f.type === 'tags' && !f.archived);
  const [fieldId, setFieldId] = useState(tagFields[0]?.id || '');
  const [editing, setEditing] = useState<{ tag?: Tag }>();
  const [deletion, setDeletion] = useState<{ tag: Tag; usage: TagUsage }>();
  const a = useAction();
  const addButton = useRef<HTMLButtonElement>(null);
  const focusAdd = useRef(false);
  const load = useLoad(
    (s) =>
      fieldId
        ? api<{ items: Tag[] }>('/fields/' + fieldId + '/tags', 'GET', undefined, s)
        : Promise.resolve({ items: [] }),
    [fieldId],
  );
  // The delete button goes with its row, so focus moves on once the list has been redrawn.
  useEffect(() => {
    if (focusAdd.current && !load.loading) {
      focusAdd.current = false;
      addButton.current?.focus();
    }
  }, [load.loading]);
  const path = (t: Tag) => '/fields/' + fieldId + '/tags/' + t.id;
  const remove = (t: Tag) =>
    void a.run(async () => {
      await api(path(t), 'DELETE', { version: t.version, confirm: true });
      invalidateTags();
      // The field's default value may have changed with it.
      await refresh();
      setDeletion(undefined);
      focusAdd.current = true;
      load.reload();
    });
  return (
    <>
      <div className="settings-section-heading">
        <h2>Tags</h2>
        <Button
          ref={addButton}
          variant="primary"
          disabled={!fieldId}
          onClick={() => setEditing({})}
        >
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
      <ErrorMessage error={load.error || (deletion ? undefined : a.error)} />
      {!tagFields.length && <Empty>Erst ein Tagfeld anlegen.</Empty>}
      {load.data?.items.map((t) => (
        <div className="setting-line" key={t.id}>
          <span className="tag" style={{ backgroundColor: paletteColor(t.paletteId) }}>
            {t.name}
          </span>
          <div className="actions table-actions">
            <Button onClick={() => setEditing({ tag: t })}>Bearbeiten</Button>
            <IconButton
              icon="trash"
              label={`${t.name} löschen`}
              onClick={() => {
                // Not disabled while busy: that would drop the focus the dialog returns to.
                if (a.busy) return;
                void a.run(async () => {
                  setDeletion({ tag: t, usage: await api<TagUsage>(path(t) + '/usage') });
                });
              }}
            />
          </div>
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
      {deletion && (
        <Confirm
          title={`Tag „${deletion.tag.name}“ löschen?`}
          danger
          onCancel={() => {
            setDeletion(undefined);
            a.setError(undefined);
            // After a conflict the next attempt needs the tag as it is now.
            load.reload();
          }}
          onConfirm={() => {
            if (!a.busy) remove(deletion.tag);
          }}
        >
          <TagUsageWarning
            usage={deletion.usage}
            field={tagFields.find((f) => f.id === fieldId)}
          />
          {!!deletion.usage.liquids && (
            <Button
              onClick={() =>
                void a.run(async () =>
                  saveBlob(await download('/exports', {}), 'puffolio-sammlung.zip'),
                )
              }
            >
              Vorher Sammlung exportieren
            </Button>
          )}
          <ErrorMessage error={a.error} />
        </Confirm>
      )}
    </>
  );
}
