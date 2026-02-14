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

<div class="flex-1 overflow-auto flex items-center justify-center p-4 bg-white [&_svg]:max-w-full [&_svg]:max-h-full" bind:this={container}>
  {#if !graphvizReady}
    <p class="text-gray-600 text-sm">Loading Graphviz...</p>
  {:else if renderError}
    <p class="text-red-400 text-sm">{renderError}</p>
  {:else if !$viewModel.renderDot}
    <p class="text-gray-600 text-sm">Waiting for graph data...</p>
  {/if}
</div>
