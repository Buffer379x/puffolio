import { FormGrid } from '../../components/FormGrid';
import { useState } from 'react';
import { api, setCSRF } from '../../app/api';
import type { Me } from '../../app/types';
import { Button, FormField, ErrorMessage, Panel } from '../../components/ui';
import { useAction } from '../../app/state';
export function AuthScreen({
  mode,
  onSuccess,
  embedded = false,
  onBack,
}: {
  mode: 'setup' | 'login';
  onSuccess: (m: Me) => void;
  embedded?: boolean;
  onBack?: () => void;
}) {
  const [form, setForm] = useState({
    username: '',
    displayName: '',
    password: '',
    code: '',
    starterFields: true,
  });
  const a = useAction();
  const update = (key: string, value: string | boolean) => setForm({ ...form, [key]: value });
  const title = mode === 'setup' ? 'Einrichten' : 'Anmelden';
  return (
    <div className={embedded ? '' : 'auth-page'}>
      {!embedded && (
        <div className="auth-brand">
          <img className="brand-light" src="/brand/logo.svg" alt="puffolio" />
          <img className="brand-dark" src="/brand/logo-dark.svg" alt="puffolio" />
        </div>
      )}
      <Panel title={title}>
        <form
          onSubmit={(e) => {
            e.preventDefault();
            void a.run(async () => {
              const result = await api<Me>(
                mode === 'setup' ? '/setup' : '/auth/login',
                'POST',
                mode === 'setup' ? form : { username: form.username, password: form.password },
              );
              setCSRF(result.csrfToken);
              onSuccess(result);
            });
          }}
        >
          <FormGrid>
            {mode === 'setup' && (
              <FormField
                label="Einrichtungscode"
                wide
                hint="Steht im Container-Log und in der Datei setup-code im Datenverzeichnis."
              >
                {(bindings) => (
                  <input
                    {...bindings}
                    autoFocus
                    aria-label="Einrichtungscode"
                    autoComplete="off"
                    required
                    value={form.code}
                    onChange={(e) => update('code', e.target.value)}
                  />
                )}
              </FormField>
            )}
            <FormField label="Benutzername" wide>
              {(bindings) => (
                <input
                  {...bindings}
                  aria-label="Benutzername"
                  autoComplete="username"
                  required
                  value={form.username}
                  onChange={(e) => update('username', e.target.value)}
                />
              )}
            </FormField>
            {mode === 'setup' && (
              <FormField label="Anzeigename" wide>
                {(bindings) => (
                  <input
                    {...bindings}
                    aria-label="Anzeigename"
                    autoComplete="nickname"
                    required
                    value={form.displayName}
                    onChange={(e) => update('displayName', e.target.value)}
                  />
                )}
              </FormField>
            )}
            <FormField label="Passwort" wide>
              {(bindings) => (
                <input
                  {...bindings}
                  aria-label="Passwort"
                  type="password"
                  required
                  minLength={mode === 'login' ? undefined : 12}
                  autoComplete={mode === 'login' ? 'current-password' : 'new-password'}
                  value={form.password}
                  onChange={(e) => update('password', e.target.value)}
                />
              )}
            </FormField>
            {mode === 'setup' && (
              <label className="check-row full">
                <input
                  type="checkbox"
                  checked={form.starterFields}
                  onChange={(e) => update('starterFields', e.target.checked)}
                />
                Starter-Felder übernehmen
              </label>
            )}
          </FormGrid>
          <ErrorMessage error={a.error} />
          <Button variant="primary" type="submit" disabled={a.busy}>
            {a.busy ? 'Bitte warten…' : title}
          </Button>
          {onBack && (
            <button type="button" className="text-action" onClick={onBack}>
              Ohne Anmeldung ansehen
            </button>
          )}
        </form>
      </Panel>
    </div>
  );
}
