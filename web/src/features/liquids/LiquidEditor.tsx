import { FormGrid } from '../../components/FormGrid';
import { useState } from 'react';
import { api, APIError } from '../../app/api';
import { useApp, useAction, useDirty, useLoad } from '../../app/state';
import type { Liquid, Values } from '../../app/types';
import { canonicalNumber, changedValues } from '../fields/values';
import { FieldInput } from '../../components/FieldInput';
import { Dialog, Button, FormField, ErrorMessage, Confirm } from '../../components/ui';
type LiquidEditorProps = {
  liquid?: Liquid;
  duplicate?: boolean;
  onClose: () => void;
  onSaved: (liquid: Liquid) => void;
};
/** A new opening gets a fresh owned record; an already mounted form never rebases itself. */
export function LiquidEditor(props: LiquidEditorProps) {
  const existing = !!props.liquid && !props.duplicate;
  const load = useLoad(
    (signal) =>
      existing
        ? api<Liquid>('/liquids/' + props.liquid!.id, 'GET', undefined, signal)
        : Promise.resolve(props.liquid),
    [existing, props.liquid?.id],
  );
  if (existing && (load.loading || !load.data))
    return (
      <Dialog
        title="Liquid bearbeiten"
        width="wide"
        liquid
        onClose={props.onClose}
        footer={<Button onClick={props.onClose}>Schließen</Button>}
      >
        {load.loading ? (
          <p role="status">Aktuellen Stand laden…</p>
        ) : (
          <>
            <ErrorMessage error={load.error} />
            <Button onClick={load.reload}>Erneut versuchen</Button>
          </>
        )}
      </Dialog>
    );
  return <LiquidEditorForm {...props} liquid={existing ? load.data : props.liquid} />;
}
function LiquidEditorForm({ liquid, duplicate = false, onClose, onSaved }: LiquidEditorProps) {
  const { fields, preferences } = useApp();
  const isNew = !liquid || duplicate;
  const [name, setName] = useState(liquid?.name || '');
  const [favorite, setFavorite] = useState(liquid?.favorite || false);
  const initial: Values =
    (duplicate && liquid
      ? Object.fromEntries(
          Object.entries(liquid.values).filter(([id]) =>
            fields.some((f) => f.id === id && !f.archived),
          ),
        )
      : liquid?.values) ||
    Object.fromEntries(
      fields
        .filter((f) => !f.archived && f.visibility.form && f.config.defaultValue !== undefined)
        .map((f) => [f.id, { type: f.type, value: f.config.defaultValue! }]),
    );
  const [values, setValues] = useState<Values>(initial);
  const [confirm, setConfirm] = useState(false);
  const [current, setCurrent] = useState<Liquid>();
  const [conflict, setConflict] = useState(false);
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({});
  const a = useAction();
  const dirty =
    name !== (liquid?.name || '') ||
    favorite !== (liquid?.favorite || false) ||
    JSON.stringify(values) !== JSON.stringify(initial);
  useDirty(dirty);
  const close = () => (dirty ? setConfirm(true) : onClose());
  const save = () =>
    void a.run(async () => {
      const changed = isNew ? { ...values } : changedValues(liquid!.values, values);
      const errors: Record<string, string> = {};
      for (const f of fields) {
        const v = changed[f.id];
        if (v && f.type === 'number' && typeof v.value === 'string') {
          try {
            changed[f.id] = { ...v, value: canonicalNumber(v.value, f.config.scale || 0) };
          } catch (e) {
            errors[f.id] = (e as Error).message;
          }
        }
      }
      setFieldErrors(errors);
      if (Object.keys(errors).length) throw new Error('Eingaben prüfen.');
      let result: Liquid;
      try {
        result = await api<Liquid>(
          isNew ? '/liquids' : '/liquids/' + liquid!.id,
          isNew ? 'POST' : 'PATCH',
          { name, favorite, version: isNew ? undefined : liquid!.version, values: changed },
        );
      } catch (e) {
        setConflict(e instanceof APIError && e.status === 409);
        throw e;
      }
      onSaved(result);
    });
  return (
    <>
      <Dialog
        title={isNew ? 'Liquid hinzufügen' : 'Liquid bearbeiten'}
        width="wide"
        liquid
        onClose={close}
        footer={
          <>
            <Button onClick={close}>Abbrechen</Button>
            <Button variant="primary" disabled={a.busy || !name.trim()} onClick={save}>
              Speichern
            </Button>
          </>
        }
      >
        <form
          id="liquid-form"
          onSubmit={(e) => {
            e.preventDefault();
            save();
          }}
        >
          <FormGrid>
            <FormField
              label="Name"
              wide
              error={a.error instanceof APIError ? a.error.fields.name : undefined}
            >
              {(bindings) => (
                <input
                  {...bindings}
                  autoFocus
                  aria-label="Name"
                  required
                  maxLength={200}
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                />
              )}
            </FormField>
            {fields
              .filter((f) => !f.archived && f.visibility.form)
              .map((f) => (
                <FieldInput
                  key={f.id}
                  field={f}
                  value={values[f.id]?.value ?? null}
                  isNew={isNew}
                  fallback={
                    f.id === preferences.roles.shelfOverride && preferences.shelfLifeMonths > 0
                      ? `Leer: Standard (${preferences.shelfLifeMonths} Monate)`
                      : undefined
                  }
                  error={
                    fieldErrors[f.id] ||
                    (a.error instanceof APIError ? a.error.fields[f.id] : undefined)
                  }
                  onChange={(v) =>
                    setValues({ ...values, [f.id]: v === null ? null : { type: f.type, value: v } })
                  }
                />
              ))}
            <label className="check-row full">
              <input
                type="checkbox"
                checked={favorite}
                onChange={(e) => setFavorite(e.target.checked)}
              />
              Favorit
            </label>
          </FormGrid>
        </form>
        <ErrorMessage error={a.error} />
        {conflict && (
          <>
            <Button
              onClick={() =>
                void a.run(async () => setCurrent(await api<Liquid>('/liquids/' + liquid!.id)))
              }
            >
              Aktuellen Stand vergleichen
            </Button>
            {current && (
              <section className="comparison">
                <h3>Aktueller Stand</h3>
                <p>
                  {current.name} · Version {current.version}
                </p>
                <p>
                  Der Entwurf bleibt unverändert. Änderungen bewusst verwerfen und schließen. Beim
                  erneuten Öffnen wird der aktuelle Stand geladen.
                </p>
              </section>
            )}
          </>
        )}
      </Dialog>
      {confirm && (
        <Confirm
          title="Änderungen verwerfen?"
          onCancel={() => setConfirm(false)}
          onConfirm={onClose}
        >
          <p>Ungespeicherte Änderungen gehen verloren.</p>
        </Confirm>
      )}
    </>
  );
}
