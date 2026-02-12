import { Graphviz } from "https://cdn.jsdelivr.net/npm/@hpcc-js/wasm-graphviz@1.6.1/dist/index.js";

const graphviz = await Graphviz.load();
const container = document.getElementById("graph-container");
const statusEl = document.getElementById("status");
const statusText = document.getElementById("status-text");
const sliderEl = document.getElementById("timeline-slider");
const liveBtnEl = document.getElementById("timeline-live");
const modeEl = document.getElementById("timeline-mode");
const metaEl = document.getElementById("timeline-meta");
const sourceEl = document.getElementById("snapshot-source");

let workingSnapshots = [];
let pastSnapshots = [];
let manualMode = false;
let snapshotSource = "working";

function renderGraph(dot) {
  try {
    const svg = graphviz.layout(dot, "svg", "dot");
    container.innerHTML = svg;
    statusText.textContent = "Connected";
    statusEl.classList.remove("disconnected");
  } catch (err) {
    console.error("Graphviz render error:", err);
    statusText.textContent = "Render error";
  }
}

function renderWaitingState() {
  if (snapshotSource === "past") {
    container.innerHTML = '<p id="placeholder">No past snapshots yet. Commit or clear working changes to archive snapshots.</p>';
    return;
  }
  container.innerHTML = '<p id="placeholder">No uncommitted changes. Waiting for file changes...</p>';
}

function formatSnapshotMeta(snapshot, index, total) {
  const time = new Date(snapshot.timestamp);
  return `#${index + 1}/${total} | id ${snapshot.id} | ${time.toLocaleTimeString()}`;
}

function getSelectedSnapshots() {
  return snapshotSource === "past" ? pastSnapshots : workingSnapshots;
}

function syncTimelineUI(selectedSnapshots) {
  const total = selectedSnapshots.length;
  sliderEl.disabled = total <= 1;
  sliderEl.max = total > 0 ? String(total - 1) : "0";
  if (total === 0) {
    sliderEl.value = "0";
  }

  if (!manualMode && total > 0) {
    sliderEl.value = String(total - 1);
  }

  const selected = Number(sliderEl.value || "0");
  const snapshot = selectedSnapshots[selected];
  const modeText = manualMode
    ? (snapshotSource === "past" ? "Past snapshot" : "Working snapshot")
    : (snapshotSource === "past" ? "Past commits (latest)" : "Working directory (live)");
  modeEl.textContent = modeText;
  liveBtnEl.disabled = !manualMode || total === 0;
  sourceEl.value = snapshotSource;

  const canInspectPast = pastSnapshots.length > 0;
  sourceEl.querySelector('option[value="past"]').disabled = !canInspectPast;

  if (snapshot) {
    metaEl.textContent = `${total} snapshots | ${formatSnapshotMeta(snapshot, selected, total)}`;
  } else {
    metaEl.textContent = snapshotSource === "past" ? "0 past snapshots" : "0 working snapshots";
  }
}

function renderSelectedSnapshot() {
  const selectedSnapshots = getSelectedSnapshots();
  if (selectedSnapshots.length === 0) {
    syncTimelineUI(selectedSnapshots);
    renderWaitingState();
    return;
  }
  const idx = manualMode ? Number(sliderEl.value || "0") : selectedSnapshots.length - 1;
  const snapshot = selectedSnapshots[idx];
  if (!snapshot) {
    return;
  }
  renderGraph(snapshot.dot);
  syncTimelineUI(selectedSnapshots);
}

function mergePayload(payload) {
  workingSnapshots = payload.workingSnapshots || [];
  pastSnapshots = payload.pastSnapshots || [];

  if (snapshotSource === "past" && pastSnapshots.length === 0) {
    snapshotSource = "working";
    manualMode = false;
  }

  const selectedSnapshots = getSelectedSnapshots();
  if (selectedSnapshots.length === 0) {
    if (snapshotSource === "working") {
      manualMode = false;
    }
    renderSelectedSnapshot();
    return;
  }

  if (!manualMode) {
    renderSelectedSnapshot();
    return;
  }

  const maxIdx = Math.max(selectedSnapshots.length - 1, 0);
  sliderEl.value = String(Math.min(Number(sliderEl.value || "0"), maxIdx));
  renderSelectedSnapshot();
}

sliderEl.addEventListener("input", function() {
  if (getSelectedSnapshots().length === 0) {
    return;
  }
  manualMode = true;
  renderSelectedSnapshot();
});

liveBtnEl.addEventListener("click", function() {
  manualMode = false;
  renderSelectedSnapshot();
});

sourceEl.addEventListener("change", function(event) {
  const selected = event.target.value === "past" ? "past" : "working";
  if (selected === "past" && pastSnapshots.length === 0) {
    snapshotSource = "working";
    sourceEl.value = "working";
    manualMode = false;
    renderSelectedSnapshot();
    return;
  }

  snapshotSource = selected;
  manualMode = false;
  renderSelectedSnapshot();
});

function connectSSE() {
  const source = new EventSource("/events");

  source.addEventListener("graph", function(event) {
    try {
      const payload = JSON.parse(event.data);
      mergePayload(payload);
    } catch (err) {
      console.error("Invalid graph payload:", err);
      statusText.textContent = "Payload error";
    }
  });

  source.addEventListener("open", function() {
    statusText.textContent = "Connected";
    statusEl.classList.remove("disconnected");
  });

  source.addEventListener("error", function() {
    statusText.textContent = "Reconnecting...";
    statusEl.classList.add("disconnected");
  });
}

connectSSE();
