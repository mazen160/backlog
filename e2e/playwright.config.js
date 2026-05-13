const { defineConfig } = require('@playwright/test');

const chromiumExec = process.env.HOME +
  '/Library/Caches/ms-playwright/chromium-1217/chrome-mac-arm64' +
  '/Google Chrome for Testing.app/Contents/MacOS/Google Chrome for Testing';

module.exports = defineConfig({
  testDir: './tests',
  timeout: 20000,
  retries: 0,
  workers: 1,
  globalSetup:    './globalSetup.js',
  globalTeardown: './globalTeardown.js',
  use: {
    baseURL: 'http://localhost:8181',
    launchOptions: { executablePath: chromiumExec },
    headless: true,
  },
  reporter: [['list']],
});
