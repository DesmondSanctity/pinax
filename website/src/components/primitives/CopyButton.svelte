<script lang="ts">
  type Props = { text: string; label?: string };
  let { text, label = 'Copy' }: Props = $props();
  let copied = $state(false);
  let timer: ReturnType<typeof setTimeout> | undefined;

  async function copy() {
    try {
      await navigator.clipboard.writeText(text);
      copied = true;
      clearTimeout(timer);
      timer = setTimeout(() => (copied = false), 1600);
    } catch {
      copied = false;
    }
  }
</script>

<button
  type="button"
  onclick={copy}
  aria-label={copied ? 'Copied to clipboard' : `Copy ${label}`}
  title={copied ? 'Copied' : `Copy ${label}`}
  class="copy-btn"
  class:copied
>
  {#if copied}
    <svg
      width="14"
      height="14"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      stroke-width="2.25"
      stroke-linecap="round"
      stroke-linejoin="round"
      aria-hidden="true"
    >
      <polyline points="20 6 9 17 4 12" />
    </svg>
  {:else}
    <svg
      width="14"
      height="14"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      stroke-width="2"
      stroke-linecap="round"
      stroke-linejoin="round"
      aria-hidden="true"
    >
      <rect x="9" y="9" width="13" height="13" rx="2" ry="2" />
      <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1" />
    </svg>
  {/if}
</button>

<style>
  .copy-btn {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 28px;
    height: 28px;
    padding: 0;
    border-radius: 6px;
    border: 1px solid var(--color-hairline);
    background: var(--color-bg);
    color: var(--color-muted);
    cursor: pointer;
    transition:
      color 120ms,
      border-color 120ms,
      background 120ms;
  }
  .copy-btn:hover {
    color: var(--color-fg);
    border-color: var(--color-fg);
  }
  .copy-btn.copied {
    color: var(--color-success);
    border-color: var(--color-success);
  }
</style>
