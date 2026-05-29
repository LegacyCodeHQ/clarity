/**
 * Svelte store wrapper for viewer state management.
 * Provides reactive state updates and view model derivation.
 */

import { writable, derived } from 'svelte/store';
import {
  normalizeState,
  mergePayload,
  applySliderInput,
  applyLiveSelection,
  applySourceSelection,
  selectRepo,
  getViewModel,
  type ViewerState,
  type ViewModel,
} from '../viewer/viewerState';
import type { GraphStreamPayload } from '../protocol/viewerProtocol';

function createGraphStore() {
  const initialState: ViewerState = normalizeState({
    repos: [],
    selectedRepoID: "primary",
    byRepo: {},
    workingSnapshots: [],
    pastCollections: [],
    selectedCollectionID: null,
    selectedCollectionSnapshotIndex: 0,
    liveSnapshotIndex: null,
  });

  const { subscribe, update } = writable<ViewerState>(initialState);

  return {
    subscribe,

    mergePayload: (payload: GraphStreamPayload) => {
      update(state => mergePayload(state, payload));
    },

    onSliderInput: (rawValue: string) => {
      update(state => applySliderInput(state, rawValue));
    },

    onJumpToLatest: () => {
      update(state => applyLiveSelection(state));
    },

    onSourceChange: (selected: string) => {
      update(state => applySourceSelection(state, selected));
    },

    onSelectRepo: (repoID: string) => {
      update(state => selectRepo(state, repoID));
    },

    reset: () => {
      update(() => initialState);
    },
  };
}

export const graphStore = createGraphStore();

/**
 * Derived store that computes the view model from the current state
 */
export const viewModel = derived<typeof graphStore, ViewModel>(
  graphStore,
  ($state) => getViewModel($state)
);
