export const palette = [
  ['lime', 'Limette', '#C5E879'],
  ['green', 'Grün', '#ACD49D'],
  ['teal', 'Türkis', '#91CDC5'],
  ['cyan', 'Cyan', '#A0D6E3'],
  ['blue', 'Blau', '#A8C4E5'],
  ['indigo', 'Indigo', '#B5B9E7'],
  ['violet', 'Violett', '#C6AFE4'],
  ['pink', 'Rosa', '#D9B4D7'],
  ['red', 'Rot', '#E9ADA8'],
  ['orange', 'Orange', '#E9BB94'],
  ['amber', 'Gelb', '#E3D18B'],
  ['slate', 'Grau', '#BBC5B5'],
];
export function paletteColor(id: string) {
  return palette.find((p) => p[0] === id)?.[2] || palette[0][2];
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
              const columns = matchMedia('(max-width: 600px)').matches ? 4 : 6;
              const step =
                e.key === 'ArrowLeft'
                  ? -1
                  : e.key === 'ArrowRight'
                    ? 1
                    : e.key === 'ArrowUp'
                      ? -columns
                      : columns;
              const next = (i + step + 12) % 12;
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
