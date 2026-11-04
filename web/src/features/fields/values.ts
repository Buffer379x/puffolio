import type { Values } from '../../app/types';
export function canonicalNumber(raw: string, scale = 0): string {
  const s = raw.trim().replace(',', '.');
  if (!/^-?\d+(?:\.\d+)?$/.test(s)) throw new Error('Zahl eingeben.');
  const [whole, fraction = ''] = s.split('.');
  if (fraction.length > scale) throw new Error(`Höchstens ${scale} Nachkommastellen.`);
  const negative = whole.startsWith('-');
  const digits = whole.replace('-', '').replace(/^0+(?=\d)/, '');
  const f = fraction.padEnd(scale, '0');
  const sign = negative && (digits !== '0' || /[1-9]/.test(f)) ? '-' : '';
  return sign + digits + (scale ? '.' + f : '');
}
export function formatDecimal(s: string): string {
  const [whole, part] = s.split('.');
  return whole.replace(/\B(?=(\d{3})+(?!\d))/g, '.') + (part === undefined ? '' : ',' + part);
}
/** For reading, not editing: "60.0" shows as "60", while the stored scale stays exact. */
export function formatNumber(s: string): string {
  return formatDecimal(s.includes('.') ? s.replace(/\.?0+$/, '') : s);
}
export function formatDate(s: string): string {
  return /^\d{4}-\d{2}-\d{2}$/.test(s) ? s.slice(8) + '.' + s.slice(5, 7) + '.' + s.slice(0, 4) : s;
}
export function changedValues(before: Values, after: Values): Values {
  return Object.fromEntries(
    Object.entries(after).filter(([id, v]) => JSON.stringify(v) !== JSON.stringify(before[id])),
  );
}
export function saveBlob(blob: Blob, name: string) {
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = name;
  a.click();
  setTimeout(() => URL.revokeObjectURL(url), 1000);
}
