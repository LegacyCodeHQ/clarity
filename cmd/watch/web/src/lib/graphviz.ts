/**
 * Graphviz WASM module loader and DOT renderer.
 * Provides async initialization and rendering utilities.
 */

import { Graphviz } from '@hpcc-js/wasm/graphviz';

let graphvizInstance: Graphviz | null = null;

/**
 * Initialize the Graphviz WASM module.
 * This is an async operation that should be called once on app startup.
 */
export async function initGraphviz(): Promise<Graphviz> {
  if (graphvizInstance) {
    return graphvizInstance;
  }

  graphvizInstance = await Graphviz.load();
  return graphvizInstance;
}

/**
 * Render DOT string to SVG.
 * Must call initGraphviz() before using this function.
 *
 * @param dot - DOT language graph definition
 * @returns SVG string
 * @throws Error if Graphviz is not initialized
 */
export async function renderDot(dot: string): Promise<string> {
  if (!graphvizInstance) {
    throw new Error('Graphviz not initialized. Call initGraphviz() first.');
  }

  return graphvizInstance.dot(dot);
}

/**
 * Check if Graphviz is initialized
 */
export function isGraphvizReady(): boolean {
  return graphvizInstance !== null;
}
