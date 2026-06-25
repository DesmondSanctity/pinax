export type ClientId = 'claude-desktop' | 'claude-code' | 'cursor' | 'windsurf' | 'cline' | 'copilot';

export type Client = {
  id: ClientId;
  label: string;
  configPath: string;
  docsHref: string;
};

export const clients: readonly Client[] = [
  {
    id: 'claude-desktop',
    label: 'Claude Desktop',
    configPath: '~/Library/Application Support/Claude/claude_desktop_config.json',
    docsHref: '/docs/clients/claude-desktop',
  },
  {
    id: 'claude-code',
    label: 'Claude Code',
    configPath: 'claude mcp add … -- pinax serve …',
    docsHref: '/docs/clients/claude-code',
  },
  {
    id: 'cursor',
    label: 'Cursor',
    configPath: '~/.cursor/mcp.json',
    docsHref: '/docs/clients/cursor',
  },
  {
    id: 'windsurf',
    label: 'Windsurf',
    configPath: '~/.codeium/windsurf/mcp_config.json',
    docsHref: '/docs/clients/windsurf',
  },
  {
    id: 'cline',
    label: 'Cline (VS Code)',
    configPath: 'cline_mcp_settings.json',
    docsHref: '/docs/clients/cline',
  },
  {
    id: 'copilot',
    label: 'GitHub Copilot',
    configPath: '.vscode/mcp.json',
    docsHref: '/docs/clients/copilot',
  },
];
