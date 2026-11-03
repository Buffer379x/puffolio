import { useEffect, useState } from 'react';
const changed = 'puffolio-tags-changed';
/** Tag definitions are server-owned; this only invalidates mounted per-session readers. */
export function invalidateTags() {
  window.dispatchEvent(new Event(changed));
}
export function useTagRevision() {
  const [revision, setRevision] = useState(0);
  useEffect(() => {
    const update = () => setRevision((value) => value + 1);
    window.addEventListener(changed, update);
    return () => window.removeEventListener(changed, update);
  }, []);
  return revision;
}
