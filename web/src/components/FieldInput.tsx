import { useDraftGuard } from './useDraftGuard';
import { invalidateTags, useTagRevision } from '../app/tagEvents';
import { DateInput } from './DateInput';
import { useState, useId, useEffect, useRef } from 'react';
import type { Field, Scalar, Tag, Media } from '../app/types';
import { api, upload } from '../app/api';
import { useAction, useLoad } from '../app/state';
import { Button, Dialog, ErrorMessage, FormField, IconButton } from './ui';
import { PalettePicker, paletteColor } from './PalettePicker';
import { ImageCropEditor } from './ImageCropEditor';
export function TagDialog({
  fieldId,
  tag,
  onClose,
  onSaved,
}: {
  fieldId: string;
  tag?: Tag;
  onClose: () => void;
  onSaved: (t: Tag) => void;
}) {
  const [name, setName] = useState(tag?.name || '');
  const [color, setColor] = useState(tag?.paletteId || 'lime');
  const guard = useDraftGuard(
    name !== (tag?.name || '') || color !== (tag?.paletteId || 'lime'),
    onClose,
  );
  const a = useAction();
  return (
    <Dialog
      title={tag ? 'Tag bearbeiten' : 'Tag hinzufügen'}
      width="compact"
      onClose={guard.close}
      footer={
        <>
          <Button onClick={guard.close}>Abbrechen</Button>
          <Button
            variant="primary"
            disabled={a.busy || !name.trim()}
            onClick={() =>
              void a.run(async () => {
                const saved = await api<Tag>(
                  `/fields/${fieldId}/tags${tag ? '/' + tag.id : ''}`,
                  tag ? 'PATCH' : 'POST',
                  { name, paletteId: color, version: tag?.version },
                );
                invalidateTags();
                onSaved(saved);
              })
            }
          >
            Speichern
          </Button>
        </>
      }
    >
      <FormField label="Tagname">
        {(bindings) => (
          <input
            {...bindings}
            aria-label="Tagname"
            value={name}
            onChange={(e) => setName(e.target.value)}
            maxLength={100}
          />
        )}
      </FormField>
      <PalettePicker value={color} onChange={setColor} />
      {guard.confirmation}
      <ErrorMessage error={a.error} />
    </Dialog>
  );
}
export function TagsInput({
  field,
  value,
  onChange,
  describedBy,
  invalid = false,
}: {
  field: Field;
  value: Scalar;
  onChange: (v: Scalar) => void;
  describedBy?: string;
  invalid?: boolean;
}) {
  const revision = useTagRevision();
  const tags = useLoad(
    (s) => api<{ items: Tag[] }>(`/fields/${field.id}/tags`, 'GET', undefined, s),
    [field.id, revision],
  );
  const [add, setAdd] = useState(false);
  const selected = Array.isArray(value) ? value : [];
  return (
    <>
      <div className="tag-options">
        {tags.data?.items.map((t) => (
          <label key={t.id} className="check-row">
            <input
              type="checkbox"
              aria-describedby={describedBy}
              aria-invalid={invalid}
              checked={selected.includes(t.id)}
              onChange={(e) =>
                onChange(
                  e.target.checked ? [...selected, t.id] : selected.filter((id) => id !== t.id),
                )
              }
            />
            <span className="tag" style={{ backgroundColor: paletteColor(t.paletteId) }}>
              {t.name}
            </span>
          </label>
        ))}
        <Button onClick={() => setAdd(true)}>Tag hinzufügen</Button>
      </div>
      <ErrorMessage error={tags.error} />
      {add && (
        <TagDialog
          fieldId={field.id}
          onClose={() => setAdd(false)}
          onSaved={(t) => {
            onChange([...selected, t.id]);
            setAdd(false);
          }}
        />
      )}
    </>
  );
}
export function RatingInput({
  label,
  stars,
  value,
  onChange,
  describedBy,
  invalid = false,
  grouped = false,
}: {
  label: string;
  stars: number;
  value: Scalar;
  onChange: (v: Scalar) => void;
  describedBy?: string;
  invalid?: boolean;
  grouped?: boolean;
}) {
  const name = useId();
  return (
    <div
      className="rating-input"
      role={grouped ? undefined : 'group'}
      aria-label={grouped ? undefined : label}
      aria-describedby={describedBy}
    >
      {Array.from({ length: stars }, (_, i) => (
        <label
          className={
            'rating-option ' + (typeof value === 'number' && value >= i + 1 ? 'selected' : '')
          }
          key={i}
        >
          <input
            type="radio"
            aria-describedby={describedBy}
            aria-invalid={invalid}
            name={name}
            aria-label={`${label}: ${i + 1} von ${stars}`}
            checked={value === i + 1}
            onChange={() => onChange(i + 1)}
          />
          <span aria-hidden="true">★</span>
        </label>
      ))}
      <IconButton icon="close" label={`${label} entfernen`} onClick={() => onChange(null)} />
    </div>
  );
}
export function FieldInput({
  field,
  value,
  onChange,
  error,
  isNew = false,
  compact = false,
}: {
  field: Field;
  value: Scalar;
  onChange: (v: Scalar) => void;
  error?: string;
  isNew?: boolean;
  compact?: boolean;
}) {
  const [media, setMedia] = useState<Media>();
  const uploadController = useRef<AbortController | null>(null);
  useEffect(() => () => uploadController.current?.abort(), []);
  const a = useAction();
  const label = field.label;
  const grouped = ['rating', 'tags', 'image', 'color'].includes(field.type);
  const hint =
    isNew && field.type === 'date' && field.config.defaultToday && value === null
      ? 'Heute beim Anlegen'
      : undefined;
  return (
    <FormField
      label={label}
      error={error}
      hint={hint}
      required={field.required}
      compact={compact}
      wide={['longtext', 'tags', 'image', 'rating'].includes(field.type)}
      group={grouped}
    >
      {(inputProps) => {
        switch (field.type) {
          case 'longtext':
            return (
              <textarea
                {...inputProps}
                value={String(value ?? '')}
                onChange={(e) => onChange(e.target.value || null)}
                maxLength={10000}
              />
            );
          case 'boolean':
            return (
              <select
                {...inputProps}
                value={value === null ? '' : String(value)}
                onChange={(e) => onChange(e.target.value === '' ? null : e.target.value === 'true')}
              >
                <option value="">Nicht gesetzt</option>
                <option value="true">Ja</option>
                <option value="false">Nein</option>
              </select>
            );
          case 'rating':
            return (
              <RatingInput
                label={label}
                stars={field.config.stars || 5}
                value={value}
                onChange={onChange}
                grouped
                describedBy={inputProps['aria-describedby']}
                invalid={inputProps['aria-invalid']}
              />
            );
          case 'tags':
            return (
              <TagsInput
                field={field}
                value={value}
                onChange={onChange}
                describedBy={inputProps['aria-describedby']}
                invalid={inputProps['aria-invalid']}
              />
            );
          case 'select':
            return (
              <select
                {...inputProps}
                value={String(value ?? '')}
                onChange={(e) => onChange(e.target.value || null)}
              >
                <option value="">Nicht gesetzt</option>
                {field.config.options
                  ?.filter((o) => !o.archived || o.id === value)
                  .map((o) => (
                    <option key={o.id} value={o.id}>
                      {o.label}
                      {o.archived ? ' (archiviert)' : ''}
                    </option>
                  ))}
              </select>
            );
          case 'image':
            return (
              <>
                <div className="image-control">
                  {value && (
                    <img className="item-image" src={'/api/v1/media/' + value} alt={label} />
                  )}
                  <label className="button secondary upload-button">
                    {a.busy ? 'Hochladen…' : 'Bild auswählen'}
                    <input
                      type="file"
                      id={inputProps.id}
                      aria-describedby={inputProps['aria-describedby']}
                      aria-invalid={inputProps['aria-invalid']}
                      accept="image/jpeg,image/png,image/webp"
                      aria-label={`${label} hochladen`}
                      disabled={a.busy}
                      onChange={(e) => {
                        const file = e.target.files?.[0];
                        e.target.value = '';
                        if (file)
                          void a.run(async () => {
                            if (file.size > 8 * 1024 * 1024)
                              throw new Error('Bild darf maximal 8 MiB groß sein.');
                            uploadController.current?.abort();
                            const controller = new AbortController();
                            uploadController.current = controller;
                            setMedia(await upload<Media>('/media', file, controller.signal));
                          });
                      }}
                    />
                  </label>
                  {a.busy && (
                    <Button onClick={() => uploadController.current?.abort()}>
                      Upload abbrechen
                    </Button>
                  )}
                  {value && (
                    <div className="actions">
                      <Button
                        onClick={() =>
                          void a.run(async () =>
                            setMedia(await api<Media>('/media/' + value + '/info')),
                          )
                        }
                      >
                        Ausschnitt ändern
                      </Button>
                      <Button onClick={() => onChange(null)}>Bild entfernen</Button>
                    </div>
                  )}
                  <ErrorMessage error={a.error} />
                </div>
                {media && (
                  <ImageCropEditor
                    media={media}
                    onCancel={() => setMedia(undefined)}
                    onAccept={(m) => {
                      onChange(m.id);
                      setMedia(undefined);
                    }}
                  />
                )}
              </>
            );
          case 'date':
            return (
              <>
                <DateInput
                  id={inputProps.id}
                  label={label}
                  labelledBy={inputProps['aria-labelledby']}
                  min={field.config.min}
                  max={field.config.max}
                  value={String(value ?? '')}
                  invalid={!!error}
                  describedBy={inputProps['aria-describedby']}
                  onChange={(v) => onChange(v || null)}
                />
              </>
            );
          case 'color':
            return (
              <div className="input-unit">
                <input
                  {...inputProps}
                  type="text"
                  placeholder="#RRGGBB"
                  maxLength={7}
                  value={String(value ?? '')}
                  onChange={(e) => onChange(e.target.value || null)}
                />
                <input
                  type="color"
                  aria-describedby={inputProps['aria-describedby']}
                  aria-invalid={inputProps['aria-invalid']}
                  aria-label={`${label} auswählen`}
                  value={
                    typeof value === 'string' && /^#[0-9a-f]{6}$/i.test(value) ? value : '#C5E879'
                  }
                  onChange={(e) => onChange(e.target.value)}
                />
              </div>
            );
          default:
            return (
              <div className="input-unit">
                <input
                  {...inputProps}
                  inputMode={field.type === 'number' ? 'decimal' : undefined}
                  value={String(value ?? '')}
                  maxLength={field.type === 'text' ? 500 : 32}
                  onChange={(e) => onChange(e.target.value || null)}
                />
                {field.config.unit && <span>{field.config.unit}</span>}
              </div>
            );
        }
      }}
    </FormField>
  );
}
