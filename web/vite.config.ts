import { defineConfig } from 'vitest/config';
import react from '@vitejs/plugin-react';
export default defineConfig({
  plugins: [react()],
  server: { proxy: { '/api': 'http://127.0.0.1:8080', '/media': 'http://127.0.0.1:8080' } },
  build: { sourcemap: false, target: 'es2022' },
  test: { environment: 'jsdom', globals: true, setupFiles: ['./src/test-setup.ts'], clearMocks: true, restoreMocks: true }
});
