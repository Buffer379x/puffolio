import { readFileSync } from 'node:fs';
import { fireEvent, render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import type { Liquid } from '../app/types';
import { liquidColor } from './LiquidImage';
import { legacyPaletteIds, palette, paletteColor, PalettePicker } from './PalettePicker';
const luminance = (hex: string) => {
  const [r, g, b] = [1, 3, 5]
    .map((i) => parseInt(hex.slice(i, i + 2), 16) / 255)
    .map((c) => (c <= 0.03928 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4));
  return 0.2126 * r + 0.7152 * g + 0.0722 * b;
};
const contrast = (a: string, b: string) => {
  const [light, dark] = [luminance(a), luminance(b)].sort((x, y) => y - x);
  return (light + 0.05) / (dark + 0.05);
};
/** Hue in OKLCH, the angle a person would place the colour at on a colour wheel. */
const hue = (hex: string) => {
  const [r, g, b] = [1, 3, 5]
    .map((i) => parseInt(hex.slice(i, i + 2), 16) / 255)
    .map((c) => (c <= 0.04045 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4));
  const [l, m, s] = [
    0.4122214708 * r + 0.5363325363 * g + 0.0514459929 * b,
    0.2119034982 * r + 0.6806995451 * g + 0.1073969566 * b,
    0.0883024619 * r + 0.2817188376 * g + 0.6299787005 * b,
  ].map(Math.cbrt);
  const a = 1.9779984951 * l - 2.428592205 * m + 0.4505937099 * s;
  const bb = 0.0259040371 * l + 0.7827717662 * m - 0.808675766 * s;
  return ((Math.atan2(bb, a) * 180) / Math.PI + 360) % 360;
};
describe('palette', () => {
  it('has 32 colours with distinct ids, names and values', () => {
    expect(palette).toHaveLength(32);
    for (const column of [0, 1, 2])
      expect(new Set(palette.map((entry) => entry[column]))).toHaveProperty('size', 32);
  });
  it('runs through the spectrum from red to pink, with grey last', () => {
    // prettier-ignore
    expect(palette.map(([id]) => id)).toEqual([
      'red', 'coral', 'apricot', 'tangerine', 'orange', 'honey', 'sand', 'amber',
      'olive', 'pistachio', 'lime', 'moss', 'green', 'mint', 'jade', 'teal',
      'aqua', 'cyan', 'ice', 'sky', 'blue', 'cornflower', 'periwinkle', 'indigo',
      'lavender', 'violet', 'orchid', 'pink', 'magenta', 'rose', 'raspberry', 'slate',
    ]);
    // Going by the colours themselves, so a changed value cannot quietly break the order.
    const hues = palette.slice(0, -1).map(([, , color]) => hue(color));
    const start = hues[0];
    const unrolled = hues.map((h) => (h < start ? h + 360 : h));
    expect(unrolled).toEqual([...unrolled].sort((x, y) => x - y));
  });
  it('falls back to lime for an unknown colour', () => {
    expect(paletteColor('unknown')).toBe('#C5E879');
  });
  it('keeps tag text readable on every colour', () => {
    for (const [id, , color] of palette)
      expect(contrast(color, '#20261D'), id).toBeGreaterThanOrEqual(4.5);
  });
  it('leaves the twelve colours of 0.3.0 as they were', () => {
    expect(legacyPaletteIds.map(paletteColor)).toEqual([
      '#C5E879',
      '#ACD49D',
      '#91CDC5',
      '#A0D6E3',
      '#A8C4E5',
      '#B5B9E7',
      '#C6AFE4',
      '#D9B4D7',
      '#E9ADA8',
      '#E9BB94',
      '#E3D18B',
      '#BBC5B5',
    ]);
    expect(legacyPaletteIds).toEqual([
      'lime',
      'green',
      'teal',
      'cyan',
      'blue',
      'indigo',
      'violet',
      'pink',
      'red',
      'orange',
      'amber',
      'slate',
    ]);
  });
  it('matches the API contract', () => {
    const api = JSON.parse(readFileSync('../docs/api/openapi.yaml', 'utf8'));
    expect(palette.map(([id]) => id)).toEqual(api.components.schemas.Palette.enum);
  });
});
describe('bottle colour of a liquid without tag and colour field', () => {
  // Measured on the code of 0.3.0: the colour hangs on the name and must not change.
  it.each([
    ['Erdbeere', '#91CDC5'],
    ['Vanillecreme', '#BBC5B5'],
    ['Waldmeister', '#E9BB94'],
    ['Pfefferminze', '#E9BB94'],
    ['Tabak', '#A0D6E3'],
    ['Öl & Würze', '#A8C4E5'],
    ['🍓 Beere', '#E9BB94'],
    ['A', '#B5B9E7'],
    ['Ab', '#ACD49D'],
    ['Sehr langer Name mit vielen Zeichen', '#D9B4D7'],
    ['Blaubeere', '#A0D6E3'],
    ['Kirsche', '#ACD49D'],
    ['Mango', '#C6AFE4'],
    ['Kokos', '#A0D6E3'],
    ['Zitrone', '#B5B9E7'],
    ['Himbeere', '#E9BB94'],
  ])('%s stays %s', (name, color) => {
    expect(liquidColor({ name, values: {} } as unknown as Liquid, undefined)).toBe(color);
  });
});
describe('PalettePicker keyboard', () => {
  const press = (from: string, key: string, mobile = false) => {
    vi.stubGlobal('matchMedia', () => ({ matches: mobile }));
    const onChange = vi.fn();
    const view = render(<PalettePicker value={from} onChange={onChange} />);
    const name = palette.find(([id]) => id === from)![1];
    fireEvent.keyDown(screen.getByRole('button', { name }), { key });
    view.unmount();
    return onChange.mock.calls[0][0] as string;
  };
  const last = palette.length - 1;
  it('runs around at both ends', () => {
    expect(press('red', 'ArrowLeft')).toBe('slate');
    expect(press('slate', 'ArrowRight')).toBe('red');
  });
  it('moves a row of eight on the desktop', () => {
    expect(press('red', 'ArrowRight')).toBe('coral');
    expect(press('red', 'ArrowDown')).toBe('olive');
    expect(press('olive', 'ArrowUp')).toBe('red');
    expect(press('red', 'ArrowUp')).toBe('lavender');
  });
  it('moves a row of four on a phone', () => {
    expect(press('red', 'ArrowRight', true)).toBe('coral');
    expect(press('red', 'ArrowDown', true)).toBe('orange');
    expect(press('orange', 'ArrowUp', true)).toBe('red');
    expect(press('red', 'ArrowUp', true)).toBe('magenta');
  });
  it('moves the focus along', () => {
    vi.stubGlobal('matchMedia', () => ({ matches: false }));
    render(<PalettePicker value="red" onChange={vi.fn()} />);
    fireEvent.keyDown(screen.getByRole('button', { name: 'Rot' }), { key: 'ArrowLeft' });
    expect(document.activeElement).toBe(screen.getByRole('button', { name: palette[last][1] }));
  });
});
