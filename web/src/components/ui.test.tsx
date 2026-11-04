import { useState } from 'react';
import { render, screen, fireEvent, cleanup } from '@testing-library/react';
import { beforeAll, afterEach, it, expect, vi } from 'vitest';
import { Dialog, Button } from './ui';
import { RatingInput } from './FieldInput';
beforeAll(() => {
  HTMLDialogElement.prototype.showModal = function () {
    this.open = true;
  };
  HTMLDialogElement.prototype.close = function () {
    this.open = false;
  };
});
afterEach(cleanup);
it('treats Escape as cancel and restores the opener after closing', () => {
  function Demo() {
    const [open, set] = useState(false);
    return (
      <>
        <Button onClick={() => set(true)}>Öffnen</Button>
        {open && (
          <Dialog title="Entwurf" onClose={() => set(false)}>
            <input aria-label="Entwurf" />
          </Dialog>
        )}
      </>
    );
  }
  render(<Demo />);
  const opener = screen.getByRole('button', { name: 'Öffnen' });
  opener.focus();
  fireEvent.click(opener);
  screen.getByLabelText('Entwurf', { selector: 'input' }).focus();
  fireEvent(screen.getByRole('dialog'), new Event('cancel', { cancelable: true }));
  expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
  expect(opener).toHaveFocus();
});
it('offers native rating radios for 10 stars and a distinct unset state', () => {
  let value: unknown;
  render(
    <RatingInput
      label="Geschmack"
      stars={10}
      value={3}
      onChange={(v) => {
        value = v;
      }}
    />,
  );
  expect(screen.getAllByRole('radio')).toHaveLength(10);
  fireEvent.click(screen.getByRole('radio', { name: 'Geschmack: 10 von 10' }));
  expect(value).toBe(10);
  fireEvent.click(screen.getByRole('button', { name: 'Geschmack entfernen' }));
  expect(value).toBeNull();
});
it('cancels only the nested crop and retains the parent draft, modal and crop opener', () => {
  const outerClose = vi.fn();
  function Nested() {
    const [outer, setOuter] = useState(true);
    const [crop, setCrop] = useState(false);
    return outer ? (
      <Dialog
        title="Liquid bearbeiten"
        onClose={() => {
          outerClose();
          setOuter(false);
        }}
      >
        <input aria-label="Liquidname" defaultValue="Original" />
        <Button onClick={() => setCrop(true)}>Ausschnitt ändern</Button>
        {crop && (
          <Dialog title="Bildausschnitt" onClose={() => setCrop(false)}>
            <input aria-label="Zoom" type="range" defaultValue="1" />
          </Dialog>
        )}
      </Dialog>
    ) : (
      <p>Geschlossen</p>
    );
  }
  render(<Nested />);
  fireEvent.change(screen.getByLabelText('Liquidname'), {
    target: { value: 'Erhaltener Entwurf' },
  });
  const opener = screen.getByRole('button', { name: 'Ausschnitt ändern' });
  opener.focus();
  fireEvent.click(opener);
  screen.getByLabelText('Zoom').focus();
  fireEvent(
    screen.getByRole('dialog', { name: 'Bildausschnitt' }),
    new Event('cancel', { bubbles: false, cancelable: true }),
  );
  expect(outerClose).not.toHaveBeenCalled();
  expect(screen.queryByRole('dialog', { name: 'Bildausschnitt' })).not.toBeInTheDocument();
  expect(screen.getByRole('dialog', { name: 'Liquid bearbeiten' })).toBeInTheDocument();
  expect(screen.getByLabelText('Liquidname')).toHaveValue('Erhaltener Entwurf');
  expect(opener).toHaveFocus();
});

it('does not put a focus ring on a scroll region when a dialog closes', () => {
  HTMLDialogElement.prototype.showModal ??= function () {};
  HTMLDialogElement.prototype.close ??= function () {};
  const scroller = document.createElement('div');
  scroller.tabIndex = 0;
  document.body.append(scroller);
  scroller.focus();
  const view = render(
    <Dialog title="Detail" onClose={() => {}}>
      <p>Inhalt</p>
    </Dialog>,
  );
  view.unmount();
  expect(document.activeElement).not.toBe(scroller);
  scroller.remove();
});
