/**
 * Unified graph renderer for the watch viewer.
 *
 * Owns both rendering backends — graphviz for "dot", mermaid for "mermaid" —
 * behind one role-named module. Keeping the package imports here (rather than in
 * per-package wrapper files named `graphviz.ts` / `mermaid.ts`) means no module
 * shadows the name of a dependency it imports.
 */

import { Graphviz } from '@hpcc-js/wasm/graphviz';
import type { Mermaid } from 'mermaid';

let graphviz: Graphviz | null = null;
let mermaid: Mermaid | null = null;
let mermaidSeq = 0;

/**
 * Load the graphviz WASM module. Call once before rendering dot graphs; mermaid
 * loads lazily on first use, so it needs no eager init.
 */
export async function initRenderer(): Promise<void> {
  if (!graphviz) {
    graphviz = await Graphviz.load();
  }
}

/** Whether the dot (graphviz) backend has finished loading. */
export function isDotRendererReady(): boolean {
  return graphviz !== null;
}

async function loadMermaid(): Promise<Mermaid> {
  if (mermaid) {
    return mermaid;
  }
  const mod = await import('mermaid');
  mod.default.initialize({
    startOnLoad: false,
    theme: 'dark',
    // clarity's mermaid output uses bracketed node labels with paths and
    // punctuation; the loose security level keeps those intact.
    securityLevel: 'loose',
  });
  mermaid = mod.default;
  return mermaid;
}

/**
 * Render a graph source to SVG, dispatching on the session render format.
 *
 * @param source - dot or mermaid source as produced by the matching formatter
 * @param format - "dot" or "mermaid"
 * @returns SVG string
 */
export async function renderGraph(source: string, format: string): Promise<string> {
  if (format === 'mermaid') {
    const instance = await loadMermaid();
    // Mermaid needs a unique element id per render; a monotonic counter avoids
    // collisions across the live stream of snapshots.
    const { svg } = await instance.render(`clarity-mermaid-${mermaidSeq++}`, source);
    return svg;
  }

  if (!graphviz) {
    throw new Error('Renderer not initialized. Call initRenderer() first.');
  }
  return graphviz.dot(source);
}
