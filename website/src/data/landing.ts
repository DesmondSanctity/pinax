export type Stat = {
  value: string;
  label: string;
  sub: string;
};

export const stats: readonly Stat[] = [
  { value: '15', label: 'curated docs sites', sub: 'one-word names → indexed' },
  { value: '4', label: 'MCP tools', sub: 'list · search · sections · get' },
  { value: '0', label: 'cloud', sub: 'stays on your machine' },
];

export type Step = {
  index: string;
  verb: string;
  command: string;
  body: string;
};

export const howItWorks: readonly Step[] = [
  {
    index: '01',
    verb: 'Add',
    command: 'pinax add stripe',
    body: 'Pick a name from the catalog or paste any docs URL. Pinax discovers structure via llms.txt or sitemap.xml and writes a local manifest.',
  },
  {
    index: '02',
    verb: 'Serve',
    command: 'pinax serve',
    body: 'A local MCP server speaks stdio to your client and HTTP to the log UI. Pages are fetched live, so what your agent reads is what the docs site serves today.',
  },
  {
    index: '03',
    verb: 'Use',
    command: 'mcp tools/call search_pages',
    body: 'Your client gets four focused tools: list manifests, list sections, full-text search a page, and fetch the page. No vectors, no retraining.',
  },
];

export type InstallOption = {
  label: string;
  command: string;
  hint: string;
};

export const installOptions: readonly InstallOption[] = [
  {
    label: 'Homebrew',
    command: 'brew install desmondsanctity/tap/pinax',
    hint: 'Recommended on macOS · auto-updates with brew upgrade.',
  },
  {
    label: 'Go install',
    command: 'go install github.com/desmondsanctity/pinax/cmd/pinax@latest',
    hint: 'Requires Go 1.22+. Binary lands in $GOBIN.',
  },
  {
    label: 'Prebuilt binary',
    command: 'curl -L https://github.com/desmondsanctity/pinax/releases/latest -o pinax',
    hint: 'Linux/macOS/Windows builds on the releases page.',
  },
];

export type FinalCta = {
  kicker: string;
  headline: string;
  command: string;
  href: string;
  hrefLabel: string;
};

export const finalCta: FinalCta = {
  kicker: 'One last thing',
  headline: 'Index your first docs site now.',
  command: 'pinax add stripe && pinax serve',
  href: '/docs/quick-start',
  hrefLabel: 'Or read the quick start →',
};
