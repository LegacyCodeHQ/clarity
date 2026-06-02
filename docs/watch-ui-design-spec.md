# Clarity Watch UI — Design Brief & Redesign Spec

> **Purpose.** A briefing for the designer: what the Watch UI shows today, the
> information architecture and its problems, the new worktree-tab lifecycle we
> are building, an assessment of the screen's visual hierarchy, and a proposed
> redesign direction. The intent is to restructure the information architecture
> and arrive at a more elegant, legible design.
>
> **Status.** Discussion brief — not yet a final visual design.
> **Source of truth (current code):**
> `clarity/clarity-cli/cmd/watch/web/` (Svelte) and
> `clarity/clarity-cli/cmd/show/formatters/formatter_dot.go` (graph encoding).

---

## 1. What the page is

A single full-height, dark (VS Code / Zed–inspired) screen that live-streams a
**dependency graph of your uncommitted changes**. As you edit files, the Go
server pushes new graph snapshots over SSE (`/events`); you can scrub backward
through the session's history. **The graph is the hero**; everything else is
navigation and context around it.

Today it is a flat four-row vertical stack (`App.svelte`):

```
┌─────────────────────────────────────────────────────┐
│  HEADER         title · ─── · source ▾ · ● status    │  ← fixed
├─────────────────────────────────────────────────────┤
│  REPO TABS      [tree A] [tree B]   (only if >1 repo)│  ← conditional
├─────────────────────────────────────────────────────┤
│                                                       │
│  GRAPH CANVAS   the rendered SVG dependency graph     │  ← flex-1 (hero)
│                                                       │
├─────────────────────────────────────────────────────┤
│  TIMELINE       ▓▓▓▓░░░░ |amber| · [Live] · #3/12 …  │  ← fixed
└─────────────────────────────────────────────────────┘
```

---

## 2. Current information architecture (zone-by-zone inventory)

### A. Header — `Header.svelte`
| Element | What it shows | Notes |
|---|---|---|
| **Page title** | Server-injected project/repo name | `text-sm` semibold, left-aligned |
| **Source selector** ▾ | `Current working directory (live)` + one row per archived "Collection N (X snapshots, timestamp)" | The switch between *live* and *history* lives here |
| **Connection dot** | 2px dot — green w/ glow = Connected, red = Reconnecting… | Status text appears only on hover (`title`) |

### B. Repo / worktree tabs — `RepoTabs.svelte`
- Renders **only when there is more than one working tree**. Tab label =
  `repo.label`; active tab gets a primary-colored underline; full path shown
  only on hover tooltip.

### C. Graph canvas — `GraphContainer.svelte`
The Graphviz-WASM-rendered SVG, centered on a darker inset panel. Four mutually
exclusive states:
1. **Loading** — skeleton + "Loading Graphviz…"
2. **Error** — "Render error" / "Failed to load Graphviz"
3. **Empty** — icon + "Waiting for changes / Make changes to your files to see
   the dependency graph appear here"
4. **Graph rendered** — the SVG

No zoom / pan / fit controls — only native browser scroll on overflow.

### D. Timeline footer — `Timeline.svelte`
| Element | What it shows |
|---|---|
| **Scrubber** | Range slider across all snapshots in the current source; blue fill shows position |
| **Amber marker** | "Session start" tick — where Clarity began watching (explained in tooltip) |
| **Live button** | Jumps to latest; disabled when already live |
| **Meta text** | `12 working snapshots \| #3/12 \| id 47 \| 2:14:09 PM` |

---

## 3. The graph's hidden visual language

This is the most information-dense part of the page — and **none of it is
documented anywhere in the UI**. From `formatter_dot.go`:

| Visual encoding | Meaning |
|---|---|
| **Box = node** | A file |
| **Light-green fill** | Test file |
| **White fill** | File of the majority file-type |
| **Other fill colors** | Color-per-extension (when multiple types present) |
| **Red border + red dashed edges** | File/edge participates in a dependency **cycle** |
| **Dashed border (gray)** | Pruned node (context, not directly changed) |
| **Dotted light-green node + dotted edge** | "Phantom" test — tests that *would* be affected |
| **🪴 prefix** | Newly added file |
| **`+12 −4` in label** | Per-file additions / deletions |
| **Arrow** | Dependency direction |

So the screen encodes change-impact, cyclic risk, test coverage, and churn — all
as color/border/emoji with **zero legend**.

---

## 4. Observations & IA problems (current state)

1. **No legend.** The richest layer of meaning (cycles, tests, new files, churn,
   phantom tests) is invisible knowledge. This is the #1 gap.
2. **The temporal model is never explained.** The core concept — *live working
   dir → snapshots accumulate as you edit → a commit archives them into a
   "collection"* — is split across two disconnected controls (source dropdown in
   the header, scrubber in the footer) and never stated.
