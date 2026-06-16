import Link from '@docusaurus/Link'
import useDocusaurusContext from '@docusaurus/useDocusaurusContext'
import CodeBlock from '@theme/CodeBlock'
import Layout from '@theme/Layout'
import type { ReactNode } from 'react'

interface Feature {
	title: string
	body: string
	icon: ReactNode
}

const features: Feature[] = [
	{
		title: 'Power resilience',
		body: 'Coordinated graceful shutdown across every machine in your homelab when the UPS battery runs low.',
		icon: (
			<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden="true">
				<path
					d="M5 12h14M5 12l4-4M5 12l4 4M19 12l-4-4M19 12l-4 4"
					strokeLinecap="round"
					strokeLinejoin="round"
				/>
			</svg>
		),
	},
	{
		title: 'Real-time monitoring',
		body: 'Live UPS status across the fleet — battery, load, voltage, runtime — via the native NUT protocol.',
		icon: (
			<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden="true">
				<path d="M3 12h4l3-9 4 18 3-9h4" strokeLinecap="round" strokeLinejoin="round" />
			</svg>
		),
	},
	{
		title: 'Smart notifications',
		body: 'Slack, Discord, Pushover, Telegram, ntfy — alerts when power events happen, so you know before things go dark.',
		icon: (
			<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden="true">
				<path
					d="M18 8a6 6 0 1 0-12 0c0 7-3 9-3 9h18s-3-2-3-9M13.7 21a2 2 0 0 1-3.4 0"
					strokeLinecap="round"
					strokeLinejoin="round"
				/>
			</svg>
		),
	},
	{
		title: 'Automated actions',
		body: 'Per-host shutdown recipes for the tricky devices — UniFi, Synology, LG TVs — so you stop scripting them by hand.',
		icon: (
			<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden="true">
				<circle cx="12" cy="12" r="3" />
				<path
					d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 1 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 1 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 1 1-2.83-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 1 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 1 1 2.83-2.83l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 1 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 1 1 2.83 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 1 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z"
					strokeLinecap="round"
					strokeLinejoin="round"
				/>
			</svg>
		),
	},
]

const cliPreview = `# 1. Describe your hosts interactively
homelab-nut init

# 2. SSH out and configure NUT across the fleet
homelab-nut apply

# 3. Watch the live UPS dashboard
homelab-nut status

# Or just open the full TUI:
homelab-nut`

export default function Home(): ReactNode {
	const { siteConfig } = useDocusaurusContext()
	return (
		<Layout title={siteConfig.title} description={siteConfig.tagline}>
			<header className="hn-hero">
				<img
					src="https://raw.githubusercontent.com/rtorcato/homelab-nut/main/cover.png"
					alt="homelab-nut"
					className="hn-hero__banner"
				/>
				<h1 className="hn-hero__title">homelab-nut</h1>
				<p className="hn-hero__tagline">{siteConfig.tagline}</p>
				<div className="hn-hero__ctas">
					<Link className="hn-btn hn-btn--primary" to="/docs/intro">
						Get Started →
					</Link>
					<Link className="hn-btn hn-btn--ghost" href="https://github.com/rtorcato/homelab-nut">
						GitHub
					</Link>
					<Link className="hn-btn hn-btn--ghost" to="/docs/roadmap">
						Roadmap
					</Link>
				</div>
			</header>

			<section className="hn-features">
				<div className="hn-features__grid">
					{features.map((f) => (
						<div className="hn-feature" key={f.title}>
							<div className="hn-feature__icon">{f.icon}</div>
							<h3 className="hn-feature__title">{f.title}</h3>
							<p className="hn-feature__body">{f.body}</p>
						</div>
					))}
				</div>
			</section>

			<section className="hn-preview">
				<h2 className="hn-preview__title">What the CLI looks like</h2>
				<p className="hn-preview__hint">
					In active development — see the <Link to="/docs/roadmap">roadmap</Link> for phase status.
					Today's bash scripts keep working; the new CLI wraps them in v1, then progressively ports
					to native Go.
				</p>
				<CodeBlock language="bash">{cliPreview}</CodeBlock>
			</section>
		</Layout>
	)
}
