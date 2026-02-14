<script lang="ts">
  import { viewModel, graphStore } from '../lib/stores/graphStore';

  function handleChange(event: Event) {
    const target = event.target as HTMLSelectElement;
    graphStore.onSourceChange(target.value);
  }
</script>

<select
  id="snapshot-source"
  class="bg-secondary text-secondary-foreground border border-input rounded-md px-3 py-1.5 text-xs focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 transition-colors"
  value={$viewModel.sourceValue}
  onchange={handleChange}
>
  {#each $viewModel.sourceOptions as option}
    <option value={option.value}>{option.text}</option>
  {/each}
</select>
