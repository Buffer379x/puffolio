/** Calendar strings go directly between the native control and API, without Date/timezone conversion. */
export function DateInput({
  id,
  label,
  value,
  min,
  max,
  onChange,
  invalid = false,
  describedBy,
  labelledBy,
}: {
  id?: string;
  label: string;
  value: string;
  min?: string;
  max?: string;
  onChange: (value: string) => void;
  invalid?: boolean;
  describedBy?: string;
  labelledBy?: string;
}) {
  return (
    <input
      id={id}
      type="date"
      aria-label={label}
      aria-labelledby={labelledBy}
      aria-invalid={invalid}
      aria-describedby={describedBy}
      min={min}
      max={max}
      value={value}
      onChange={(e) => onChange(e.target.value)}
    />
  );
}
