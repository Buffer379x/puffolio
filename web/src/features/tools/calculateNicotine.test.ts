import { describe, expect, it } from 'vitest';
import { calculateNicotine } from './calculateNicotine';
const base = {
  bottleMl: '120',
  aromaMl: '10',
  shotsMl: '20',
  concentration: '20',
  concentrationUnit: 'mg/ml' as const,
};
describe('Nikotinstärke aus der tatsächlichen Endfüllmenge', () => {
  it('füllt den Rest mit Base auf und berechnet beide Einheiten unabhängig', () => {
    expect(calculateNicotine(base)).toEqual({
      finalMl: '120.000',
      baseMl: '90.000',
      strengthMgMl: '3.333',
      strengthMgL: '3333.333',
      totalNicotineMg: '400.000',
    });
  });
  it('unterscheidet mg/l ausdrücklich um den Faktor 1000', () => {
    expect(calculateNicotine({ ...base, concentrationUnit: 'mg/l' })).toMatchObject({
      baseMl: '90.000',
      strengthMgMl: '0.003',
      strengthMgL: '3.333',
      totalNicotineMg: '0.400',
    });
  });
  it('akzeptiert eine volle Flasche ohne Base und Nullshots ohne Stärke', () => {
    expect(calculateNicotine({ ...base, bottleMl: '60', shotsMl: '50' })).toEqual({
      finalMl: '60.000',
      baseMl: '0.000',
      strengthMgMl: '16.667',
      strengthMgL: '16666.667',
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
      strengthMgL: '1000000000.000',
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
  it.each([
    ['mg/ml', '20', '3.333', '3333.333', '200.000'],
    ['mg/l', '20000', '3.333', '3333.333', '200.000'],
    ['mg/l', '20', '0.003', '3.333', '0.200'],
  ] as const)(
    '60/10/10 bei %s und Stärke %s',
    (concentrationUnit, concentration, strengthMgMl, strengthMgL, totalNicotineMg) => {
      expect(
        calculateNicotine({
          ...base,
          bottleMl: '60',
          aromaMl: '10',
          shotsMl: '10',
          concentrationUnit,
          concentration,
        }),
      ).toEqual({
        finalMl: '60.000',
        baseMl: '40.000',
        strengthMgMl,
        strengthMgL,
        totalNicotineMg,
      });
    },
  );
  it('erlaubt Stärke null bei einem Shotvolumen größer null', () => {
    expect(calculateNicotine({ ...base, concentration: '0' })).toEqual({
      finalMl: '120.000',
      baseMl: '90.000',
      strengthMgMl: '0.000',
      strengthMgL: '0.000',
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
      strengthMgL: '0.500',
      totalNicotineMg: '0.001',
    });
  });
  it('rundet mg/l unabhängig bei einer exakten Hälfte auf', () => {
    expect(
      calculateNicotine({
        ...base,
        bottleMl: '2',
        aromaMl: '0',
        shotsMl: '1',
        concentration: '0.001',
        concentrationUnit: 'mg/l',
      }),
    ).toEqual({
      finalMl: '2.000',
      baseMl: '1.000',
      strengthMgMl: '0.000',
      strengthMgL: '0.001',
      totalNicotineMg: '0.000',
    });
  });
});
