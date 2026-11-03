import { useState } from 'react';
import { render, screen, fireEvent, act, cleanup } from '@testing-library/react';
import { beforeAll, afterEach, it, expect, vi } from 'vitest';
import { ImageCropEditor } from './ImageCropEditor';
beforeAll(() => {
  HTMLDialogElement.prototype.showModal = function () {
    this.open = true;
  };
  HTMLDialogElement.prototype.close = function () {
    this.open = false;
  };
});
afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});
it.each(['cancel', 'escape'])(
  'ignores a late accepted crop after %s so the parent image reference stays unchanged',
  async (action) => {
    let resolve!: (r: Response) => void;
    vi.stubGlobal(
      'fetch',
      () =>
        new Promise<Response>((r) => {
          resolve = r;
        }),
    );
    function Parent() {
      const [image, setImage] = useState('old-media');
      const [open, setOpen] = useState(true);
      return (
        <>
          <output aria-label="Bildreferenz">{image}</output>
          {open && (
            <ImageCropEditor
              media={{
                id: 'old-media',
                width: 1600,
                height: 1200,
                url: '/old',
                masterUrl: '/master',
                thumbnailUrl: '/thumb',
              }}
              onCancel={() => setOpen(false)}
              onAccept={(m) => {
                setImage(m.id);
                setOpen(false);
              }}
            />
          )}
        </>
      );
    }
    render(<Parent />);
    fireEvent.click(screen.getByRole('button', { name: 'Ausschnitt übernehmen' }));
    if (action === 'cancel') fireEvent.click(screen.getByRole('button', { name: 'Abbrechen' }));
    else fireEvent(screen.getByRole('dialog'), new Event('cancel', { cancelable: true }));
    await act(async () =>
      resolve(
        new Response(
          JSON.stringify({
            id: 'new-media',
            width: 1600,
            height: 1200,
            url: '/new',
            masterUrl: '/master',
            thumbnailUrl: '/thumb',
          }),
        ),
      ),
    );
    expect(screen.getByLabelText('Bildreferenz')).toHaveTextContent('old-media');
  },
);
