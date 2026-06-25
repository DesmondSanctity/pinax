<script lang="ts">
  type Props = { tags: readonly string[]; total: number };
  let { tags, total }: Props = $props();

  let query = $state('');
  let selected: Set<string> = $state(new Set());
  let visible = $state(total);

  function toggle(tag: string) {
    const next = new Set(selected);
    if (next.has(tag)) next.delete(tag);
    else next.add(tag);
    selected = next;
  }

  function clear() {
    query = '';
    selected = new Set();
  }

  function apply() {
    const q = query.trim().toLowerCase();
    const sel = selected;
    let count = 0;
    document.querySelectorAll<HTMLElement>('[data-catalog-card]').forEach((el) => {
      const search = (el.dataset['search'] ?? '').toLowerCase();
      const cardTags = (el.dataset['tags'] ?? '').split(' ').filter(Boolean);
      const matchSearch = q === '' || search.includes(q);
      const matchTags = sel.size === 0 || cardTags.some((t) => sel.has(t));
      const show = matchSearch && matchTags;
      el.hidden = !show;
      if (show) count++;
    });
    visible = count;
    const empty = document.querySelector<HTMLElement>('[data-catalog-empty]');
    if (empty) empty.hidden = count > 0;
  }

  $effect(() => {
    void query;
    void selected;
    apply();
  });

  const hasFilter = $derived(query.trim() !== '' || selected.size > 0);
</script>

<div class="filter">
  <div class="search-row">
    <svg
      class="search-icon"
      width="16"
      height="16"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      stroke-width="2"
      stroke-linecap="round"
      stroke-linejoin="round"
      aria-hidden="true"
    >
      <circle cx="11" cy="11" r="7" />
      <path d="m21 21-4.3-4.3" />
    </svg>
    <input
      type="search"
      bind:value={query}
      placeholder="Search by name, tag, or URL…"
      aria-label="Search catalog"
    />
    {#if hasFilter}
      <button type="button" class="clear" onclick={clear}>Clear</button>
    {/if}
  </div>

  <div class="chips" role="group" aria-label="Filter by tag">
    {#each tags as tag}
      <button
        type="button"
        class="chip"
        class:active={selected.has(tag)}
        aria-pressed={selected.has(tag)}
        onclick={() => toggle(tag)}
      >
        {tag}
      </button>
    {/each}
  </div>

  <p class="count">
    {visible} of {total}
    {total === 1 ? 'entry' : 'entries'}
  </p>
</div>

<style>
  .filter {
    display: flex;
    flex-direction: column;
    gap: 14px;
  }
  .search-row {
    position: relative;
    display: flex;
    align-items: center;
    gap: 8px;
  }
  .search-icon {
    position: absolute;
    left: 12px;
    color: var(--color-muted);
    pointer-events: none;
  }
  input[type='search'] {
    flex: 1;
    padding: 10px 12px 10px 36px;
    border: 1px solid var(--color-hairline);
    border-radius: 8px;
    background: var(--color-bg);
    color: var(--color-fg);
    font: 400 14px/1.4 var(--font-sans);
    outline: none;
    transition:
      border-color 120ms,
      box-shadow 120ms;
  }
  input[type='search']:focus {
    border-color: var(--color-primary);
    box-shadow: 0 0 0 3px color-mix(in srgb, var(--color-primary) 16%, transparent);
  }
  .clear {
    padding: 8px 12px;
    border: 1px solid var(--color-hairline);
    border-radius: 8px;
    background: var(--color-bg);
    color: var(--color-muted);
    font: 500 12px/1 var(--font-sans);
    cursor: pointer;
    transition:
      color 120ms,
      border-color 120ms;
  }
  .clear:hover {
    color: var(--color-fg);
    border-color: var(--color-fg);
  }
  .chips {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
  }
  .chip {
    padding: 4px 10px;
    border: 1px solid var(--color-hairline);
    border-radius: 999px;
    background: var(--color-bg);
    color: var(--color-muted);
    font: 500 10px/1 var(--font-sans);
    text-transform: uppercase;
    letter-spacing: 0.06em;
    cursor: pointer;
    transition:
      color 120ms,
      border-color 120ms,
      background 120ms;
  }
  .chip:hover {
    color: var(--color-fg);
    border-color: var(--color-fg);
  }
  .chip.active {
    color: var(--color-bg);
    background: var(--color-primary);
    border-color: var(--color-primary);
  }
  .count {
    font: 500 11px/1 var(--font-mono);
    text-transform: uppercase;
    letter-spacing: 0.12em;
    color: var(--color-muted);
  }
</style>