3. **Two competing time controls.** "Live vs Collection" (header) and "scrub
   within source" (footer) operate on the same axis but sit at opposite ends of
   the screen.
4. **Meta text is debug-oriented.** `#3/12 | id 47 | 2:14:09 PM` reads like a log
   line, not a human summary.
5. **No textual summary of the change.** The graph shows everything implicitly
   but says nothing in words — no "3 files changed, +40/−12, 1 new cycle
   introduced, 5 tests impacted." Insight must be *spotted* in the picture.
6. **Health / cycles aren't first-class.** A newly introduced cycle — arguably
   the single most important signal — is just "some red somewhere in the graph."
7. **Connection status is too quiet.** A 2px dot with hover-only text; a dropped
   stream (stale graph) is easy to miss.
8. **Canvas has no viewport affordances.** No fit-to-screen, zoom, or node
   interaction on what is often a large graph.
9. **Repo path & metadata are tooltip-only**, never persistently visible.

---

## 5. New requirement — the worktree-tab lifecycle

We are now giving worktree tabs a **state machine bound to the worktree's
existence**. Today `RepoTabs.svelte` models only one state (present +
selectable). The lifecycle introduces three more:

| State | Trigger | Selectable? | Closeable? | Visual treatment |
|---|---|---|---|---|
| **Live** | Worktree created (server-driven) | ✓ | ✗ — no X | Normal tab, full color |
| **Orphaned / tombstone** | Worktree deleted on disk | ✓ (history still viewable) | ✓ — X appears | Dimmed, "(deleted)" label, tombstone/unplugged icon |
| **Closed** | User dismisses tombstone | — | — | Gone permanently, not re-addable |

**Rules:**
- A tab **appears automatically** when a new worktree is created (server-driven,
  not a user action).
- While the worktree is **live**, the user can **only interact** with the tab
  (select it). It **cannot be closed**.
- Only when the worktree is **deleted** does the tab become closeable.
- Once **closed**, the tab is gone and **cannot be re-added**.

### Design implications
1. **The close affordance is inverted from every tab metaphor users know.** In
   browsers/editors you close a *live* tab. Here you *can't* — the X only unlocks
   after the worktree dies. Don't show a greyed-out X (reads as a bug); show
   **no** X on live tabs, and on hover give a one-line tooltip:
   *"Active worktree — the tab closes when the worktree is removed."*
2. **The death transition must be perceptible.** The live → tombstone moment is
   server-driven and invisible if you're not watching. It needs a state-change
   animation and a persistent altered appearance, or the X just silently appears
   and the user never learns the rule. The tombstone state also does real work:
   it preserves the snapshots/history of a tree that no longer exists so the user
   can finish reading before dismissing.
3. **This breaks the "show tabs only when >1" rule.** A level that owns lifecycle
   state must be **persistent and stable** — creation, death, and dismissal each
   need a fixed place to happen. A region that pops in/out causes a layout jump
   the first time a worktree appears and gives the tombstone nowhere to live.
4. **Primary worktree is the permanent anchor** (`isPrimary`) — it should never
   reach the tombstone/closeable state.

**Deeper point:** because users can't freely open/close these, they are *less
like tabs and more like a managed list of contexts with status*. The "switch
active view" part of the tab metaphor still works, but we are really rendering
**worktree objects with state**, and the UI should read that way.

---

## 6. Visual hierarchy assessment — do we have the right levels?

**Verdict in one line:** the screen has roughly the **right number of
conceptual levels (six), none redundant** — but the **visual weight does not
match the conceptual nesting** in three specific places, and the lifecycle work
makes one of them worse.

The concepts form a clean containment hierarchy (each narrows the scope of the
one above):

```
L1  WHERE   Project being watched            ⊃
L2  WHICH   Worktree (now stateful)          ⊃
L3  WHEN¹   Source: live vs archived cycle    ⊃
L4  WHEN²   Snapshot: exact frame in time     →
L5  WHAT    The graph (hero)                   ⊃
L6  DETAIL  Node/file encoding (churn/cycle/test/new)
       +    SYSTEM  connection / streaming state (cross-cutting)
```

How each is expressed today:

| Level | Should weigh | Currently weighs | Verdict |
|---|---|---|---|
| L1 Project | Low (ambient identity) | Low (small title) | ✓ ok |
| **L2 Worktree** | **High** (scope + lifecycle objects) | **Low** (thin strip, vanishes at 1) | ✗ **under-weighted** |
| **L3 Source** | Medium, *nested under L2* | Medium, *in header above L2* | ✗ **inverted altitude** |
| L4 Snapshot | Medium | Medium, but cryptic meta | ~ split |
| L5 Graph | High (hero) | High (flex-1) | ✓ correct |
| L6 Node detail | Supported (legend) | Implicit only, no legend | ✗ unsupported |
| System state | Visible | 2px hover-only dot | ✗ too quiet |

