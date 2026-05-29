<script lang="ts">
  import { graphStore, viewModel } from '../lib/stores/graphStore';

  function handleClick(repoID: string) {
    graphStore.onSelectRepo(repoID);
  }
</script>

{#if $viewModel.state.repos.length > 1}
  <div
    class="px-4 pt-1.5 pb-0 bg-card border-b border-border flex items-end gap-1 overflow-x-auto"
    role="tablist"
    aria-label="Working trees"
  >
    {#each $viewModel.state.repos as repo (repo.id)}
      {@const isActive = repo.id === $viewModel.state.selectedRepoID}
      <button
        type="button"
        role="tab"
        aria-selected={isActive}
        title={repo.path}
        class="px-3 py-1.5 text-xs font-medium rounded-t border-b-2 whitespace-nowrap transition-colors cursor-pointer focus:outline-none focus:ring-2 focus:ring-primary/40"
        class:border-primary={isActive}
        class:text-foreground={isActive}
        class:bg-background={isActive}
        class:border-transparent={!isActive}
        class:text-muted-foreground={!isActive}
        class:hover:text-foreground={!isActive}
        class:hover:bg-input={!isActive}
        onclick={() => handleClick(repo.id)}
      >
        {repo.label || repo.id}
      </button>
    {/each}
  </div>
{/if}
