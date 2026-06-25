// @ts-check
import { defineConfig } from 'astro/config';
import mdx from '@astrojs/mdx';
import sitemap from '@astrojs/sitemap';
import svelte from '@astrojs/svelte';
import icon from 'astro-icon';
import tailwindcss from '@tailwindcss/vite';

export default defineConfig({
  // pinax.dev not yet registered; pinax.pages.dev is the Cloudflare Pages fallback.
  site: 'https://pinax.pages.dev',
  trailingSlash: 'never',
  integrations: [mdx(), sitemap(), svelte(), icon()],
  vite: {
    plugins: [tailwindcss()],
  },
  markdown: {
    shikiConfig: {
      theme: 'min-light',
      wrap: true,
    },
  },
});
