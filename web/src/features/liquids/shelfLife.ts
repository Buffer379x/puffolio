/** Green when full, through yellow, to red when empty. The server computes the value. */
export const shelfColor = (percent: number) => `hsl(${Math.round(percent * 1.2)} 65% 42%)`;
