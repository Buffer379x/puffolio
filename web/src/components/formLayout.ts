/** Wide fields keep their order; each intervening simple run is paired from its start. */
export function formSpans(wide: boolean[]): boolean[] {
  const full = [...wide];
  let start = 0;
  for (let i = 0; i <= wide.length; i++) {
    if (i === wide.length || wide[i]) {
      if ((i - start) % 2 === 1) full[i - 1] = true;
      start = i + 1;
    }
  }
  return full;
}
