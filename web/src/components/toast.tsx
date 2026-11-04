import { createContext, useCallback, useContext, useEffect, useRef, useState } from 'react';
import type { ReactNode } from 'react';
interface Toast {
  message: string;
  action?: { label: string; run: () => void };
}
const ToastContext = createContext<(toast: Toast | string) => void>(() => {});
/** Confirms an action that left no other visible trace, and offers its undo. */
export const useToast = () => useContext(ToastContext);
export function ToastProvider({ children }: { children: ReactNode }) {
  const [toast, setToast] = useState<Toast>();
  const timer = useRef<ReturnType<typeof setTimeout>>(undefined);
  const show = useCallback((next: Toast | string) => {
    const value = typeof next === 'string' ? { message: next } : next;
    setToast(value);
    clearTimeout(timer.current);
    timer.current = setTimeout(() => setToast(undefined), value.action ? 8000 : 3000);
  }, []);
  useEffect(() => () => clearTimeout(timer.current), []);
  return (
    <ToastContext.Provider value={show}>
      {children}
      <div className={'toast ' + (toast ? 'visible' : '')} role="status" aria-live="polite">
        {toast && (
          <>
            <span>{toast.message}</span>
            {toast.action && (
              <button
                type="button"
                onClick={() => {
                  setToast(undefined);
                  toast.action!.run();
                }}
              >
                {toast.action.label}
              </button>
            )}
          </>
        )}
      </div>
    </ToastContext.Provider>
  );
}
