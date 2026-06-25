# Pinax website

Marketing + docs site for [Pinax](https://github.com/desmondsanctity/pinax),
deployed at https://pinax.dev. See `../docs/SITE_PLAN.md` for the build plan.

## Stack

- **Astro 5** — content-first, near-zero JS by default
- **Tailwind CSS v4** — CSS-first config via `@theme` in `src/styles/tokens.css`
- **Svelte 5** — used only for client islands (copy button, catalog filter)
- **MDX + content collections** — docs authored in `src/content/docs/`
- **Pagefind** — static post-build search index
- **pnpm** — package manager

## Develop

```sh
pnpm install
pnpm dev          # http://localhost:4321
pnpm check        # astro check + prettier --check
pnpm build        # astro build + pagefind
pnpm preview
```

## Layout

```
src/
  layouts/       BaseLayout, MarketingLayout, DocsLayout
  components/    nav/, marketing/, docs/, primitives/, icons/
  content/
    config.ts    zod schema for the docs collection
    docs/        MDX pages mirroring /docs IA
  data/          catalog.ts (imports ../../internal/catalog/catalog.json), clients.ts
  lib/           site.ts, seo.ts
  pages/         index.astro, catalog.astro, docs/, 404.astro
  styles/        globals.css, tokens.css
public/          favicon.svg, og.png (later), assets/
```

The catalog page imports `../../../internal/catalog/catalog.json` directly,
so it can never drift from the CLI's embedded copy.

## Deploy

Cloudflare Pages, build command `pnpm install --frozen-lockfile && pnpm build`,
output directory `dist/`. PR previews enabled.
