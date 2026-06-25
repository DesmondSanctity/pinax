export type DocsGroup = {
  label: string;
  items: readonly { label: string; slug: string }[];
};

export const docsGroups: readonly DocsGroup[] = [
  {
    label: 'Guides',
    items: [
      { label: 'Quick start', slug: 'quick-start' },
      { label: 'Catalog', slug: 'catalog' },
      { label: 'Serve', slug: 'serve' },
      { label: 'Configuration', slug: 'configuration' },
      { label: 'Troubleshooting', slug: 'troubleshooting' },
    ],
  },
  {
    label: 'Reference',
    items: [
      { label: 'CLI', slug: 'cli' },
      { label: 'MCP tools', slug: 'mcp-tools' },
      { label: 'Architecture', slug: 'architecture' },
      { label: 'Contributing', slug: 'contributing' },
    ],
  },
  {
    label: 'Clients',
    items: [
      { label: 'Claude Desktop', slug: 'clients/claude-desktop' },
      { label: 'Claude Code', slug: 'clients/claude-code' },
      { label: 'Cursor', slug: 'clients/cursor' },
      { label: 'Windsurf', slug: 'clients/windsurf' },
      { label: 'Cline', slug: 'clients/cline' },
    ],
  },
];

export const docsOrder: readonly { label: string; slug: string; group: string }[] =
  docsGroups.flatMap((g) => g.items.map((i) => ({ ...i, group: g.label })));

export function groupForSlug(slug: string): string {
  return docsOrder.find((d) => d.slug === slug)?.group ?? 'Guides';
}
