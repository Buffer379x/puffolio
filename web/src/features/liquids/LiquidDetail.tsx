import { useApp, useAction, useCanEdit } from '../../app/state';
import type { Liquid, Tag } from '../../app/types';
import { api } from '../../app/api';
import { useState } from 'react';
import { Dialog, Button, ErrorMessage, Confirm, Icon } from '../../components/ui';
import { FieldValue } from '../../components/FieldValue';
import { LiquidImage } from '../../components/LiquidImage';
import { ShelfBar } from '../../components/ShelfBar';
import { formatDate } from '../fields/values';
export function LiquidDetail({
  liquid: initial,
  tags,
  onClose,
  onEdit,
  onDuplicate,
  onChange,
  onTrashed,
}: {
  liquid: Liquid;
  tags: Tag[];
  onClose: () => void;
  onEdit: () => void;
  onDuplicate: () => void;
  onChange: (liquid?: Liquid) => void;
  onTrashed?: (liquid: Liquid) => void;
}) {
  const { fields, preferences } = useApp();
  const [liquid, setLiquid] = useState(initial);
  const a = useAction();
  const canEdit = useCanEdit();
  const [remove, setRemove] = useState(false);
  const shown = fields.filter(
    (f) => !f.archived && f.visibility.detail && f.id !== preferences.roles.image,
  );
  return (
    <>
      <Dialog
        title={liquid.name}
        width="wide"
        onClose={onClose}
        footer={
          canEdit ? (
            <>
              <Button variant="danger" className="push-left" onClick={() => setRemove(true)}>
                Löschen
              </Button>
              <Button onClick={onDuplicate}>
                <Icon name="copy" />
                Duplizieren
              </Button>
              <Button
                aria-pressed={liquid.favorite}
                onClick={() =>
                  void a.run(async () => {
                    const next = await api<Liquid>('/liquids/' + liquid.id, 'PATCH', {
                      version: liquid.version,
                      favorite: !liquid.favorite,
                    });
                    setLiquid(next);
                    onChange(next);
                  })
                }
              >
                <Icon name="heart" />
                {liquid.favorite ? 'Favorit entfernen' : 'Favorit'}
              </Button>
              <Button variant="primary" onClick={onEdit}>
                <Icon name="edit" />
                Bearbeiten
              </Button>
            </>
          ) : undefined
        }
      >
        {preferences.images && (
          <div className="details-visual">
            <LiquidImage liquid={liquid} tags={tags} full />
          </div>
        )}
        <dl className="details-grid">
          {liquid.shelfLife && (
            <div className="full">
              <dt>Haltbarkeit bis {formatDate(liquid.shelfLife.expiresOn)}</dt>
              <dd>
                <ShelfBar liquid={liquid} />
              </dd>
            </div>
          )}
          {shown.map((f) => (
            <div key={f.id} className={f.type === 'longtext' || f.type === 'tags' ? 'full' : ''}>
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
              const trashed = await api<Liquid>('/liquids/' + liquid.id, 'DELETE', {
                version: liquid.version,
              });
              onChange();
              onClose();
              onTrashed?.(trashed);
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
