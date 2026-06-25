import { site } from './site';

export type SeoInput = {
  title?: string | undefined;
  description?: string | undefined;
  path?: string | undefined;
};

export function buildSeo({ title, description, path = '/' }: SeoInput) {
  const fullTitle = title ? `${title} — ${site.name}` : `${site.name} — ${site.tagline}`;
  return {
    title: fullTitle,
    description: description ?? site.description,
    canonical: new URL(path, site.url).toString(),
    ogImage: new URL('/og.png', site.url).toString(),
  };
}

export function softwareApplicationJsonLd() {
  return {
    '@context': 'https://schema.org',
    '@type': 'SoftwareApplication',
    name: site.name,
    description: site.description,
    url: site.url,
    applicationCategory: 'DeveloperApplication',
    operatingSystem: 'macOS, Linux, Windows',
    offers: { '@type': 'Offer', price: '0', priceCurrency: 'USD' },
    license: 'https://opensource.org/licenses/MIT',
    codeRepository: site.repo,
    image: new URL('/og.png', site.url).toString(),
  };
}
