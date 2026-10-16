import Link from '@docusaurus/Link'
import useBaseUrl from '@docusaurus/useBaseUrl'
import useDocusaurusContext from '@docusaurus/useDocusaurusContext'
import CodeBlock from '@theme/CodeBlock'
import Layout from '@theme/Layout'
import ThemedImage from '@theme/ThemedImage'
import type { ReactNode } from 'react'

// Update with each release (DESIGN.md › Alpha pill).
const RELEASE_NOTE = 'v0.1.0 · fleet apply lands in v0.2'

const iconProps = {
	width: 24,
	height: 24,
	viewBox: '0 0 24 24',
	fill: 'none',
	stroke: 'currentColor',
	strokeWidth: 1.7,
	strokeLinecap: 'round',
	strokeLinejoin: 'round',
} as const

interface Feature {
	title: string
	body: string
	icon: ReactNode
}

const features: Feature[] = [
	{
		title: 'Power resilience',
		body: 'Coordinated graceful shutdown across every machine when the UPS battery runs low.',
		icon: (
			<svg {...iconProps} aria-hidden="true">
				<path d="M12 3 4 6v6c0 4.5 3.4 8.4 8 9 4.6-.6 8-4.5 8-9V6z" />
				<path d="m9 12 2 2 4-4" />
			</svg>
		),
	},
	{
		title: 'Real-time monitoring',
		body: 'Battery, load, voltage and runtime across the fleet, over the native NUT protocol.',
		icon: (
			<svg {...iconProps} aria-hidden="true">
				<path d="M22 12h-4l-3 9L9 3l-3 9H2" />
			</svg>
		),
	},
	{
		title: 'Smart notifications',
		body: 'Slack, Discord, Pushover, Telegram and ntfy, so you know before things go dark.',
		icon: (
			<svg {...iconProps} aria-hidden="true">
				<path d="M6 8a6 6 0 0 1 12 0c0 7 3 9 3 9H3s3-2 3-9" />
				<path d="M10.3 21a1.94 1.94 0 0 0 3.4 0" />
			</svg>
		),
	},
	{
		title: 'Automated actions',
		body: 'Per-host shutdown recipes for the tricky devices: UniFi, Synology, LG TVs.',
		icon: (
			<svg {...iconProps} aria-hidden="true">
				<circle cx="12" cy="12" r="3" />
				<path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 1 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 1 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 1 1-2.83-2.83l.06-.06A1.65 1.65 0 0 0 4.68 15a1.65 1.65 0 0 0-1.51-1H3a2 2 0 1 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 1 1 2.83-2.83l.06.06A1.65 1.65 0 0 0 9 4.68 1.65 1.65 0 0 0 10 3.17V3a2 2 0 1 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 1 1 2.83 2.83l-.06.06A1.65 1.65 0 0 0 19.4 9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 1 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z" />
			</svg>
		),
	},
]

const steps = [
	{ cmd: 'homelab-nut init', body: 'Describe your hosts and UPS interactively.' },
	{ cmd: 'homelab-nut apply', body: 'SSH out and configure NUT across the fleet.' },
	{ cmd: 'homelab-nut', body: 'Open the dashboard: battery, load and runtime per host.' },
]

// Pages come from the "publish the guides" work (apps/docs/docs/guides/).
const guides = [
	{ title: 'Server setup', sub: 'The machine wired to the UPS', to: '/docs/guides/server-setup' },
	{
		title: 'Client setup',
		sub: 'Every machine that should shut down',
		to: '/docs/guides/client-setup',
	},
	{
		title: 'Smart shutdown',
		sub: 'Order, timers and tricky devices',
		to: '/docs/guides/smart-shutdown',
	},
	{
		title: 'Notifications',
		sub: 'Slack, Discord, ntfy and more',
		to: '/docs/guides/notifications',
	},
	{ title: 'Docker setup', sub: 'nut-webgui and exporters', to: '/docs/guides/docker-setup' },
	{
		title: 'Prometheus & Grafana',
		sub: 'Dashboards and alerts',
		to: '/docs/guides/prometheus-grafana',
	},
]

export default function Home(): ReactNode {
	const { siteConfig } = useDocusaurusContext()
	return (
		<Layout title={siteConfig.title} description={siteConfig.tagline}>
			<main className="hn-home">
				<section className="hn-hero">
					<div className="hn-hero__inner">
						<div className="hn-hero__copy">
							<span className="hn-pill">
								<b>alpha</b> {RELEASE_NOTE}
							</span>
							<h1 className="hn-wordmark">
								homelab-<span>nut</span>
							</h1>
							<p className="hn-lead">{siteConfig.tagline}</p>
							<p className="hn-sub">
								One CLI to wire every machine to your UPS: live status, alerts when the power drops,
								and a clean shutdown before the battery runs out.
							</p>
							<div className="hn-ctas">
								<Link className="hn-btn hn-btn--primary" to="/docs/intro">
									Get started →
								</Link>
								<Link className="hn-btn" to="/docs/guides">
									Read the guides
								</Link>
							</div>
							<div className="hn-install">
								<CodeBlock language="bash">homelab-nut init</CodeBlock>
							</div>
						</div>
						<div className="hn-mascot">
							{/* ThemedImage forces display: initial on the img, so the wrapper sizes it. */}
							<div className="hn-mascot__img">
								<ThemedImage
									alt=""
									sources={{
										dark: useBaseUrl('/img/mascot-ascii-dark.png'),
										light: useBaseUrl('/img/mascot-ascii-light.png'),
									}}
								/>
							</div>
							<div className="hn-status" aria-hidden="true">
								<span>
									<span className="hn-status__ol">OL</span>
									<b>pi</b> · cyberpower
								</span>
								<span>battery 100% · load 8% · 2h03m</span>
							</div>
						</div>
					</div>
				</section>

				<section className="hn-section">
					<div className="hn-features">
						{features.map((f) => (
							<div className="hn-card" key={f.title}>
								{f.icon}
								<h3>{f.title}</h3>
								<p>{f.body}</p>
							</div>
						))}
					</div>
				</section>

				<section className="hn-section">
					<div className="hn-split">
						<div>
							<h2>What the CLI looks like</h2>
							<p className="hn-hint">
								Three commands from a laptop, then a live TUI for day-to-day ops.
							</p>
							<ol className="hn-steps">
								{steps.map((s, i) => (
									<li key={s.cmd}>
										<span className="hn-steps__n">{i + 1}</span>
										<div>
											<code>{s.cmd}</code>
											<p>{s.body}</p>
										</div>
									</li>
								))}
							</ol>
						</div>
						<div className="hn-term">
							<div className="hn-term__bar">
								<i />
								<i />
								<i />
								<span>homelab-nut — dashboard</span>
							</div>
							<img
								src={useBaseUrl('/img/tui.png')}
								alt="The homelab-nut TUI dashboard showing host pi online at 100% battery, 8% load, 2h03m runtime"
								width={1748}
								height={1008}
								loading="lazy"
							/>
						</div>
					</div>
				</section>

				<section className="hn-section hn-section--guides">
					<h2>Guides</h2>
					<p className="hn-hint">From one UPS on a Pi to a whole rack.</p>
					<div className="hn-guides">
						{guides.map((g) => (
							<Link className="hn-guide" to={g.to} key={g.to}>
								<div>
									{g.title}
									<small>{g.sub}</small>
								</div>
								<span className="hn-guide__arrow" aria-hidden="true">
									→
								</span>
							</Link>
						))}
					</div>
				</section>
			</main>
		</Layout>
	)
}
