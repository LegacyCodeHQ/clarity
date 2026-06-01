/**
 * State management for the watch viewer.
 * Pure functions for state transitions and view model computation.
 */

import type {
  Snapshot,
  Collection,
  GraphStreamPayload,
  RepoDescriptor,
} from '../protocol/viewerProtocol';

/**
 * Snapshots and archived cycles for a single working tree (tab).
 */
export interface RepoBucket {
  working: Snapshot[];
  past: Collection[];
}

export interface ViewerState {
  // Multi-repo: the registered tab set + which tab is active.
  repos: RepoDescriptor[];
  selectedRepoID: string;
  byRepo: Record<string, RepoBucket>;

  // Effective view for the selected repo — derived from byRepo[selectedRepoID]
  // on every state update so downstream timeline/graph code stays unchanged.
  workingSnapshots: Snapshot[];
  pastCollections: Collection[];

  selectedCollectionID: number | null;
  selectedCollectionSnapshotIndex: number;
  liveSnapshotIndex: number | null;
}

export interface SourceOption {
  value: string;
  text: string;
}

export interface TimelineViewModel {
  modeText: string;
  sliderDisabled: boolean;
  sliderMax: string;
  sliderValue: string;
  liveButtonDisabled: boolean;
  metaText: string;
  // Index of the session-start snapshot within the currently displayed list
  // (live working set or selected collection), or null if it isn't present —
  // e.g. after a commit archives the cycle the marker belonged to.
  sessionStartIndex: number | null;
}

export interface ViewModel {
  state: ViewerState;
  sourceValue: string;
  sourceOptions: SourceOption[];
  renderDot: string | null;
  timeline: TimelineViewModel;
}

type TimeFormatter = (timestamp: string) => string;

const DEFAULT_REPO_ID = "primary";

function clamp(value: number, min: number, max: number): number {
  return Math.max(min, Math.min(value, max));
}

function findSessionStartIndex(snapshots: Snapshot[]): number | null {
  const index = snapshots.findIndex((snapshot) => snapshot.sessionStart === true);
  return index === -1 ? null : index;
}

function formatTime(timestamp: string): string {
  return new Date(timestamp).toLocaleTimeString();
}

export function formatSnapshotMeta(
  snapshot: Snapshot,
  index: number,
  total: number,
  timeFormatter: TimeFormatter = formatTime
): string {
  return `#${index + 1}/${total} | id ${snapshot.id} | ${timeFormatter(snapshot.timestamp)}`;
}

export function getSelectedCollection(state: ViewerState): Collection | null {
  if (state.selectedCollectionID === null) {
    return null;
  }
  return state.pastCollections.find((collection) => collection.id === state.selectedCollectionID) || null;
}

/**
 * Picks the next selected repo when the previous selection becomes invalid
 * (e.g., its tab was removed). Prefers the existing selection, then "primary",
 * then the first repo in the list.
 */
function resolveSelectedRepoID(repos: RepoDescriptor[], current: string): string {
  if (repos.length === 0) {
    return current || DEFAULT_REPO_ID;
  }
  if (repos.some((r) => r.id === current)) {
    return current;
  }
  const primary = repos.find((r) => r.id === DEFAULT_REPO_ID);
  if (primary) {
    return primary.id;
  }
  return repos[0].id;
}

function projectBucketsForState(
  state: ViewerState,
  selectedRepoID: string,
): { workingSnapshots: Snapshot[]; pastCollections: Collection[] } {
  const bucket = state.byRepo[selectedRepoID];
  return {
    workingSnapshots: bucket ? bucket.working : [],
    pastCollections: bucket ? bucket.past : [],
  };
}

