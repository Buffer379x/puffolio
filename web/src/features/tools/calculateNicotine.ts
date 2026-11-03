export interface NicotineInput {
  bottleMl: string;
  aromaMl: string;
  shotsMl: string;
  concentration: string;
  concentrationUnit: 'mg/l' | 'mg/ml';
}
export interface NicotineResult {
  finalMl: string;
  baseMl: string;
  strengthMgMl: string;
  strengthMgL: string;
  totalNicotineMg: string;
}

/** Input is scaled by 1000; intermediate products always remain BigInt. */
function decimal(raw: string, label: string): bigint {
  const value = String(raw ?? '').trim();
  if (value.length > 32 || !/^\d+(?:[.,]\d{1,3})?$/.test(value)) {
    throw new Error(`${label}: Zahl mit höchstens drei Nachkommastellen eingeben.`);
  }
  const [whole, fraction = ''] = value.replace(',', '.').split('.');
  const scaled = BigInt(whole) * 1000n + BigInt(fraction.padEnd(3, '0'));
  if (scaled > 1000000000n) throw new Error(`${label}: Eingabe ist zu groß.`);
  return scaled;
}
function fixed(value: bigint): string {
  return `${value / 1000n}.${String(value % 1000n).padStart(3, '0')}`;
}
function round(numerator: bigint, denominator: bigint): bigint {
  return (2n * numerator + denominator) / (2n * denominator);
}

/** Pure local calculation: bottleMl is the final capacity, including aroma and shots. */
export function calculateNicotine(input: NicotineInput): NicotineResult {
  const bottle = decimal(input.bottleMl, 'Flaschenmenge');
  const aroma = decimal(input.aromaMl, 'Aroma');
  const shots = decimal(input.shotsMl, 'Nikotinshots');
  if (bottle === 0n) throw new Error('Flaschenmenge muss größer als 0 ml sein.');
  if (aroma + shots > bottle)
    throw new Error('Aroma und Nikotinshots überschreiten die Flaschenmenge.');
  if (!['mg/l', 'mg/ml'].includes(input.concentrationUnit))
    throw new Error('Einheit der Shotstärke auswählen.');
  let concentration =
    shots === 0n && !String(input.concentration ?? '').trim()
      ? 0n
      : decimal(input.concentration, 'Stärke der Shots');
  if (input.concentrationUnit === 'mg/ml') concentration *= 1000n;
  const mass = shots * concentration;
  return {
    finalMl: fixed(bottle),
    baseMl: fixed(bottle - aroma - shots),
    strengthMgL: fixed(round(mass, bottle)),
    strengthMgMl: fixed(round(mass, bottle * 1000n)),
    totalNicotineMg: fixed(round(mass, 1000000n)),
  };
}
