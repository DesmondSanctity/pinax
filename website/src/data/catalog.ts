import catalogJson from '../../../internal/catalog/catalog.json';

export type CatalogEntry = {
  key: string;
  displayName: string;
  url: string;
  tags: readonly string[];
  llmsTxt?: string | undefined;
  platform?: string | undefined;
  excludes?: readonly string[] | undefined;
};

type RawEntry = {
  displayName: string;
  url: string;
  tags?: string[];
  llmsTxt?: string;
  platform?: string;
  excludes?: string[];
};

type RawCatalog = {
  version: string;
  entries: Record<string, RawEntry>;
};

const raw = catalogJson as RawCatalog;

export const catalogVersion: string = raw.version;

export const catalog: readonly CatalogEntry[] = Object.entries(raw.entries)
  .map(([key, entry]) => ({
    key,
    displayName: entry.displayName,
    url: entry.url,
    tags: entry.tags ?? [],
    llmsTxt: entry.llmsTxt,
    platform: entry.platform,
    excludes: entry.excludes,
  }))
  .sort((a, b) => a.displayName.localeCompare(b.displayName));

export const allTags: readonly string[] = Array.from(
  new Set(catalog.flatMap((e) => e.tags)),
).sort();
