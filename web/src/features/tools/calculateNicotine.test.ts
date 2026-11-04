import { describe, expect, it } from 'vitest';
import { calculateNicotine } from './calculateNicotine';
const base = {
  bottleMl: '120',
  aromaMl: '10',
  shotsMl: '20',
  concentration: '20',
};
describe('Nikotinstärke aus der tatsächlichen Endfüllmenge', () => {
  it('füllt den Rest mit Base auf und berechnet die Stärke in mg/ml', () => {
    expect(calculateNicotine(base)).toEqual({
      finalMl: '120.000',
      baseMl: '90.000',
      strengthMgMl: '3.333',
      totalNicotineMg: '400.000',
    });
  });
  it('akzeptiert eine volle Flasche ohne Base und Nullshots ohne Stärke', () => {
    expect(calculateNicotine({ ...base, bottleMl: '60', shotsMl: '50' })).toEqual({
      finalMl: '60.000',
      baseMl: '0.000',
      strengthMgMl: '16.667',
      totalNicotineMg: '1000.000',
    });
    expect(calculateNicotine({ ...base, shotsMl: '0', concentration: '' })).toMatchObject({
      baseMl: '110.000',
      strengthMgMl: '0.000',
    });
  });
  it('behält große Zwischenprodukte exakt und unterstützt Dezimalkomma', () => {
    expect(
      calculateNicotine({
        ...base,
        bottleMl: '1000000',
        aromaMl: '0',
        shotsMl: '1000000',
        concentration: '1000000',
      }),
    ).toEqual({
      finalMl: '1000000.000',
      baseMl: '0.000',
      strengthMgMl: '1000000.000',
      totalNicotineMg: '1000000000000.000',
    });
    expect(
      calculateNicotine({ ...base, bottleMl: '60', aromaMl: '10,5', shotsMl: '10,5' }).baseMl,
    ).toBe('39.000');
  });
  it.each(['', '-1', '1e3', '1.000,5', 'NaN', 'Infinity', '1000000.001', '1.1234'])(
    'weist ungültige Flaschenmenge %s zurück',
    (bottleMl) => {
      expect(() => calculateNicotine({ ...base, bottleMl })).toThrow();
    },
  );
  it('verweigert Überfüllung und eine Flasche mit Volumen 0', () => {
    expect(() => calculateNicotine({ ...base, bottleMl: '25' })).toThrow(/überschreiten/);
    expect(() => calculateNicotine({ ...base, bottleMl: '0', aromaMl: '0', shotsMl: '0' })).toThrow(
      /größer/,
    );
  });
});

describe('Referenzwerte und einzelne Eingaberegeln', () => {
  it('60/10/10 bei Stärke 20', () => {
    expect(
      calculateNicotine({
        ...base,
        bottleMl: '60',
        aromaMl: '10',
        shotsMl: '10',
        concentration: '20',
      }),
    ).toEqual({
      finalMl: '60.000',
      baseMl: '40.000',
      strengthMgMl: '3.333',
      totalNicotineMg: '200.000',
    });
  });
  it('erlaubt Stärke null bei einem Shotvolumen größer null', () => {
    expect(calculateNicotine({ ...base, concentration: '0' })).toEqual({
      finalMl: '120.000',
      baseMl: '90.000',
      strengthMgMl: '0.000',
      totalNicotineMg: '0.000',
    });
  });
  it.each(['aromaMl', 'shotsMl', 'concentration'] as const)(
    'fordert die leere Eingabe %s separat an',
    (key) => {
      expect(() => calculateNicotine({ ...base, [key]: '' })).toThrow();
    },
  );
  it.each(['aromaMl', 'shotsMl', 'concentration'] as const)(
    'validiert %s unabhängig von der Flaschenmenge',
    (key) => {
      for (const invalid of [
        '-1',
        'falsch',
        '1e3',
        '1.000,5',
        '1.1234',
        '1000000.001',
        'Infinity',
        'NaN',
      ]) {
        expect(() => calculateNicotine({ ...base, [key]: invalid }), `${key}=${invalid}`).toThrow();
      }
    },
  );
  it('rundet die mg/ml-Stärke und die Nikotinmasse bei einer exakten Hälfte auf', () => {
    expect(
      calculateNicotine({
        ...base,
        bottleMl: '1',
        aromaMl: '0',
        shotsMl: '0.5',
        concentration: '0.001',
      }),
    ).toEqual({
      finalMl: '1.000',
      baseMl: '0.500',
      strengthMgMl: '0.001',
      totalNicotineMg: '0.001',
    });
  });
});
