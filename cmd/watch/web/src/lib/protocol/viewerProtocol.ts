/**
 * Type definitions and normalization functions for the SSE graph stream protocol.
 * These functions validate and normalize untrusted JSON payloads from the server.
 */

export interface RepoDescriptor {
  id: string;
  path: string;
  label: string;
  isPrimary: boolean;
  // False once the underlying git worktree is removed — the tab becomes a
  // frozen, closable record. A missing flag is treated as active.
  active: boolean;
}

export interface Snapshot {
  id: number;
  repoId?: string;
  timestamp: string;
  dot: string;
  // True for the first snapshot recorded for a worktree this session — the
  // state present when the watcher attached. Omitted (falsy) otherwise.
  sessionStart?: boolean;
}

export interface Collection {
  id: number;
  repoId?: string;
  timestamp: string;
  snapshots: Snapshot[];
}

export interface GraphStreamPayload {
  repos?: RepoDescriptor[];
  workingSnapshots: Snapshot[];
  pastCollections: Collection[];
  latestWorkingId?: number;
  latestPastCollectionId?: number;
}

function normalizeRepo(repo: unknown): RepoDescriptor | null {
  if (!repo || typeof repo !== "object") {
    return null;
  }
  const r = repo as Record<string, unknown>;
  if (typeof r.id !== "string" || r.id === "") {
    return null;
  }
  return {
    id: r.id,
    path: typeof r.path === "string" ? r.path : "",
    label: typeof r.label === "string" ? r.label : r.id,
    isPrimary: r.isPrimary === true,
    // Treat a missing flag as active so older payloads keep their tabs pinned.
    active: r.active !== false,
  };
}

function normalizeSnapshot(snapshot: unknown): Snapshot | null {
  if (!snapshot || typeof snapshot !== "object") {
    return null;
  }
  const s = snapshot as Record<string, unknown>;
  if (typeof s.dot !== "string") {
    return null;
  }

  const normalized: Snapshot = {
    id: Number.isFinite(s.id) ? (s.id as number) : 0,
    repoId: typeof s.repoId === "string" ? s.repoId : "",
    timestamp: typeof s.timestamp === "string" ? s.timestamp : new Date(0).toISOString(),
    dot: s.dot,
  };
  // Preserve the marker only when set, mirroring the backend's omitempty so
  // unmarked snapshots stay free of the field.
  if (s.sessionStart === true) {
    normalized.sessionStart = true;
  }
  return normalized;
}

function normalizeCollection(collection: unknown): Collection | null {
  if (!collection || typeof collection !== "object") {
    return null;
  }
  const c = collection as Record<string, unknown>;
  if (!Array.isArray(c.snapshots)) {
    return null;
  }

  return {
    id: Number.isFinite(c.id) ? (c.id as number) : 0,
    repoId: typeof c.repoId === "string" ? c.repoId : "",
    timestamp: typeof c.timestamp === "string" ? c.timestamp : new Date(0).toISOString(),
    snapshots: c.snapshots
      .map(normalizeSnapshot)
      .filter((snapshot): snapshot is Snapshot => snapshot !== null),
  };
}

/**
 * Normalizes untrusted SSE JSON payloads from the watch server.
 * Keeps only fields needed by the viewer state machine.
 */
export function normalizeGraphStreamPayload(payload: unknown): GraphStreamPayload {
  if (!payload || typeof payload !== "object") {
    return {
      repos: [],
      workingSnapshots: [],
      pastCollections: [],
    };
  }

  const p = payload as Record<string, unknown>;

  return {
    repos: Array.isArray(p.repos)
      ? p.repos.map(normalizeRepo).filter((repo): repo is RepoDescriptor => repo !== null)
      : [],
    workingSnapshots: Array.isArray(p.workingSnapshots)
      ? p.workingSnapshots.map(normalizeSnapshot).filter((snapshot): snapshot is Snapshot => snapshot !== null)
      : [],
    pastCollections: Array.isArray(p.pastCollections)
      ? p.pastCollections.map(normalizeCollection).filter((collection): collection is Collection => collection !== null)
      : [],
    latestWorkingId: Number.isFinite(p.latestWorkingId) ? (p.latestWorkingId as number) : 0,
    latestPastCollectionId: Number.isFinite(p.latestPastCollectionId) ? (p.latestPastCollectionId as number) : 0,
  };
}