**The three real problems:**

1. **Altitude inversion (L2 vs L3).** Worktree is a *broader* scope than Source —
   you pick the tree, then pick when within it. But Source gets a persistent,
   prominent dropdown in the header while Worktree gets a faint strip *below* it.
   The bigger scope is rendered as the smaller element. The DOM order even reads
   L1 → L3 → L2 → L4, scrambling the nesting.
2. **Disappearing scope (L2).** A level that owns lifecycle state cannot be
   allowed to disappear at count = 1. With the new requirement this is no longer
   a nicety — it's a correctness issue, because creation/death/dismissal need a
   stable stage.
3. **Split temporal axis (L3 + L4).** Choosing *which moment you're looking at* is
   one mental operation, but it's torn between the header (source) and footer
   (scrubber). They are the same family of decision at two altitudes and should
   read as one nested control, not two.

**Corrective principle for the designer:** rank visual weight by *scope breadth
and state-bearing*, not by where the controls happened to land. That yields, top
to bottom: **Worktree (broad scope + lifecycle) → Source/Snapshot unified (the
"when") → Graph (hero) → Node legend (detail)**, with Project as ambient identity
and System state promoted out of a 2px dot into something legible.

---

## 7. Proposed redesign direction

Reorganize from "four stacked strips" into **three intent-based zones —
Context · Canvas · Insight** — with one unified timeline and a persistent,
state-aware worktree region.

```
┌──────────────────────────────────────────────────────────────┐
│ CONTEXT BAR                                                    │
│  ◇ project   │ worktrees:  [tree A] [tree B ✕deleted]  │ ● Live│
├──────────────────────────────────────────┬───────────────────┤
│                                           │  INSIGHT PANEL     │
│  CANVAS                                   │                    │
│   (graph)                                 │  This change       │
│                                           │  3 files · +40 −12 │
│   ┌── viewport controls ──┐               │  🌱 1 new file     │
│   │ ⊕ ⊖ ⤢ fit            │               │  ⚠ 1 new cycle    │
│   └───────────────────────┘               │  ✓ 5 tests touched │
│                                           │                    │
│   ┌─ Legend (collapsible) ─┐              │  Files ▾           │
│   │ ■ test  ■ new  ⊘ cycle │              │   foo.go  +12 −2   │
│   └────────────────────────┘              │   bar.go  +8       │
├──────────────────────────────────────────┴───────────────────┤
│ TIMELINE   ⏱ session ──●──────◇commit────────▷  · 2m ago · Live│
└──────────────────────────────────────────────────────────────┘
```

**Key moves:**

- **Promote a right-hand Insight panel** that translates the graph's visual
  encoding into plain language: files changed, churn (+/−), new files, **cycles
  introduced/resolved**, tests impacted — plus a scrollable file list that
  highlights/centers a node on hover. Biggest single upgrade: it makes the tool
  *readable*, not just *viewable*.
- **Always-available legend** (collapsible chip-strip docked on the canvas) so the
  color language stops being tribal knowledge.
- **Unify the two time controls into one timeline.** Make "Live" the right anchor
  of a single continuous track; mark **session start** and **commit / collection
  boundaries** as labeled ticks on that same track. The header source-dropdown
  collapses into picking a point on the timeline. Replace `#3/12 | id 47` with
  human labels ("now · live", "2 min ago", "at last commit").
- **Give worktrees a persistent, state-aware home** in the context bar — always
  visible (even at one worktree), expressing the Live / tombstone / closeable
  states from §5, with the primary worktree as a permanent anchor.
- **Elevate connection state** into the context bar as a labeled, animated pill
  (`● Live · streaming` / `◌ Reconnecting…`) rather than a hover-only dot, with a
  subtle "stale" treatment on the canvas when the stream drops.
- **Add real viewport controls** to the canvas (zoom in/out, fit-to-screen, reset)
  and make nodes interactive (click → focus/file path, hover → highlight
  dependents).
- **Surface health as a status accent:** when a new cycle appears, the Insight
  panel's cycle row goes amber/red and can pulse — the most important structural
  signal can't be missed.
- **Keep the restrained Zed/VS Code aesthetic.** The redesign is about *structure
  and labeling*, not decoration: a clear type scale, generous canvas breathing
  room, one accent color (`#569cd6`) reserved for "live/now," and amber/red
  reserved exclusively for structural warnings.

---

## 8. Open questions for design

- Worktree region as **horizontal tabs** vs a **vertical list** once it carries
  state and count grows — which scales better with the tombstone treatment?
- Where does the tombstone go after dismissal — silent removal, or a brief
  "closed" confirmation so the irreversibility is clear?
- Insight panel: always open, collapsible, or responsive (hidden on narrow
  windows)?
- Does the unified timeline need named/snap-to commit markers, or is a plain
  continuous scrub with a session-start tick enough for v1?
