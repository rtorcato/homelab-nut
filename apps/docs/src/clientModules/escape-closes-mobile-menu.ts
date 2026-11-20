// Docusaurus' mobile navbar has no Escape handler. While its sidebar is open, Escape clicks the sidebar's
// own close button and returns focus to the menu toggle. On desktop the sidebar is never open, so this is a no-op.
if (typeof document !== 'undefined') {
	document.addEventListener('keydown', (event) => {
		if (event.key !== 'Escape') return
		const close = document.querySelector<HTMLButtonElement>(
			'.navbar-sidebar--show .navbar-sidebar__close'
		)
		if (!close) return
		close.click()
		document.querySelector<HTMLButtonElement>('.navbar__toggle')?.focus()
	})
}
