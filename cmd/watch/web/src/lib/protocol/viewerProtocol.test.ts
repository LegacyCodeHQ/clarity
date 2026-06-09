import { describe, it, expect } from 'vitest';
import { normalizeGraphStreamPayload, type Snapshot, type Collection, type CommitSummary } from './viewerProtocol';

const TIMESTAMP = "2026-02-12T10:00:00Z";

function snapshot(id: number, repoId = "primary", dot = `digraph ${id} {}`): Snapshot {
  return { id, repoId, timestamp: TIMESTAMP, dot };
}

function collection(
  id: number,
  snapshots: Snapshot[],
  repoId = "primary",
  commitHistory: CommitSummary[] = []
): Collection {
  return {
    id,
    repoId,
    timestamp: TIMESTAMP,
    snapshots,
    commitHistory,
  };
}

describe('normalizeGraphStreamPayload', () => {
  it('filters malformed snapshot and collection data', () => {
    const normalized = normalizeGraphStreamPayload({
      repos: [{ id: "primary", path: "/repo", label: "repo", isPrimary: true }],
      workingSnapshots: [
        snapshot(1),
        { id: 2, repoId: "primary", timestamp: TIMESTAMP }, // missing dot
        null,
      ],
      pastCollections: [
        collection(10, [snapshot(7), { id: 8, repoId: "primary", timestamp: TIMESTAMP }]), // inner snapshot missing dot
        { id: 11, repoId: "primary", timestamp: TIMESTAMP }, // missing snapshots array
        null,
      ],
      latestWorkingId: "bad", // not a number
      latestPastCollectionId: 22,
    });

    expect(normalized.workingSnapshots).toEqual([snapshot(1)]);
    expect(normalized.pastCollections).toEqual([collection(10, [snapshot(7)])]);
    expect(normalized.latestWorkingId).toBe(0);
    expect(normalized.latestPastCollectionId).toBe(22);
  });

  it('handles non-object input', () => {
    expect(normalizeGraphStreamPayload(null)).toEqual({
      repos: [],
      format: "dot",
      workingSnapshots: [],
      pastCollections: [],
    });
  });

  it('defaults format to "dot" when missing and preserves it when present', () => {
    expect(normalizeGraphStreamPayload({ workingSnapshots: [], pastCollections: [] }).format).toBe("dot");
    expect(
      normalizeGraphStreamPayload({ format: "mermaid", workingSnapshots: [], pastCollections: [] }).format,
    ).toBe("mermaid");
  });

  it('defaults repoId to empty string when missing', () => {
    const normalized = normalizeGraphStreamPayload({
      workingSnapshots: [{ id: 1, timestamp: TIMESTAMP, dot: "digraph {}" }],
      pastCollections: [{
        id: 5,
        timestamp: TIMESTAMP,
        snapshots: [{ id: 2, timestamp: TIMESTAMP, dot: "digraph {}" }],
      }],
    });

    expect(normalized.workingSnapshots[0].repoId).toBe("");
    expect(normalized.pastCollections[0].repoId).toBe("");
    expect(normalized.pastCollections[0].snapshots[0].repoId).toBe("");
  });

  it('normalizes the repos[] tab descriptor list', () => {
    const normalized = normalizeGraphStreamPayload({
      repos: [
        { id: "primary", path: "/repo", label: "clarity-cli", isPrimary: true },
        { id: "wt-abc12345", path: "/tmp/feat", label: "clarity-cli (feat)", isPrimary: false },
        { id: "" }, // empty id should be dropped
        null,
        "garbage",
      ],
      workingSnapshots: [],
      pastCollections: [],
    });

    expect(normalized.repos).toEqual([
      { id: "primary", path: "/repo", label: "clarity-cli", isPrimary: true, active: true },
      { id: "wt-abc12345", path: "/tmp/feat", label: "clarity-cli (feat)", isPrimary: false, active: true },
    ]);
  });

  it('falls back to repo id as label when label missing', () => {
    const normalized = normalizeGraphStreamPayload({
      repos: [{ id: "primary", path: "/repo", isPrimary: true }],
      workingSnapshots: [],
      pastCollections: [],
    });

    expect(normalized.repos[0].label).toBe("primary");
    expect(normalized.repos[0].isPrimary).toBe(true);
  });

  it('normalizes the active flag, defaulting missing to true', () => {
    const normalized = normalizeGraphStreamPayload({
      repos: [
        { id: "primary", path: "/repo", label: "repo", isPrimary: true, active: true },
        { id: "wt-finished", path: "/tmp/done", label: "done", isPrimary: false, active: false },
        { id: "wt-legacy", path: "/tmp/old", label: "old", isPrimary: false }, // no active field
      ],
      workingSnapshots: [],
      pastCollections: [],
    });

    expect(normalized.repos[0].active).toBe(true);
    expect(normalized.repos[1].active).toBe(false);
    // Backward tolerance: a descriptor without `active` is treated as active.
    expect(normalized.repos[2].active).toBe(true);
  });

  it('carries the sessionStart flag through normalization', () => {
    const normalized = normalizeGraphStreamPayload({
      workingSnapshots: [
        { id: 1, repoId: "primary", timestamp: TIMESTAMP, dot: "digraph {}", sessionStart: true },
        { id: 2, repoId: "primary", timestamp: TIMESTAMP, dot: "digraph {}" },
      ],
      pastCollections: [],
    });

    expect(normalized.workingSnapshots[0].sessionStart).toBe(true);
    // Absent/false flag stays falsy (omitted, matching the backend's omitempty).
    expect(normalized.workingSnapshots[1].sessionStart).toBeFalsy();
  });

  it('carries commit history through archived collection normalization', () => {
    const commit = {
      hash: "1234567890abcdef",
      shortHash: "1234567",
      subject: "add timeline shortcut",
      author: "Test User",
      email: "test@example.com",
      timestamp: TIMESTAMP,
    };

    const normalized = normalizeGraphStreamPayload({
      workingSnapshots: [],
      pastCollections: [{
        id: 5,
        repoId: "primary",
        timestamp: TIMESTAMP,
        snapshots: [snapshot(1)],
        commitHistory: [commit, { subject: "missing hash" }],
      }],
    });

    expect(normalized.pastCollections).toEqual([collection(5, [snapshot(1)], "primary", [commit])]);
  });
});
