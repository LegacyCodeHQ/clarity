<script lang="ts">
  import { onMount } from 'svelte';
  import { viewModel } from '../lib/stores/graphStore';
  import { initGraphviz, renderDot } from '../lib/graphviz';

  let container: HTMLDivElement;
  let graphvizReady = $state(false);
  let renderError = $state<string | null>(null);

  onMount(async () => {
    try {
      await initGraphviz();
      graphvizReady = true;
    } catch (err) {
      console.error('Failed to initialize Graphviz:', err);
      renderError = 'Failed to load Graphviz';
    }
  });

  async function renderGraph(dot: string) {
    if (!graphvizReady || !container) return;

    try {
      const svg = await renderDot(dot);
      container.innerHTML = svg;
      renderError = null;
    } catch (err) {
      console.error('Graphviz render error:', err);
      renderError = 'Render error';
    }
  }

  $effect(() => {
    if ($viewModel.renderDot && graphvizReady) {
      renderGraph($viewModel.renderDot);
    } else if (!$viewModel.renderDot && container) {
      container.innerHTML = '<p class="placeholder">No uncommitted changes. Waiting for file changes...</p>';
    }
  });
</script>

<div class="graph-container" bind:this={container}>
  {#if !graphvizReady}
    <p class="placeholder">Loading Graphviz...</p>
  {:else if renderError}
    <p class="placeholder error">{renderError}</p>
  {:else if !$viewModel.renderDot}
    <p class="placeholder">Waiting for graph data...</p>
  {/if}
</div>

<style>
  .graph-container {
    flex: 1;
    overflow: auto;
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 16px;
    background: #ffffff;
  }

  .graph-container :global(svg) {
    max-width: 100%;
    max-height: 100%;
  }

  .placeholder {
    color: #666;
    font-size: 14px;
  }

  .placeholder.error {
    color: #f87171;
  }
</style>
