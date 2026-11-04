#!/usr/bin/env node
// The preview is the real app with sample data, so what you review is what ships.
//   pnpm --dir web build && go build -o work/bin/puffolio ./cmd/puffolio
//   pnpm --dir web demo [port]
import { DEMO_LOGIN, startInstance } from './seed.mjs';

const { url, stop } = await startInstance(process.argv[2] || '8787');
const exit = () => {
  stop();
  process.exit(0);
};
process.on('SIGINT', exit);
process.on('SIGTERM', exit);
console.log(
  `Vorschau: ${url}\nAnmelden: ${DEMO_LOGIN.username} / ${DEMO_LOGIN.password}\nBeenden: Strg+C`,
);
