/**
 * Mermaid renderer for the watch viewer.
 * Mermaid is loaded lazily on first use so the dot-only path doesn't pay for it.
 */

import type { Mermaid } from 'mermaid';

let mermaidInstance: Mermaid | null = null;
let renderSeq = 0;

async function getMermaid(): Promise<Mermaid> {
  if (mermaidInstance) {
    return mermaidInstance;
  }

  const mod = await import('mermaid');
  const mermaid = mod.default;
  mermaid.initialize({
    startOnLoad: false,
    theme: 'dark',
    // clarity's mermaid output uses bracketed node labels containing paths and
    // punctuation; the loose security level keeps those intact.
    securityLevel: 'loose',
  });
  mermaidInstance = mermaid;
  return mermaidInstance;
}

/**
 * Render a Mermaid flowchart definition to SVG.
 *
 * @param source - Mermaid flowchart source (as produced by the mermaid formatter)
 * @returns SVG string
 */
export async function renderMermaid(source: string): Promise<string> {
  const mermaid = await getMermaid();
  // Mermaid needs a unique element id per render; a monotonic counter avoids
  // collisions across the live stream of snapshots.
  const id = `clarity-mermaid-${renderSeq++}`;
  const { svg } = await mermaid.render(id, source);
  return svg;
}
