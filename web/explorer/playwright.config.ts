import { defineConfig } from "@playwright/test";

const executablePath = process.env.EVALWITNESS_BROWSER_EXECUTABLE;

export default defineConfig({
  expect: { timeout: 5_000 },
  fullyParallel: false,
  outputDir: "test-results",
  projects: [
    { name: "desktop", use: { viewport: { height: 1_000, width: 1_440 } } },
    { name: "narrow-laptop", use: { viewport: { height: 800, width: 1_100 } } },
    { name: "tablet", use: { viewport: { height: 1_024, width: 768 } } },
    { name: "mobile", use: { viewport: { height: 844, width: 390 } } },
  ],
  reporter: [["list"]],
  testDir: "./e2e",
  timeout: 30_000,
  use: {
    browserName: "chromium",
    ...(executablePath === undefined ? {} : { launchOptions: { executablePath } }),
    colorScheme: "light",
    contextOptions: { reducedMotion: "reduce" },
    locale: "en-US",
    serviceWorkers: "block",
  },
  workers: 1,
});
