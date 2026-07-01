export const site = {
  name: 'Pinax',
  tagline: 'Any docs site. A local MCP server. Under a minute.',
  url: 'https://pinax.dev',
  repo: 'https://github.com/desmondsanctity/pinax',
  releases: 'https://github.com/desmondsanctity/pinax/releases',
  brewTap: 'desmondsanctity/tap/pinax',
  goModule: 'github.com/desmondsanctity/pinax/cmd/pinax',
  description:
    'Pinax crawls a public documentation site, indexes its structure, and exposes it to any MCP client as four focused tools. Pages are fetched live, so what your agent reads is what the docs site serves today.',
} as const;

export const nav = {
  top: [
    { label: 'Docs', href: '/docs/quick-start' },
    { label: 'Catalog', href: '/catalog' },
    { label: 'GitHub', href: site.repo, external: true },
  ],
} as const;
