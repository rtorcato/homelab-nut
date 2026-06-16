import type * as Preset from '@docusaurus/preset-classic'
import type { Config } from '@docusaurus/types'
import { themes as prismThemes } from 'prism-react-renderer'

const config: Config = {
	title: 'homelab-nut',
	tagline: 'Network UPS Tools, set up from your laptop.',
	favicon: 'img/favicon.svg',

	url: 'https://rtorcato.github.io',
	baseUrl: '/homelab-nut/',

	organizationName: 'rtorcato',
	projectName: 'homelab-nut',
	trailingSlash: false,

	onBrokenLinks: 'warn',

	markdown: {
		format: 'detect',
		hooks: {
			onBrokenMarkdownLinks: 'warn',
		},
	},

	i18n: {
		defaultLocale: 'en',
		locales: ['en'],
	},

	headTags: [
		{
			tagName: 'link',
			attributes: { rel: 'preconnect', href: 'https://fonts.googleapis.com' },
		},
		{
			tagName: 'link',
			attributes: {
				rel: 'preconnect',
				href: 'https://fonts.gstatic.com',
				crossorigin: 'anonymous',
			},
		},
	],

	stylesheets: [
		'https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700;800&family=JetBrains+Mono:wght@400;500;600&display=swap',
	],

	presets: [
		[
			'classic',
			{
				docs: {
					sidebarPath: './sidebars.ts',
					routeBasePath: '/docs',
					editUrl: 'https://github.com/rtorcato/homelab-nut/edit/main/apps/docs/',
				},
				blog: false,
				theme: {
					customCss: './src/css/custom.css',
				},
			} satisfies Preset.Options,
		],
	],

	plugins: [
		[
			require.resolve('@easyops-cn/docusaurus-search-local'),
			{
				hashed: true,
				language: ['en'],
				indexBlog: false,
				indexDocs: true,
				indexPages: true,
				docsRouteBasePath: '/docs',
			},
		],
	],

	themeConfig: {
		colorMode: {
			defaultMode: 'dark',
			disableSwitch: false,
			respectPrefersColorScheme: false,
		},
		image: 'img/og.png',
		navbar: {
			title: 'homelab-nut',
			logo: {
				alt: 'homelab-nut',
				src: 'img/logo.svg',
			},
			items: [
				{
					type: 'doc',
					docId: 'intro',
					position: 'left',
					label: 'Docs',
				},
				{
					to: '/docs/cli',
					position: 'left',
					label: 'CLI',
				},
				{
					to: '/docs/roadmap',
					position: 'left',
					label: 'Roadmap',
				},
				{
					href: 'https://github.com/rtorcato/homelab-nut',
					label: 'GitHub',
					position: 'right',
				},
			],
		},
		footer: {
			style: 'dark',
			links: [
				{
					title: 'Docs',
					items: [
						{ label: 'Getting Started', to: '/docs/intro' },
						{ label: 'CLI Reference', to: '/docs/cli' },
						{ label: 'Roadmap', to: '/docs/roadmap' },
					],
				},
				{
					title: 'Community',
					items: [
						{
							label: 'GitHub Issues',
							href: 'https://github.com/rtorcato/homelab-nut/issues',
						},
						{
							label: 'Contributing',
							to: '/docs/contributing',
						},
					],
				},
				{
					title: 'NUT Ecosystem',
					items: [
						{ label: 'Network UPS Tools', href: 'https://networkupstools.org/' },
						{
							label: 'nut-webgui',
							href: 'https://github.com/SuperioOne/nut_webgui',
						},
						{
							label: 'nut_exporter',
							href: 'https://github.com/DRuggeri/nut_exporter',
						},
					],
				},
			],
			copyright: `Copyright © ${new Date().getFullYear()} Richard Torcato · MIT licensed`,
		},
		prism: {
			theme: prismThemes.github,
			darkTheme: prismThemes.dracula,
			additionalLanguages: ['bash', 'yaml', 'toml', 'go', 'docker', 'ini'],
		},
	} satisfies Preset.ThemeConfig,
}

export default config
