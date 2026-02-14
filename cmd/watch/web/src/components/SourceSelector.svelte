<script lang="ts">
  import { viewModel, graphStore } from '../lib/stores/graphStore';

  function handleChange(event: Event) {
    const target = event.target as HTMLSelectElement;
    graphStore.onSourceChange(target.value);
  }
</script>

<select
  id="snapshot-source"
  value={$viewModel.sourceValue}
  onchange={handleChange}
>
  {#each $viewModel.sourceOptions as option}
    <option value={option.value}>{option.text}</option>
  {/each}
</select>

<style>
  select {
    background: #1f3f73;
    color: #e0e0e0;
    border: 1px solid #2f5ea4;
    border-radius: 4px;
    padding: 4px 8px;
    font-size: 12px;
  }
</style>
