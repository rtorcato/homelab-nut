import { defineConfig, devices } from '@playwright/test'

export default defineConfig({
	testDir: './tests',
	fullyParallel: true,
	forbidOnly: !!process.env.CI,
	retries: process.env.CI ? 2 : 0,
	workers: process.env.CI ? 1 : undefined,
	reporter: process.env.CI ? [['github'], ['html', { open: 'never' }]] : 'html',
	use: {
		baseURL: 'http://127.0.0.1:3000',
		trace: 'on-first-retry',
	},
	projects: [
		{ name: 'chromium', use: { ...devices['Desktop Chrome'] } },
	],
	webServer: {
		// Build the static site, then serve it with the docusaurus production
		// server so the test exercises the same build that ships.
		command: 'pnpm build && pnpm serve --no-open --port 3000',
		port: 3000,
		reuseExistingServer: !process.env.CI,
		stdout: 'ignore',
		stderr: 'pipe',
		timeout: 180_000,
	},
})
