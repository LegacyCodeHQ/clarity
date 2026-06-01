<script lang="ts">
  import { graphStore, viewModel } from '../lib/stores/graphStore';

  function handleClick(repoID: string) {
    graphStore.onSelectRepo(repoID);
  }

  // Tear down a finished (inactive) worktree tab. The server drops the tab and
  // broadcasts the new set, so the UI updates through the normal SSE flow — no
  // local state mutation here. Active tabs never expose this control.
  async function handleClose(event: MouseEvent, repoID: string) {
    event.stopPropagation();
    try {
      await fetch(`/repos/${encodeURIComponent(repoID)}/close`, { method: 'POST' });
    } catch (err) {
      console.error('Failed to close worktree tab:', err);
    }
  }
</script>

{#if $viewModel.state.repos.length > 1 || $viewModel.state.repos.some((r) => !r.active)}
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
        title={repo.active ? repo.path : `${repo.path} (removed)`}
        class="group flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium rounded-t border-b-2 whitespace-nowrap transition-colors cursor-pointer focus:outline-none focus:ring-2 focus:ring-primary/40"
        class:border-primary={isActive}
        class:text-foreground={isActive && repo.active}
        class:bg-background={isActive}
        class:border-transparent={!isActive}
        class:text-muted-foreground={!isActive || !repo.active}
        class:italic={!repo.active}
        class:hover:text-foreground={!isActive && repo.active}
        class:hover:bg-input={!isActive}
        onclick={() => handleClick(repo.id)}
      >
        <span class="truncate">{repo.label || repo.id}</span>
        {#if !repo.active}
          <span
            role="button"
            tabindex="0"
            aria-label={`Close ${repo.label || repo.id}`}
            title="Close tab"
            class="flex items-center justify-center w-4 h-4 rounded-sm text-muted-foreground hover:text-foreground hover:bg-destructive/20 leading-none"
            onclick={(e) => handleClose(e, repo.id)}
            onkeydown={(e) => { if (e.key === 'Enter' || e.key === ' ') { handleClose(e as unknown as MouseEvent, repo.id); } }}
          >
            ×
          </span>
        {/if}
      </button>
    {/each}
  </div>
{/if}
