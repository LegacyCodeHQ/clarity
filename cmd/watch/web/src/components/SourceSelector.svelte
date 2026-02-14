<script lang="ts">
  import { viewModel, graphStore } from '../lib/stores/graphStore';

  function handleChange(event: Event) {
    const target = event.target as HTMLSelectElement;
    graphStore.onSourceChange(target.value);
  }
</script>

<select
  id="snapshot-source"
  class="bg-[#1f3f73] text-[#e0e0e0] border border-[#2f5ea4] rounded px-2 py-1 text-xs"
  value={$viewModel.sourceValue}
  onchange={handleChange}
>
  {#each $viewModel.sourceOptions as option}
    <option value={option.value}>{option.text}</option>
  {/each}
</select>
