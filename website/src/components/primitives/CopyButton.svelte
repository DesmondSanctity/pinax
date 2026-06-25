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
  class="copy-btn"
  class:copied
>
  {copied ? 'Copied' : label}
</button>

<style>
  .copy-btn {
    font: 500 11px/1 var(--font-mono, ui-monospace, monospace);
    text-transform: uppercase;
    letter-spacing: 0.08em;
    padding: 6px 10px;
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
