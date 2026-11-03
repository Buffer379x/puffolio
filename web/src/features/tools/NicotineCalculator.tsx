import { FormGrid } from '../../components/FormGrid';
import { useState } from 'react';
import { calculateNicotine, type NicotineInput, type NicotineResult } from './calculateNicotine';
import { formatDecimal } from '../fields/values';
import {
  PageHeader,
  Panel,
  Button,
  FormField,
  ErrorMessage,
  HelpPopover,
} from '../../components/ui';
const initial: NicotineInput = {
  bottleMl: '',
  aromaMl: '0',
  shotsMl: '',
  concentration: '',
  concentrationUnit: 'mg/l',
};
export function NicotineCalculator() {
  const [input, setInput] = useState(initial);
  const [result, setResult] = useState<NicotineResult>();
  const [error, setError] = useState<unknown>();
  const change = (key: keyof NicotineInput, value: string) => {
    setInput({ ...input, [key]: value });
    setResult(undefined);
    setError(undefined);
  };
  return (
    <>
      <PageHeader title="Nikotinrechner" />
      <div className="calculator-layout">
        <Panel
          title="Mischung"
          actions={
            <HelpPopover label="Annahmen und Einheiten">
              <p>
                Flaschenmenge ist die gesamte Endfüllmenge. Aroma und Base gelten als nikotinfrei.
                Die Shots haben eine gemeinsame Stärke.
              </p>
              <p>1 mg/ml = 1.000 mg/l. Volumen und Konzentration erlauben drei Nachkommastellen.</p>
            </HelpPopover>
          }
        >
          <form
            onSubmit={(e) => {
              e.preventDefault();
              setResult(undefined);
              try {
                setResult(calculateNicotine(input));
                setError(undefined);
              } catch (e) {
                setError(e);
              }
            }}
          >
            <FormGrid>
              {(
                [
                  ['bottleMl', 'Flaschenmenge'],
                  ['aromaMl', 'Aroma'],
                  ['shotsMl', 'Nikotinshots'],
                  ['concentration', 'Stärke der Shots'],
                ] as const
              ).map(([key, label]) => (
                <FormField key={key} label={label} wide={key === 'concentration'}>
                  {(bindings) => (
                    <div className="input-unit">
                      <input
                        {...bindings}
                        aria-label={label}
                        inputMode="decimal"
                        value={input[key]}
                        onChange={(e) => change(key, e.target.value)}
                      />
                      {key === 'concentration' ? (
                        <select
                          aria-label="Einheit der Shotstärke"
                          value={input.concentrationUnit}
                          onChange={(e) => change('concentrationUnit', e.target.value)}
                        >
                          <option>mg/l</option>
                          <option>mg/ml</option>
                        </select>
                      ) : (
                        <span>ml</span>
                      )}
                    </div>
                  )}
                </FormField>
              ))}
            </FormGrid>
            <ErrorMessage error={error} />
            <div className="actions">
              <Button variant="primary" type="submit">
                Berechnen
              </Button>
              <Button
                onClick={() => {
                  setInput({ ...initial, aromaMl: '' });
                  setResult(undefined);
                  setError(undefined);
                }}
              >
                Zurücksetzen
              </Button>
            </div>
          </form>
        </Panel>
        {result && (
          <Panel title="Ergebnis">
            <dl className="result-list">
              <dt>Benötigte Base</dt>
              <dd className="result-main">{formatDecimal(result.baseMl)} ml</dd>
              <dt>Nikotinstärke</dt>
              <dd>{formatDecimal(result.strengthMgMl)} mg/ml</dd>
              <dd>{formatDecimal(result.strengthMgL)} mg/l</dd>
              <dt>Fertige Menge</dt>
              <dd>{formatDecimal(result.finalMl)} ml</dd>
              <dt>Nikotin gesamt</dt>
              <dd>{formatDecimal(result.totalNicotineMg)} mg</dd>
            </dl>
          </Panel>
        )}
      </div>
    </>
  );
}
