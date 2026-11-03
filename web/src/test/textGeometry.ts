export interface TextRect {
  top: number;
  bottom: number;
}
/** Count visual rows by their shared vertical band, allowing inline font metrics. */
export function visualLineCount(rects: TextRect[]): number {
  const lines: TextRect[] = [];
  for (const rect of [...rects].sort((a, b) => a.top - b.top)) {
    const line = lines.find(
      (candidate) =>
        Math.min(candidate.bottom, rect.bottom) - Math.max(candidate.top, rect.top) >
        Math.min(candidate.bottom - candidate.top, rect.bottom - rect.top) / 2,
    );
    if (line) {
      line.top = Math.max(line.top, rect.top);
      line.bottom = Math.min(line.bottom, rect.bottom);
    } else lines.push({ ...rect });
  }
  return lines.length;
}
