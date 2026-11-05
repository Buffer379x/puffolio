/**
 * In dialog order, row by row: through the spectrum from red to pink, with grey last. Rows of
 * eight, four on a phone.
 */
export const palette = [
  ['red', 'Rot', '#E9ADA8'],
  ['coral', 'Koralle', '#F19A85'],
  ['apricot', 'Aprikose', '#FBDCC6'],
  ['tangerine', 'Mandarine', '#F0A765'],
  ['orange', 'Orange', '#E9BB94'],
  ['honey', 'Honig', '#EDBF57'],
  ['sand', 'Sand', '#E2D8C3'],
  ['amber', 'Gelb', '#E3D18B'],
  ['olive', 'Olive', '#B9C46A'],
  ['pistachio', 'Pistazie', '#DDEFA6'],
  ['lime', 'Limette', '#C5E879'],
  ['moss', 'Moos', '#9FBF73'],
  ['green', 'Grün', '#ACD49D'],
  ['mint', 'Minze', '#BDEBCB'],
  ['jade', 'Jade', '#7FCB9F'],
  ['teal', 'Türkis', '#91CDC5'],
  ['aqua', 'Aqua', '#A9E8E2'],
  ['cyan', 'Cyan', '#A0D6E3'],
  ['ice', 'Eisblau', '#C9E4F5'],
  ['sky', 'Himmelblau', '#86C5EC'],
  ['blue', 'Blau', '#A8C4E5'],
  ['cornflower', 'Kornblume', '#8FAFEA'],
  ['periwinkle', 'Immergrün', '#CBD3F6'],
  ['indigo', 'Indigo', '#B5B9E7'],
  ['lavender', 'Lavendel', '#DCCFF5'],
  ['violet', 'Violett', '#C6AFE4'],
  ['orchid', 'Orchidee', '#D39BE3'],
  ['pink', 'Rosa', '#D9B4D7'],
  ['magenta', 'Magenta', '#EB9FD0'],
  ['rose', 'Rosé', '#F5C6D6'],
  ['raspberry', 'Himbeere', '#EB94A8'],
  ['slate', 'Grau', '#BBC5B5'],
];
/**
 * The colours up to 0.3.0, in their order then. A liquid without tag and colour field gets
 * its bottle colour from its name through this list, so the list never grows.
 */
export const legacyPaletteIds = [
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
];
/** Limette, the first colour up to 0.4.0. Only for an id outside the palette. */
const fallbackColor = '#C5E879';
export function paletteColor(id: string) {
  return palette.find((p) => p[0] === id)?.[2] || fallbackColor;
}
export function PalettePicker({
  value,
  onChange,
}: {
  value: string;
  onChange: (s: string) => void;
}) {
  return (
    <div className="palette-picker" role="group" aria-label="Tagfarbe">
      {palette.map(([id, name, color], i) => (
        <button
          type="button"
          key={id}
          style={{ backgroundColor: color }}
          aria-label={name}
          aria-pressed={id === value}
          onClick={() => onChange(id)}
          onKeyDown={(e) => {
            if (['ArrowLeft', 'ArrowRight', 'ArrowUp', 'ArrowDown'].includes(e.key)) {
              e.preventDefault();
              const columns = matchMedia('(max-width: 600px)').matches ? 4 : 8;
              const step =
                e.key === 'ArrowLeft'
                  ? -1
                  : e.key === 'ArrowRight'
                    ? 1
                    : e.key === 'ArrowUp'
                      ? -columns
                      : columns;
              const next = (i + step + palette.length) % palette.length;
              onChange(palette[next][0]);
              (e.currentTarget.parentElement?.children[next] as HTMLElement)?.focus();
            }
          }}
        >
          {value === id ? '✓' : ''}
        </button>
      ))}
    </div>
  );
}