export function normalizeState(state: Partial<ViewerState>): ViewerState {
  const repos = Array.isArray(state.repos) ? state.repos : [];
  const selectedRepoID = resolveSelectedRepoID(repos, state.selectedRepoID ?? DEFAULT_REPO_ID);

  // Ad-hoc callers (notably test fixtures) can set workingSnapshots /
  // pastCollections directly without populating a byRepo bucket. When the
  // selected repo has no bucket yet, treat those arrays as its initial state.
  // mergePayload clears these arrays explicitly so an empty payload doesn't
  // bring stale snapshots back through this fallback.
  const fallbackWorking = Array.isArray(state.workingSnapshots) ? state.workingSnapshots : [];
  const fallbackPast = Array.isArray(state.pastCollections) ? state.pastCollections : [];
  const byRepo: Record<string, RepoBucket> = state.byRepo ? { ...state.byRepo } : {};
  if (!byRepo[selectedRepoID] && (fallbackWorking.length > 0 || fallbackPast.length > 0)) {
    byRepo[selectedRepoID] = { working: fallbackWorking, past: fallbackPast };
  }

  const projected = projectBucketsForState({ ...(state as ViewerState), byRepo }, selectedRepoID);

  const next: ViewerState = {
    repos,
    selectedRepoID,
    byRepo,
    workingSnapshots: projected.workingSnapshots,
    pastCollections: projected.pastCollections,
    selectedCollectionID: state.selectedCollectionID ?? null,
    selectedCollectionSnapshotIndex: Number.isFinite(state.selectedCollectionSnapshotIndex)
      ? state.selectedCollectionSnapshotIndex!
      : 0,
    liveSnapshotIndex: state.liveSnapshotIndex === null || Number.isFinite(state.liveSnapshotIndex)
      ? state.liveSnapshotIndex ?? null
      : null,
  };

  if (next.workingSnapshots.length === 0) {
    next.liveSnapshotIndex = null;
  }

  const selectedCollection = getSelectedCollection(next);
  if (next.selectedCollectionID !== null && !selectedCollection) {
    next.selectedCollectionID = null;
    next.selectedCollectionSnapshotIndex = 0;
  }

  if (next.selectedCollectionID === null) {
    const total = next.workingSnapshots.length;
    const latestIndex = total > 0 ? total - 1 : 0;
    if (next.liveSnapshotIndex !== null) {
      next.liveSnapshotIndex = clamp(next.liveSnapshotIndex, 0, latestIndex);
      if (next.liveSnapshotIndex === latestIndex) {
        next.liveSnapshotIndex = null;
      }
    }
    return next;
  }

  const collection = getSelectedCollection(next);
  const snapshots = collection ? collection.snapshots || [] : [];
  if (snapshots.length === 0) {
    next.selectedCollectionSnapshotIndex = 0;
    return next;
  }
  next.selectedCollectionSnapshotIndex = clamp(
    next.selectedCollectionSnapshotIndex,
    0,
    snapshots.length - 1,
  );
  return next;
}

/**
 * Buckets a flat payload by repoId. Snapshots/collections without a repoId
 * fall into the primary bucket — keeps backward-tolerance with older payloads
 * and with single-repo callers.
 */
function bucketPayload(payload: GraphStreamPayload): Record<string, RepoBucket> {
  const byRepo: Record<string, RepoBucket> = {};
  const knownIds = new Set<string>();
  for (const repo of payload.repos || []) {
    knownIds.add(repo.id);
    byRepo[repo.id] = { working: [], past: [] };
  }
  for (const snap of payload.workingSnapshots || []) {
    const id = snap.repoId || DEFAULT_REPO_ID;
    if (!byRepo[id]) {
      byRepo[id] = { working: [], past: [] };
    }
    byRepo[id].working.push(snap);
  }
  for (const coll of payload.pastCollections || []) {
    const id = coll.repoId || DEFAULT_REPO_ID;
    if (!byRepo[id]) {
      byRepo[id] = { working: [], past: [] };
    }
    byRepo[id].past.push(coll);
  }
  // Drop any synthesized empty buckets that weren't declared by repos[] AND
  // received no snapshots — keeps `byRepo` honest.
  for (const id of Object.keys(byRepo)) {
    if (!knownIds.has(id) && byRepo[id].working.length === 0 && byRepo[id].past.length === 0) {
      delete byRepo[id];
    }
  }
  return byRepo;
}

export function mergePayload(state: ViewerState, payload: GraphStreamPayload): ViewerState {
  const repos = payload.repos || state.repos;
  const byRepo = bucketPayload(payload);
  return normalizeState({
    ...state,
    repos,
    byRepo,
    // Discard the previous projection so it can't leak through normalizeState's
    // fallback when the new bucket is empty for the selected repo.
    workingSnapshots: [],
    pastCollections: [],
  });
}

export function selectRepo(state: ViewerState, repoID: string): ViewerState {
  if (!state.repos.some((r) => r.id === repoID)) {
    return state;
  }
  if (state.selectedRepoID === repoID) {
    return state;
  }
  // Switching tabs resets the per-tab timeline selection.
  return normalizeState({
    ...state,
    selectedRepoID: repoID,
    selectedCollectionID: null,
    selectedCollectionSnapshotIndex: 0,
    liveSnapshotIndex: null,
  });
}

