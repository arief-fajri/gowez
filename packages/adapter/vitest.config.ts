import { defineConfig } from 'vitest/config';

// The adapter's tests are build-time only: they run in Node and never touch
// the Go runtime. `css-parity.test.ts` shells out to `go test` for the
// cross-language check, so the suite needs a generous timeout.
export default defineConfig({
  test: {
    include: ['test/**/*.test.ts'],
    environment: 'node',
    testTimeout: 120_000,
    hookTimeout: 120_000,
  },
});
