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

<div class="timeline">
  <span class="mode">{$viewModel.timeline.modeText}</span>
  <input
    type="range"
    min="0"
    max={$viewModel.timeline.sliderMax}
    value={$viewModel.timeline.sliderValue}
    disabled={$viewModel.timeline.sliderDisabled}
    oninput={handleSliderInput}
  />
  <button
    disabled={$viewModel.timeline.liveButtonDisabled}
    onclick={handleJumpToLatest}
  >
    Jump to latest
  </button>
  <span class="meta">{$viewModel.timeline.metaText}</span>
</div>

<style>
  .timeline {
    padding: 10px 16px;
    background: #16213e;
    border-top: 1px solid #0f3460;
    display: flex;
    align-items: center;
    gap: 10px;
    font-size: 12px;
  }

  input[type="range"] {
    flex: 1;
    min-width: 120px;
  }

  button {
    background: #1f3f73;
    color: #e0e0e0;
    border: 1px solid #2f5ea4;
    border-radius: 4px;
    padding: 4px 8px;
    cursor: pointer;
    font-size: 12px;
  }

  button:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }

  .meta {
    min-width: 120px;
    text-align: right;
    color: #9eb2d3;
  }
</style>
