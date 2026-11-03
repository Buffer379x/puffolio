import { useState, useRef, useEffect, type CSSProperties } from 'react';
import type { Media, Crop } from '../app/types';
import { api } from '../app/api';
import { useAction } from '../app/state';
import { cropAt, cropPosition } from './cropGeometry';
import { Dialog, Button, ErrorMessage } from './ui';
export function ImageCropEditor({
  media,
  onCancel,
  onAccept,
}: {
  media: Media;
  onCancel: () => void;
  onAccept: (m: Media) => void;
}) {
  const [position, setPosition] = useState(() =>
    media.crop ? cropPosition(media.width, media.height, media.crop) : { zoom: 1, x: 50, y: 50 },
  );
  const [drag, setDrag] = useState<{ x: number; y: number; startX: number; startY: number }>();
  const [maxZoom] = useState(() =>
    Math.max(5, media.crop ? cropPosition(media.width, media.height, media.crop).zoom : 5),
  );
  const active = useRef(true);
  const pending = useRef<AbortController | null>(null);
  useEffect(() => {
    active.current = true;
    return () => {
      active.current = false;
      pending.current?.abort();
    };
  }, []);
  const cancel = () => {
    active.current = false;
    pending.current?.abort();
    onCancel();
  };
  const a = useAction();
  const crop: Crop = cropAt(media.width, media.height, position.zoom, position.x, position.y);
  const style = {
    '--crop-width': `${100 / crop.width}%`,
    '--crop-height': `${100 / crop.height}%`,
    '--crop-left': `${(-crop.x / crop.width) * 100}%`,
    '--crop-top': `${(-crop.y / crop.height) * 100}%`,
  } as CSSProperties;
  return (
    <Dialog
      title="Bildausschnitt"
      width="wide"
      onClose={cancel}
      footer={
        <>
          <Button onClick={cancel}>Abbrechen</Button>
          <Button
            variant="primary"
            disabled={a.busy}
            onClick={() =>
              void a.run(async () => {
                const controller = new AbortController();
                pending.current = controller;
                const saved = await api<Media>(
                  '/media/' + media.id + '/crop',
                  'POST',
                  crop,
                  controller.signal,
                );
                if (active.current && !controller.signal.aborted) onAccept(saved);
              })
            }
          >
            Ausschnitt übernehmen
          </Button>
        </>
      }
    >
      <div className="crop-stage">
        <div
          className="crop-frame"
          style={style}
          onPointerDown={(e) => {
            e.currentTarget.setPointerCapture(e.pointerId);
            setDrag({ x: position.x, y: position.y, startX: e.clientX, startY: e.clientY });
          }}
          onPointerMove={(e) => {
            if (!drag) return;
            const r = e.currentTarget.getBoundingClientRect();
            const dx =
              (((e.clientX - drag.startX) / r.width) * 100 * crop.width) /
              Math.max(0.001, 1 - crop.width);
            const dy =
              (((e.clientY - drag.startY) / r.height) * 100 * crop.height) /
              Math.max(0.001, 1 - crop.height);
            setPosition({
              ...position,
              x: Math.max(0, Math.min(100, drag.x - dx)),
              y: Math.max(0, Math.min(100, drag.y - dy)),
            });
          }}
          onPointerUp={() => setDrag(undefined)}
          onPointerCancel={() => setDrag(undefined)}
        >
          <img draggable={false} src={media.masterUrl} alt="Bildausschnitt verschieben" />
        </div>
      </div>
      <div className="crop-controls">
        {(
          [
            ['zoom', 'Zoom', 1, maxZoom, 0.01],
            ['x', 'Horizontal', 0, 100, 0.1],
            ['y', 'Vertikal', 0, 100, 0.1],
          ] as const
        ).map(([key, label, min, max, step]) => (
          <label key={key}>
            {label}
            <input
              aria-label={label}
              type="range"
              min={min}
              max={max}
              step={step}
              value={position[key]}
              onChange={(e) => setPosition({ ...position, [key]: Number(e.target.value) })}
            />
          </label>
        ))}
      </div>
      <ErrorMessage error={a.error} />
    </Dialog>
  );
}
