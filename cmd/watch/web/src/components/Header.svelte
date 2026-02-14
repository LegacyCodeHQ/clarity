<script lang="ts">
  import { viewModel } from '../lib/stores/graphStore';
  import SourceSelector from './SourceSelector.svelte';

  interface Props {
    pageTitle: string;
    connected: boolean;
  }

  let { pageTitle, connected }: Props = $props();

  const statusText = $derived(
    connected ? 'Connected' : 'Reconnecting...'
  );
</script>

<div class="px-4 py-2 bg-[#16213e] border-b border-[#0f3460] flex items-center gap-3 text-[13px]">
  <span class="font-semibold">{pageTitle}</span>
  <span class="inline-flex items-center gap-1.5">
    <span class="w-2 h-2 rounded-full transition-colors duration-300" class:bg-green-400={connected} class:bg-red-400={!connected}></span>
    <span>{statusText}</span>
  </span>
  <span class="flex-1"></span>
  <label for="snapshot-source" class="text-xs">View:</label>
  <SourceSelector />
</div>
