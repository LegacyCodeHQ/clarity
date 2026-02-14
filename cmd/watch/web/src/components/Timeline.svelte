<script lang="ts">
  import { viewModel, graphStore } from '../lib/stores/graphStore';

  function handleSliderInput(event: Event) {
    const target = event.target as HTMLInputElement;
    graphStore.onSliderInput(target.value);
  }

  function handleJumpToLatest() {
    graphStore.onJumpToLatest();
  }
</script>

<div class="px-4 py-2.5 bg-[#16213e] border-t border-[#0f3460] flex items-center gap-2.5 text-xs">
  <span>{$viewModel.timeline.modeText}</span>
  <input
    type="range"
    class="flex-1 min-w-[120px]"
    min="0"
    max={$viewModel.timeline.sliderMax}
    value={$viewModel.timeline.sliderValue}
    disabled={$viewModel.timeline.sliderDisabled}
    oninput={handleSliderInput}
  />
  <button
    class="bg-[#1f3f73] text-[#e0e0e0] border border-[#2f5ea4] rounded px-2 py-1 cursor-pointer text-xs disabled:opacity-50 disabled:cursor-not-allowed"
    disabled={$viewModel.timeline.liveButtonDisabled}
    onclick={handleJumpToLatest}
  >
    Jump to latest
  </button>
  <span class="min-w-[120px] text-right text-[#9eb2d3]">{$viewModel.timeline.metaText}</span>
</div>
