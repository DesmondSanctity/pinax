export type FaqItem = {
  q: string;
  a: string;
};

export const faq: readonly FaqItem[] = [
  {
    q: 'What is Pinax?',
    a: 'A small Go CLI that turns any public documentation site into a local Model Context Protocol server. Run it once, point your MCP client at it, and the docs become live tools your agent can call — no copy-pasting pages into chat.',
  },
  {
    q: 'Why MCP and not just RAG or a vector database?',
    a: 'Docs already have structure (headings, sitemaps, llms.txt). Pinax keeps that structure intact and serves pages live, so answers come from the version of the docs that exists today, not a stale embedding from last month. No embeddings, no re-indexing, no drift.',
  },
  {
    q: 'Does Pinax send anything off my machine?',
    a: "Only the requests you'd make anyway — fetching pages from the docs site itself. No telemetry, no analytics, no third-party APIs. The MCP transport is stdio between Pinax and your client; the log UI is a localhost-only HTTP server.",
  },
  {
    q: "Which docs sites work? Which don't?",
    a: 'Anything with a sitemap.xml or llms.txt works out of the box. Pages need to render on the server (HTML, not pure SPA shells) — most popular dev docs do. The catalog page lists the 15 we know index cleanly today; any URL with /, . or :// is a valid add target.',
  },
  {
    q: "How do I add a docs site that isn't in the catalog?",
    a: 'Run pinax add <url> with any HTTPS URL. Pinax discovers structure on the fly. If you want it shipped in the curated catalog, open an issue or PR against internal/catalog/catalog.json and add the three required fields: displayName, url, tags.',
  },
  {
    q: 'Can I run it against private or auth-gated docs?',
    a: 'Not in v0.3. Auth headers and cookie-based crawling are planned for v0.4. Today Pinax fetches as an anonymous client, so anything behind a login screen is invisible to it.',
  },
  {
    q: 'How is it different from gitingest, repomix, or just downloading llms.txt?',
    a: 'Those tools concatenate or summarise content into one large blob. Pinax exposes structured tools — list_sections, search_pages, get_page — so the agent fetches only the page it needs, when it needs it. No prompt-bloat, no out-of-date copies.',
  },
];
