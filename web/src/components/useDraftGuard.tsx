import { useState } from 'react';
import { useDirty } from '../app/state';
import { Confirm } from './ui';
/** Local close and app navigation/unload share the same semantic dirty state. */
export function useDraftGuard(dirty: boolean, onClose: () => void) {
  const [confirm, setConfirm] = useState(false);
  useDirty(dirty);
  return {
    close: () => (dirty ? setConfirm(true) : onClose()),
    confirmation: confirm ? (
      <Confirm
        title="Änderungen verwerfen?"
        onCancel={() => setConfirm(false)}
        onConfirm={() => {
          setConfirm(false);
          onClose();
        }}
      >
        <p>Ungespeicherte Änderungen gehen verloren.</p>
      </Confirm>
    ) : null,
  };
}