export function applySliderInput(state: ViewerState, rawValue: string): ViewerState {
  const next = normalizeState(state);
  if (next.selectedCollectionID === null) {
    if (next.workingSnapshots.length === 0) {
      return next;
    }
    const latestIndex = next.workingSnapshots.length - 1;
    const idx = clamp(Number(rawValue || "0"), 0, latestIndex);
    next.liveSnapshotIndex = idx === latestIndex ? null : idx;
    return normalizeState(next);
  }

  const collection = getSelectedCollection(next);
  const snapshots = collection ? collection.snapshots || [] : [];
  if (snapshots.length === 0) {
    return next;
  }
  next.selectedCollectionSnapshotIndex = clamp(Number(rawValue || "0"), 0, snapshots.length - 1);
  return normalizeState(next);
}

export function applyLiveSelection(state: ViewerState): ViewerState {
  return normalizeState({
    ...state,
    liveSnapshotIndex: null,
    selectedCollectionID: null,
    selectedCollectionSnapshotIndex: 0,
  });
}

export function applySourceSelection(state: ViewerState, selected: string): ViewerState {
  if (selected === "live") {
    return applyLiveSelection(state);
  }
  if (!selected.startsWith("collection:")) {
    return applyLiveSelection(state);
  }

  const selectedID = Number(selected.split(":")[1]);
  if (!Number.isFinite(selectedID)) {
    return applyLiveSelection(state);
  }

  return normalizeState({
    ...state,
    selectedCollectionID: selectedID,
    selectedCollectionSnapshotIndex: 0,
  });
}

export function getSourceOptions(state: ViewerState, timeFormatter: TimeFormatter = formatTime): SourceOption[] {
  const liveOption: SourceOption = {
    value: "live",
    text: "Current working directory (live)",
  };
  const orderedCollections = [...state.pastCollections].reverse();
  const collectionOptions = orderedCollections.map((collection, index) => {
    const number = state.pastCollections.length - index;
    const snapshots = collection.snapshots || [];
    return {
      value: `collection:${collection.id}`,
      text: `Collection ${number} (${snapshots.length} snapshots, ${timeFormatter(collection.timestamp)})`,
    };
  });

  return [liveOption, ...collectionOptions];
}

export function getViewModel(state: ViewerState, timeFormatter: TimeFormatter = formatTime): ViewModel {
  const normalized = normalizeState(state);
  const sourceValue = normalized.selectedCollectionID === null
    ? "live"
    : `collection:${normalized.selectedCollectionID}`;

  if (normalized.selectedCollectionID === null) {
    const total = normalized.workingSnapshots.length;
    const latestIndex = total > 0 ? total - 1 : 0;
    const selectedIndex = normalized.liveSnapshotIndex === null
      ? latestIndex
      : normalized.liveSnapshotIndex;

    return {
      state: normalized,
      sourceValue,
      sourceOptions: getSourceOptions(normalized, timeFormatter),
      renderDot: total > 0 ? normalized.workingSnapshots[selectedIndex]!.dot : null,
      timeline: {
        modeText: normalized.liveSnapshotIndex === null
          ? "Working directory (live)"
          : "Working directory snapshot",
        sliderDisabled: total <= 1,
        sliderMax: total > 0 ? String(total - 1) : "0",
        sliderValue: total > 0 ? String(selectedIndex) : "0",
        liveButtonDisabled: total === 0 || normalized.liveSnapshotIndex === null,
        metaText: total === 0
          ? "0 working snapshots"
          : `${total} working snapshots | ${formatSnapshotMeta(
            normalized.workingSnapshots[selectedIndex]!,
            selectedIndex,
            total,
            timeFormatter,
          )}`,
        sessionStartIndex: findSessionStartIndex(normalized.workingSnapshots),
      },
    };
  }

  const selectedCollection = getSelectedCollection(normalized);
  const snapshots = selectedCollection ? selectedCollection.snapshots || [] : [];
  const total = snapshots.length;

  return {
    state: normalized,
    sourceValue,
    sourceOptions: getSourceOptions(normalized, timeFormatter),
    renderDot: total > 0 ? snapshots[normalized.selectedCollectionSnapshotIndex]!.dot : null,
    timeline: {
      modeText: "Snapshot collection",
      sliderDisabled: total <= 1,
      sliderMax: total > 0 ? String(total - 1) : "0",
      sliderValue: total > 0 ? String(normalized.selectedCollectionSnapshotIndex) : "0",
      liveButtonDisabled: false,
      metaText: total === 0
        ? "Collection is empty"
        : `${total} snapshots | ${formatSnapshotMeta(
          snapshots[normalized.selectedCollectionSnapshotIndex]!,
          normalized.selectedCollectionSnapshotIndex,
          total,
          timeFormatter,
        )}`,
      sessionStartIndex: findSessionStartIndex(snapshots),
    },
  };
}
