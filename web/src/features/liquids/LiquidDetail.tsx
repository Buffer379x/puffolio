import { useApp, useAction } from '../../app/state';
import type { Liquid, Tag } from '../../app/types';
import { api } from '../../app/api';
import { useState } from 'react';
import { Dialog, Button, ErrorMessage, Confirm, Icon } from '../../components/ui';
import { FieldValue } from '../../components/FieldValue';
export function LiquidDetail({
  liquid,
  tags,
  onClose,
  onEdit,
  onDuplicate,
  onChange,
}: {
  liquid: Liquid;
  tags: Tag[];
  onClose: () => void;
  onEdit: () => void;
  onDuplicate: () => void;
  onChange: () => void;
}) {
  const { fields } = useApp();
  const a = useAction();
  const [remove, setRemove] = useState(false);
  return (
    <>
      <Dialog
        title={liquid.name}
        width="wide"
        onClose={onClose}
        footer={
          <>
            <Button variant="danger" onClick={() => setRemove(true)}>
              Löschen
            </Button>
            <Button onClick={onDuplicate}>Duplizieren</Button>
            <Button
              onClick={() =>
                void a.run(async () => {
                  await api('/liquids/' + liquid.id, 'PATCH', {
                    version: liquid.version,
                    favorite: !liquid.favorite,
                  });
                  onChange();
                  onClose();
                })
              }
            >
              <Icon name="star" />
              {liquid.favorite ? 'Favorit entfernen' : 'Favorit'}
            </Button>
            <Button variant="primary" onClick={onEdit}>
              Bearbeiten
            </Button>
          </>
        }
      >
        <dl className="details-grid">
          {fields
            .filter((f) => !f.archived && f.visibility.detail)
            .map((f) => (
              <div key={f.id}>
                <dt>{f.label}</dt>
                <dd>
                  <FieldValue field={f} value={liquid.values[f.id]?.value} tags={tags} />
                </dd>
              </div>
            ))}
        </dl>
        <ErrorMessage error={a.error} />
      </Dialog>
      {remove && (
        <Confirm
          title="Liquid löschen?"
          danger
          onCancel={() => setRemove(false)}
          onConfirm={() =>
            void a.run(async () => {
              await api('/liquids/' + liquid.id, 'DELETE', { version: liquid.version });
              onChange();
              onClose();
            })
          }
        >
          <p>Das Liquid bleibt 30 Tage im Papierkorb.</p>
          <ErrorMessage error={a.error} />
        </Confirm>
      )}
    </>
  );
}
