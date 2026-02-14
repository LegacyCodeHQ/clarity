/**
 * Type definitions and normalization functions for the SSE graph stream protocol.
 * These functions validate and normalize untrusted JSON payloads from the server.
 */

export interface Snapshot {
  id: number;
  timestamp: string;
  dot: string;
}

export interface Collection {
  id: number;
  timestamp: string;
  snapshots: Snapshot[];
}

export interface GraphStreamPayload {
  workingSnapshots: Snapshot[];
  pastCollections: Collection[];
  latestWorkingId?: number;
  latestPastCollectionId?: number;
}

function normalizeSnapshot(snapshot: unknown): Snapshot | null {
  if (!snapshot || typeof snapshot !== "object") {
    return null;
  }
  const s = snapshot as Record<string, unknown>;
  if (typeof s.dot !== "string") {
    return null;
  }

  return {
    id: Number.isFinite(s.id) ? (s.id as number) : 0,
    timestamp: typeof s.timestamp === "string" ? s.timestamp : new Date(0).toISOString(),
    dot: s.dot,
  };
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
      workingSnapshots: [],
      pastCollections: [],
    };
  }

  const p = payload as Record<string, unknown>;

  return {
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
