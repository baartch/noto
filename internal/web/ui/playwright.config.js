import { defineConfig, devices } from "@playwright/test";

export default defineConfig({
	testDir: "./browser",
	outputDir: "./node_modules/.cache/playwright-test-results",
	fullyParallel: true,
	reporter: "list",
	use: {
		baseURL: "http://127.0.0.1:4173",
		trace: "retain-on-failure",
	},
	projects: [
		{
			name: "desktop-chrome",
			use: { ...devices["Desktop Chrome"], launchOptions: { channel: "chrome" } },
		},
		{
			name: "mobile-chrome",
			use: { ...devices["Pixel 7"], viewport: { width: 390, height: 844 }, launchOptions: { channel: "chrome" } },
		},
	],
	webServer: {
		command: "bun run browser/fixture-server.js",
		url: "http://127.0.0.1:4173/healthz",
		reuseExistingServer: !process.env.CI,
	},
});
