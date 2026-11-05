/** A widget's entry count as typed by the owner: a whole number from 1 to 10, otherwise null. */
export function parseWidgetLimit(text: string): number | null {
  const value = text.trim();
  if (!/^\d{1,2}$/.test(value)) return null;
  const limit = Number(value);
  return limit >= 1 && limit <= 10 ? limit : null;
}
