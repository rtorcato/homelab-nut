import { expect, test } from '@playwright/test'

test.describe('homelab-nut docs site — smoke', () => {
	test('landing renders hero + features + CLI preview', async ({ page }) => {
		await page.goto('/homelab-nut/')

		// Hero
		await expect(page).toHaveTitle(/homelab-nut/)
		await expect(page.locator('.hn-hero__title')).toHaveText('homelab-nut')
		await expect(page.locator('.hn-hero__tagline')).toContainText('Network UPS Tools')

		// Primary CTA is reachable
		const cta = page.locator('a.hn-btn--primary', { hasText: 'Get Started' })
		await expect(cta).toBeVisible()

		// Feature grid has all 4 cards
		await expect(page.locator('.hn-feature')).toHaveCount(4)

		// CLI preview code block renders
		await expect(page.getByText('homelab-nut init').first()).toBeVisible()
	})

	test('navigation reaches Getting Started', async ({ page }) => {
		await page.goto('/homelab-nut/')
		await page.locator('a.hn-btn--primary', { hasText: 'Get Started' }).click()
		await expect(page).toHaveURL(/\/docs\/intro/)
		await expect(page.locator('h1')).toContainText('Getting Started')
	})

	test('dark/light theme toggle works', async ({ page }) => {
		await page.goto('/homelab-nut/')

		// Default is dark (configured in docusaurus.config.ts).
		await expect(page.locator('html')).toHaveAttribute('data-theme', 'dark')

		// Toggle to light mode using the navbar control.
		await page.getByRole('button', { name: /switch/i }).click()
		await expect(page.locator('html')).toHaveAttribute('data-theme', 'light')
	})
})
