import { expect, test } from '@playwright/test'

test.describe('homelab-nut docs site — smoke', () => {
	test('landing renders hero + features + CLI preview', async ({ page }) => {
		await page.goto('/homelab-nut/')

		// Hero
		await expect(page).toHaveTitle(/homelab-nut/)
		await expect(page.locator('.hn-wordmark')).toHaveText('homelab-nut')
		await expect(page.locator('.hn-lead')).toContainText('Network UPS Tools')

		// Primary CTA is reachable
		const cta = page.locator('a.hn-btn--primary', { hasText: 'Get started' })
		await expect(cta).toBeVisible()

		// Feature grid has all 4 cards
		await expect(page.locator('.hn-card')).toHaveCount(4)

		// CLI section: steps and the TUI screenshot
		await expect(page.getByText('homelab-nut init').first()).toBeVisible()
		await expect(page.locator('.hn-term img')).toBeVisible()
	})

	test('no sideways scroll on a 320px phone', async ({ page }) => {
		await page.setViewportSize({ width: 320, height: 720 })
		await page.goto('/homelab-nut/')
		const overflow = await page.evaluate(
			() => document.documentElement.scrollWidth - document.documentElement.clientWidth,
		)
		expect(overflow).toBe(0)
	})

	test('navigation reaches Getting Started', async ({ page }) => {
		await page.goto('/homelab-nut/')
		await page.locator('a.hn-btn--primary', { hasText: 'Get started' }).click()
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

	for (const theme of ['dark', 'light'] as const) {
		test(`Escape closes the open mobile menu and returns focus to its toggle (${theme}, reduced motion)`, async ({
			page,
		}) => {
			await page.emulateMedia({ reducedMotion: 'reduce' })
			await page.addInitScript((t) => localStorage.setItem('theme', t), theme)
			await page.setViewportSize({ width: 375, height: 720 })
			await page.goto('/homelab-nut/')
			await expect(page.locator('html')).toHaveAttribute('data-theme', theme)

			const toggle = page.locator('.navbar__toggle')
			await toggle.click()
			await expect(page.locator('.navbar-sidebar--show')).toBeVisible()

			await page.keyboard.press('Escape')
			await expect(page.locator('.navbar-sidebar--show')).toHaveCount(0)
			await expect(toggle).toBeFocused()
		})
	}
})
