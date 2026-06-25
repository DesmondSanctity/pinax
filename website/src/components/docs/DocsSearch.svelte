<script lang="ts">
  import { onMount } from 'svelte';

  type Result = {
    url: string;
    excerpt: string;
    meta: { title?: string };
  };

  let { compact = false }: { compact?: boolean } = $props();

  let query = $state('');
  let results: Result[] = $state([]);
  let open = $state(false);
  let loading = $state(false);
  let pagefind: { search: (q: string) => Promise<{ results: { data: () => Promise<Result> }[] }> } | null = null;

  async function ensureLoaded() {
    if (pagefind) return;
    try {
      const url = '/pagefind/pagefind.js';
      const mod = await import(/* @vite-ignore */ url);
      await mod.init?.();
      pagefind = mod;
    } catch {
      pagefind = null;
    }
  }

  async function run() {
    const q = query.trim();
    if (!q) {
      results = [];
      return;
    }
    loading = true;
    await ensureLoaded();
    if (!pagefind) {
      loading = false;
      return;
    }
    const search = await pagefind.search(q);
    const data = await Promise.all(search.results.slice(0, 8).map((r) => r.data()));
    results = data;
    loading = false;
  }

  $effect(() => {
    void query;
    run();
  });

  onMount(() => {
    const onKey = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key === 'k') {
        e.preventDefault();
        open = true;
        setTimeout(() => document.getElementById('pinax-docs-search-input')?.focus(), 0);
      }
      if (e.key === 'Escape') open = false;
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  });
</script>

<button
  type="button"
  class="search-trigger"
  class:compact
  onclick={() => {
    open = true;
    setTimeout(() => document.getElementById('pinax-docs-search-input')?.focus(), 0);
  }}
  aria-label="Search docs"
>
  <svg width="14" height="14" viewBox="0 0 24 24" fill="none" aria-hidden="true">
    <circle cx="11" cy="11" r="7" stroke="currentColor" stroke-width="2" />
    <path d="m20 20-3.5-3.5" stroke="currentColor" stroke-width="2" stroke-linecap="round" />
  </svg>
  <span class="label">Search docs</span>
  <kbd>⌘K</kbd>
</button>

{#if open}
  <div
    class="backdrop"
    role="presentation"
    onclick={(e) => {
      if (e.target === e.currentTarget) open = false;
    }}
  >
    <div class="dialog" role="dialog" aria-modal="true" aria-label="Search docs">
      <div class="dialog-input">
        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" aria-hidden="true">
          <circle cx="11" cy="11" r="7" stroke="currentColor" stroke-width="2" />
          <path d="m20 20-3.5-3.5" stroke="currentColor" stroke-width="2" stroke-linecap="round" />
        </svg>
        <input
          id="pinax-docs-search-input"
          type="search"
          placeholder="Search the docs…"
          bind:value={query}
          autocomplete="off"
          spellcheck="false"
        />
        <button class="esc" type="button" onclick={() => (open = false)}>Esc</button>
      </div>
      <div class="dialog-body">
        {#if loading}
          <p class="hint">Searching…</p>
        {:else if query.trim() === ''}
          <p class="hint">Type to search across every page.</p>
        {:else if results.length === 0}
          <p class="hint">No results.</p>
        {:else}
          <ul>
            {#each results as r}
              <li>
                <a href={r.url}>
                  <span class="title">{r.meta?.title ?? r.url}</span>
                  <span class="excerpt">{@html r.excerpt}</span>
                </a>
              </li>
            {/each}
          </ul>
        {/if}
      </div>
    </div>
  </div>
{/if}

<style>
  .search-trigger {
    display: inline-flex;
    align-items: center;
    gap: 0.5rem;
    height: 32px;
    padding: 0 0.65rem;
    border-radius: 6px;
    background: var(--color-panel);
    border: 1px solid var(--color-hairline);
    color: var(--color-muted);
    font: inherit;
    font-size: 0.8rem;
    cursor: pointer;
    transition: border-color 0.15s;
  }
  .search-trigger:hover {
    border-color: var(--color-fg);
  }
  .search-trigger.compact .label {
    display: none;
  }
  .label {
    flex: 1;
    text-align: left;
  }
  kbd {
    font-family: var(--font-mono);
    font-size: 0.65rem;
    padding: 1px 5px;
    border-radius: 3px;
    border: 1px solid var(--color-hairline);
    background: var(--color-bg);
    color: var(--color-muted);
  }

  .backdrop {
    position: fixed;
    inset: 0;
    background: color-mix(in srgb, var(--color-fg) 35%, transparent);
    z-index: 100;
    display: flex;
    align-items: flex-start;
    justify-content: center;
    padding: 6rem 1rem 1rem;
  }
  .dialog {
    width: 100%;
    max-width: 560px;
    background: var(--color-bg);
    border: 1px solid var(--color-hairline);
    border-radius: 10px;
    box-shadow: 0 20px 50px -10px rgba(0, 0, 0, 0.25);
    overflow: hidden;
  }
  .dialog-input {
    display: flex;
    align-items: center;
    gap: 0.6rem;
    padding: 0.85rem 1rem;
    border-bottom: 1px solid var(--color-hairline);
    color: var(--color-muted);
  }
  .dialog-input input {
    flex: 1;
    border: none;
    outline: none;
    background: transparent;
    font: inherit;
    font-size: 0.95rem;
    color: var(--color-fg);
  }
  .esc {
    border: 1px solid var(--color-hairline);
    background: var(--color-panel);
    border-radius: 4px;
    padding: 2px 6px;
    font-size: 0.7rem;
    color: var(--color-muted);
    cursor: pointer;
  }
  .dialog-body {
    max-height: 60vh;
    overflow-y: auto;
  }
  .hint {
    padding: 1rem;
    color: var(--color-muted);
    font-size: 0.85rem;
  }
  .dialog-body ul {
    list-style: none;
    padding: 0;
    margin: 0;
  }
  .dialog-body li + li {
    border-top: 1px solid var(--color-hairline);
  }
  .dialog-body a {
    display: block;
    padding: 0.75rem 1rem;
    color: var(--color-fg);
    text-decoration: none;
  }
  .dialog-body a:hover {
    background: var(--color-panel);
  }
  .title {
    display: block;
    font-family: var(--font-serif);
    font-size: 0.95rem;
    margin-bottom: 0.2rem;
  }
  .excerpt {
    display: block;
    font-size: 0.8rem;
    color: var(--color-muted);
    line-height: 1.45;
  }
  :global(.dialog-body mark) {
    background: color-mix(in srgb, var(--color-primary) 18%, transparent);
    color: inherit;
    padding: 0 2px;
    border-radius: 2px;
  }
</style>
