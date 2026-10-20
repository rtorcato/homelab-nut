# homelab-nut docs: design spec

The design for the docs site in `apps/docs` (Docusaurus v3). @rtorcato signed off on it on 2026-10-16 (direction A: ASCII mascot, split hero). Build against this file. If something here doesn't fit Docusaurus, ask before you invent a new layout.

The site is dark-first, with one navy field, an emerald accent and the nut mascot. Light mode gets the same care, and every text colour passes WCAG 2.1 AA in both themes.

## What changes

| Area | Today | New |
|---|---|---|
| Hero | `cover.png` banner, dark even in light mode | Split hero: wordmark, tagline and CTAs on the left, the ASCII mascot on the right |
| Backgrounds | Navy hero, grey body, slate footer | One field for the whole page; the footer is one step lighter |
| Features | 3 + 1 grid; Power resilience uses a ↔ icon | 4 across; Power resilience uses a shield |
| Content | Getting started, CLI and Roadmap only | Adds a **Guides** section built from the repo's `docs/` guides |
| Navbar | Green bolt (supabase-common's glyph); Docs lights up on every page | Nut mark; each item lights up only for its own section |
| CLI reference | Title twice, `homelab-nut` prefix on every label, flags cut off | One code-font title, short labels, flags as a table |
| Contrast | Muted text 2.5:1 in light mode; code comments 2.7:1 and 3.0:1 | All ≥ 4.5:1 |
| Mobile | Page is 511px wide on a 390px phone | No sideways scroll at 320px and up |

## Tokens

Keep the existing `--hn-*` names in `src/css/custom.css`. Fix the values and add a light-mode override for every token.

| Token | Light | Dark | Use |
|---|---|---|---|
| `--hn-bg` | `#ffffff` | `#0a1422` | Page, hero and navbar field |
| `--hn-surface` | `#f6f9f8` | `#0f1c2e` | Cards, footer, code blocks (dark) |
| `--hn-surface-2` | `#eef3f1` | `#142841` | Search field, hover |
| `--hn-border` | `rgba(10,20,34,.10)` | `rgba(148,163,184,.16)` | All 1px borders |
| `--hn-text-bright` | `#0a1422` | `#e9f5ee` | Headings, card titles |
| `--hn-text` | `#374151` | `#b4bfcc` | Body copy |
| `--hn-text-muted` | `#5b6675` | `#9aa4b2` | Hints, card body, table headers |
| `--hn-accent` | `#047857` | `#34d399` | Links, active nav, accent text |
| `--hn-accent-ui` | `#047857` | `#10b981` | Primary button fill, status chips |
| `--hn-on-accent` | `#ffffff` | `#06231a` | Text on `--hn-accent-ui` |
| `--hn-accent-soft` | `rgba(5,150,105,.10)` | `rgba(16,185,129,.12)` | Mark tile, step numbers, active sidebar item |
| `--hn-mascot` | `#b45309` | `#fb923c` | Amber: the release pill, flag types, caution admonitions |
| `--hn-code-bg` | `#f3f6f9` | `#0f1c2e` | Inline code, install line, code blocks |
| `--hn-comment` | `#5b6675` | `#8b9bc4` | Prism comment tokens |
| `--hn-glow` | `none` | `0 0 60px -12px rgba(16,185,129,.45)` | Primary button only |

**Infima mapping (light).** `--ifm-color-primary: #047857`, then dark `#046c4e`, darker `#03664a`, darkest `#03543d`, light `#1d8668`, lighter `#2a8c70`, lightest `#4fa089`. `--ifm-link-color: var(--hn-accent)`. The old light primary `#059669` must go: white on it is 3.8:1.

**Infima mapping (dark).** Keep the current primary scale (`#10b981`). Set `--ifm-link-color: #34d399`, `--ifm-background-color: var(--hn-bg)`, `--ifm-footer-background-color: var(--hn-surface)` and `--ifm-code-background: var(--hn-code-bg)`.

**Status colours** come from the TUI and always sit next to a text label: OL `#10b981`, OB `#fbbf24`, LB `#f87171`.

### Checked contrast

| Pair | Ratio |
|---|---|
| Light: muted `#5b6675` on `#ffffff` / `#f6f9f8` | 5.8 / 5.5 |
| Light: accent `#047857` on `#ffffff`; white on `#047857` | 5.5 / 5.5 |
| Light: amber `#b45309` on `#f6f9f8` | 4.7 |
| Light: comment `#5b6675` on `#f3f6f9` | 5.4 |
| Dark: muted `#9aa4b2` on `#0f1c2e` | 6.8 |
| Dark: accent `#34d399` on `#0f1c2e` | 8.9 |
| Dark: `#06231a` on `#10b981` (primary button) | 6.6 |
| Dark: comment `#8b9bc4` on `#0f1c2e` | 6.2 |

If you add a colour, compute its ratio for both themes before you use it for text.

## Type

- **Sans:** Inter 400/500/600/700/800. **Mono:** JetBrains Mono 400/500/600. Both are already set as `--ifm-font-family-*`.
- **Wordmark (hero H1):** Inter 800, `clamp(44px, 6vw, 72px)`, letter-spacing −0.04em, line-height 1. It reads `homelab-nut`, with `nut` in `--hn-accent`.
- **Hero lead:** Inter 600, `clamp(20px, 2.2vw, 26px)`, `--hn-text-bright`. **Hero sub:** 17px, `--hn-text-muted`, max 520px wide.
- **Section H2:** 28px, letter-spacing −0.02em. **Card H3:** 16px. **Card body:** 14.5px, line-height 1.55.
- **Doc H1:** 40px Inter 800 (30px on mobile). On CLI pages the H1 is the command in mono 600.
- Body is 16px / 1.6.

## Components

| Component | Where | Spec |
|---|---|---|
| Navbar | `docusaurus.config.ts` `navbar` | Items: **Docs** (`/docs/intro`), **Guides** (`/docs/guides`), **CLI** (`/docs/cli`), **Roadmap**, then GitHub ↗, the theme toggle and search on the right. Give Docs `activeBaseRegex: '^/docs/(intro\|contributing)'` so it lights up only on those pages. Guides and CLI match their own paths. |
| Navbar mark | `static/img/logo.svg` (+ `srcDark`) | The nut mark below, 20px glyph centred in a 32px rounded tile (radius 9px, `--hn-accent-soft` fill, 1px border at 35% accent). It's the same tile in both themes; only the colours swap. The mark is vertically centred on the title text. |
| Hero | `src/pages/index.tsx` | Two-column grid (1.1fr / 0.9fr, gap 40px), max 1160px wide, padding 72px 32px 56px. Left: release pill, wordmark H1, lead, sub, CTAs, install line. Right: the mascot with a status chip. A soft radial glow of `--hn-accent-soft` sits behind it, top right. No banner image. |
| Release pill | Hero | Mono 13px, a 999px radius, `--hn-surface` fill and border. Text: **beta** (amber, 600) · a link to the [newest release](https://github.com/rtorcato/homelab-nut/releases) (link uses `--ifm-link-color`, 500, underline on hover only). No version in the text, so it doesn't go stale on each tag. |
| CTAs | Hero | **Get started →** (primary: `--hn-accent-ui` fill, `--hn-on-accent` text, `--hn-glow`) → `/docs/intro`. **Read the guides** (secondary: surface fill and border) → `/docs/guides`. Min height 46px, radius 10px, 16px/600. Below 996px, both stretch to equal widths. |
| Install line | Hero | `$ homelab-nut init` in mono 14px on `--hn-code-bg`, radius 10px, max 520px wide, with a copy button. The `$` is accent-coloured and isn't copied. Docusaurus' `CodeBlock` copy button is fine. |
| Mascot | Hero | `static/img/mascot-ascii-dark.png` (green on transparent) and `mascot-ascii-light.png` (dark green on transparent), made from the repo's `image 2.png`. Show them with `ThemedImage` **wrapped in a div**, because `ThemedImage` forces `display: initial` over any class on the image. Width `min(400px, 100%)`. Dark mode only: `drop-shadow(0 0 24px rgba(16,185,129,.25))`. `alt=""` (decorative). |
| Status chip | Hero, over the mascot | Positioned bottom left: `OL` badge then **pi** · cyberpower, and `battery 100% · load 8% · 2h03m` below it. Mono 12px, surface card, `aria-hidden="true"` (it's sample data). Hidden below 996px. |
| Feature cards | Home | 4 columns, gap 16px; 1 column below 996px. Each card has 22px padding, radius 14px, surface fill and border, a 24px stroke icon (1.7 stroke) in `--hn-accent`, then the H3 and body. Icons: **shield with a check** (Power resilience), pulse (Real-time monitoring), bell (Smart notifications), gear (Automated actions). |
| "What the CLI looks like" | Home | Two columns (0.8fr / 1.2fr). Left: H2, a hint, then three numbered steps: `homelab-nut init`, `homelab-nut apply`, `homelab-nut`. Each has a 28px accent-soft number tile, the command in mono 600 and a one-line description. Right: the TUI screenshot (`image.png`, copied into `static/img/tui.png`) in a terminal frame: radius 14px, a `#1a1c22` title bar with three dots and the title `homelab-nut — dashboard`. Alt text: "The homelab-nut TUI dashboard showing host pi online at 100% battery, 8% load, 2h03m runtime". Below 996px the columns stack, and `min-width: 0` keeps it inside the viewport. |
| Guides grid | Home | H2 "Guides", hint "From one UPS on a Pi to a whole rack." Then a 3-column grid of link cards (min height 56px, title 15px/600 with a 13px muted subtitle, → on the right). Six cards: Server setup, Client setup, Smart shutdown, Notifications, Docker setup, Prometheus & Grafana. Slack is reached from Notifications. |
| Footer | `themeConfig.footer` | Keep the three columns (Docs, Community, NUT ecosystem). Add Guides to Docs. `--hn-surface` fill with a top border, so there's no seam against the page. |
| Doc sidebar | `sidebars.ts` | Getting started → **Guides** (category) → **CLI reference** (category) → Roadmap → Contributing. Active item: `--hn-accent-soft` fill, `--hn-accent` text, 600. |
| Guides section | `apps/docs/docs/guides/` | The seven guides from the repo's `docs/`: server setup, client setup, smart shutdown, notifications (Slack nested under it), Docker setup, Prometheus & Grafana. Add a generated index at `/docs/guides`. `docs/cli-reference.md` is already covered by the generated CLI pages, so don't publish it. |
| Code blocks | `prism` in config | Keep github / dracula, but override the comment token colour with `--hn-comment` for each theme, and set the block background to `--hn-code-bg`. |

### Navbar mark (SVG)

```svg
<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round">
  <path d="M10 2.5v2.5M14 2.5v2.5"/>
  <path d="M12 5c4 0 6.5 3.4 6.5 8S15.7 21.5 12 21.5 5.5 17.6 5.5 13 8 5 12 5z"/>
  <path d="M12 5.5c-1 2.2 1 3.8 0 6s1 3.8 0 6.2"/>
  <path d="M8.6 10.5c.8.6.8 1.6 0 2.3M15.4 12.2c-.8.6-.8 1.6 0 2.3"/>
</svg>
```

It's a walnut shell with a centre seam and two plug prongs. Stroke it in `--hn-accent`. The favicon uses the same glyph.

## CLI reference pages

These pages come from `cmd/gen-docs`. Change the generator, not the generated `.md` files.

- **One title.** Drop cobra's `## homelab-nut apply` heading. Emit `` # `homelab-nut apply` `` as the page's H1 so Docusaurus renders it in mono and doesn't add a second title. Keep `title:` in the front matter for the `<title>` tag.
- **Short labels.** Set `sidebar_label` to the command minus `homelab-nut ` (e.g. `apply`, `inventory validate` → `validate`). Root `homelab-nut.md` is labelled **Overview**. Labels use mono 14px.
- **Grouping.** `inventory` holds `edit`, `list`, `show` and `validate`; `shutdown` holds `test`. Every other command sits at the top level. New commands should land in the sidebar without anyone editing `sidebars.ts`.
- **Keep slugs** (`/cli/homelab-nut_apply` etc.), so existing links still work.
- **Description.** The short description sits under the H1 at 18px muted. The synopsis follows as body text.
- **Usage.** `## Usage`, then the usage line as a code block.
- **Flags as a table.** `## Flags` and `## Inherited flags`, each with the columns Flag | Type | Default | Description. Flags are in mono with `white-space: nowrap`, type in amber mono, default in muted mono (`—` when empty). Leave out `-h, --help`. Below 996px the table scrolls inside its own box (`display: block; overflow-x: auto`).
- **See also.** Use `## See also` (sentence case, not SEE ALSO), then a list of links styled as chips: mono 13.5px, 1px border, radius 8px, wrapped with an 8px gap. Style them with `h2#see-also + ul`, so the generator writes no MDX.
- The page TOC lists Usage, Flags, Inherited flags and See also.

## Layout rules

- **Breakpoint:** use Infima's 996px for every stack and collapse, so the hero, grids and navbar all switch at the same width.
- **No sideways scroll** from 320px wide up. Any wide child (terminal frame, tables, install line) gets `min-width: 0` or its own `overflow-x: auto`.
- **Mobile hero:** the mascot goes above the wordmark at 190px wide. The status chip is hidden, and the CTAs are full width.
- **Content width:** 1160px max for home sections, with 32px side padding (20px on mobile).

## Motion

Keep motion rare: it should only show that something changed state. There are two transitions, both CSS only (homelab-nut#161). Don't add a motion library.

| Where | What moves | Timing |
|---|---|---|
| Copy button (hero install line and every code block) | The success check scales 0.6 → 1 and fades in, coloured `--hn-accent`. The copy icon fades out. | 200ms in, 120ms out |
| Mobile menu (below 996px) | `.navbar-sidebar` slides with `transform`, and its backdrop fades | 250ms open, 200ms close |

- **Easing:** `cubic-bezier(0.2, 0, 0, 1)` for both.
- **Only `transform` and `opacity` animate.** Never width, height, top or left.
- **Use what Docusaurus already has:** the CodeBlock icon swap and Infima's sidebar slide. Restyle them with attribute selectors (`[class*="copyButtonSuccessIcon"]`) instead of swizzling.
- **Reduced motion:** under `prefers-reduced-motion: reduce` there are no transitions. The icon swaps instantly and the menu appears in place.
- **Not in this scope:** animated mascot, page transitions, scroll effects. A Lottie walnut for the 404 and empty states can come later, once the full-colour source exists.

## Accessibility

- AA contrast for all text in both themes (see the table above). Focus rings stay visible: don't remove Infima's outline.
- Touch targets are at least 44px: CTAs are 46px, guide cards 56px, and nav links use Infima's padding.
- The mascot and status chip are decorative (`alt=""`, `aria-hidden`). The TUI screenshot has real alt text.
- Status always appears as text (`OL`, `OB`, `LB`) as well as colour.

## Brand

- **Mascot:** the walnut from the README art. The ASCII version (`image 2.png`) is what ships now. A full-colour transparent version needs either the original source file or generated art, and isn't in this scope.
- **Accent:** emerald `#10b981` (dark) / `#047857` (light). Brand-kit assets (homelab-nut#140) should use `--accent 10b981` and the nut mark instead of the bolt.
- `cover.png` stays in the repo, but neither the site nor the README uses it. The README leads with the brand-kit banner (homelab-nut#182).
