import { useEffect, useMemo, useRef, useState } from 'react';
import type { ActivityNote, MemoryTopology, MemoryTopologyNode } from '../types';

interface MemoryGraphProps {
  fetchRuntime: (path: string, init?: RequestInit) => Promise<Response>;
  pushActivity: (kind: ActivityNote['kind'], title: string, body: string, meta?: string) => void;
}

/** Simulation canvas. The SVG scales to its container; these are layout units. */
const W = 1200;
const H = 820;
const SETTLE_TICKS = 260;
// Bigger steps, fewer commits: the simulation itself is cheap next to
// re-rendering several hundred SVG elements, so the layout settles in ~10
// renders instead of ~44.
const TICKS_PER_FRAME = 26;
const NODE_LIMIT = 300;

type Point = { x: number; y: number; vx: number; vy: number };

/**
 * Deterministic ring seeding. Random starts would make the layout differ on
 * every reload, so the same vault always settles into the same picture.
 */
function seedPositions(count: number): Point[] {
  const points: Point[] = [];
  const radius = Math.min(W, H) * 0.36;
  for (let i = 0; i < count; i += 1) {
    const angle = (i / Math.max(1, count)) * Math.PI * 2;
    // A second, faster winding keeps early neighbours from starting collinear.
    const wobble = 1 + 0.28 * Math.sin(i * 2.399);
    points.push({
      x: W / 2 + Math.cos(angle) * radius * wobble,
      y: H / 2 + Math.sin(angle) * radius * wobble,
      vx: 0,
      vy: 0,
    });
  }
  return points;
}

/**
 * A small force-directed layout: Coulomb repulsion between every pair, Hooke
 * springs along links, and weak gravity toward the centre. At a few hundred
 * nodes the naive O(n²) pass is well under a frame, so no quadtree is needed.
 */
function tick(points: Point[], links: Array<[number, number]>, degrees: number[]) {
  const repulsion = 7600;
  const springLength = 96;
  const springK = 0.04;
  const gravity = 0.012;
  const damping = 0.86;

  for (let i = 0; i < points.length; i += 1) {
    const a = points[i];
    for (let j = i + 1; j < points.length; j += 1) {
      const b = points[j];
      let dx = a.x - b.x;
      let dy = a.y - b.y;
      let distSq = dx * dx + dy * dy;
      if (distSq < 0.01) {
        // Perturb coincident nodes deterministically instead of dividing by ~0.
        dx = (i % 7) - 3;
        dy = (j % 7) - 3;
        distSq = dx * dx + dy * dy || 1;
      }
      const dist = Math.sqrt(distSq);
      const force = repulsion / distSq;
      const fx = (dx / dist) * force;
      const fy = (dy / dist) * force;
      a.vx += fx;
      a.vy += fy;
      b.vx -= fx;
      b.vy -= fy;
    }
  }

  for (const [from, to] of links) {
    const a = points[from];
    const b = points[to];
    const dx = b.x - a.x;
    const dy = b.y - a.y;
    const dist = Math.sqrt(dx * dx + dy * dy) || 1;
    const force = (dist - springLength) * springK;
    const fx = (dx / dist) * force;
    const fy = (dy / dist) * force;
    a.vx += fx;
    a.vy += fy;
    b.vx -= fx;
    b.vy -= fy;
  }

  for (let i = 0; i < points.length; i += 1) {
    const p = points[i];
    // Hubs resist gravity less, so they settle toward the middle and the
    // periphery belongs to leaf notes.
    const pull = gravity * (1 + Math.min(degrees[i], 12) * 0.06);
    p.vx += (W / 2 - p.x) * pull;
    p.vy += (H / 2 - p.y) * pull;
    p.vx *= damping;
    p.vy *= damping;
    p.x += Math.max(-24, Math.min(24, p.vx));
    p.y += Math.max(-24, Math.min(24, p.vy));
  }
}

/** Four steps of one hue: fill darkness carries degree, same as node radius. */
function fillStep(degree: number): string {
  if (degree >= 12) return 'var(--viz-4)';
  if (degree >= 6) return 'var(--viz-3)';
  if (degree >= 3) return 'var(--viz-2)';
  return 'var(--viz-1)';
}

function radiusFor(degree: number): number {
  return Math.min(26, 6 + Math.sqrt(degree) * 3.4);
}

export function MemoryGraph({ fetchRuntime, pushActivity }: MemoryGraphProps) {
  const [topology, setTopology] = useState<MemoryTopology | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [includeIsolated, setIncludeIsolated] = useState(false);
  const [query, setQuery] = useState('');
  const [selected, setSelected] = useState<string | null>(null);
  const [hovered, setHovered] = useState<string | null>(null);
  const [positions, setPositions] = useState<Point[]>([]);
  const [view, setView] = useState({ x: 0, y: 0, k: 1 });
  const [settling, setSettling] = useState(false);

  const frameRef = useRef<number | null>(null);
  const dragRef = useRef<{ x: number; y: number; vx: number; vy: number } | null>(null);
  const svgRef = useRef<SVGSVGElement | null>(null);

  // Memoized so the fallbacks keep a stable identity. A fresh `[]` on every
  // render would re-trigger the layout effect, which calls setState — an
  // endless render loop for as long as the graph has not loaded.
  const nodes = useMemo(() => topology?.nodes ?? [], [topology]);
  const edges = useMemo(() => topology?.edges ?? [], [topology]);

  const indexByID = useMemo(() => {
    const map = new Map<string, number>();
    nodes.forEach((node, index) => map.set(node.id, index));
    return map;
  }, [nodes]);

  const links = useMemo(() => {
    const out: Array<[number, number]> = [];
    for (const edge of edges) {
      const from = indexByID.get(edge.source);
      const to = indexByID.get(edge.target);
      if (from !== undefined && to !== undefined) out.push([from, to]);
    }
    return out;
  }, [edges, indexByID]);

  const neighbours = useMemo(() => {
    const map = new Map<string, Set<string>>();
    for (const edge of edges) {
      if (!map.has(edge.source)) map.set(edge.source, new Set());
      if (!map.has(edge.target)) map.set(edge.target, new Set());
      map.get(edge.source)!.add(edge.target);
      map.get(edge.target)!.add(edge.source);
    }
    return map;
  }, [edges]);

  async function loadGraph(isolated = includeIsolated) {
    setLoading(true);
    setError('');
    try {
      const response = await fetchRuntime(`/v1/memory/graph?limit=${NODE_LIMIT}${isolated ? '&isolated=1' : ''}`);
      if (!response.ok) throw new Error(`memory graph ${response.status}`);
      const payload = (await response.json()) as MemoryTopology;
      setTopology(payload);
      setSelected(null);
      setView({ x: 0, y: 0, k: 1 });
    } catch (loadError) {
      const message = String(loadError);
      setError(message);
      pushActivity('error', 'Memory graph unavailable', message);
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    void loadGraph(false);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // Run the simulation in frame-sized slices so the graph visibly settles
  // instead of appearing after a long blocked frame.
  useEffect(() => {
    if (!nodes.length) {
      setPositions([]);
      return;
    }

    const points = seedPositions(nodes.length);
    const degrees = nodes.map((node) => node.degree);
    let done = 0;
    setSettling(true);

    function step() {
      for (let i = 0; i < TICKS_PER_FRAME && done < SETTLE_TICKS; i += 1) {
        tick(points, links, degrees);
        done += 1;
      }
      setPositions(points.map((p) => ({ ...p })));
      if (done < SETTLE_TICKS) {
        frameRef.current = window.requestAnimationFrame(step);
      } else {
        frameRef.current = null;
        setSettling(false);
      }
    }

    frameRef.current = window.requestAnimationFrame(step);
    return () => {
      if (frameRef.current !== null) window.cancelAnimationFrame(frameRef.current);
      frameRef.current = null;
    };
  }, [nodes, links]);

  const needle = query.trim().toLowerCase();
  const matches = useMemo(() => {
    if (!needle) return null;
    const hits = new Set<string>();
    for (const node of nodes) {
      const haystack = `${node.title} ${node.category ?? ''} ${(node.tags ?? []).join(' ')}`.toLowerCase();
      if (haystack.includes(needle)) hits.add(node.id);
    }
    return hits;
  }, [needle, nodes]);

  const focus = hovered ?? selected;
  const focusSet = useMemo(() => {
    if (!focus) return null;
    const set = new Set<string>([focus]);
    neighbours.get(focus)?.forEach((id) => set.add(id));
    return set;
  }, [focus, neighbours]);

  function isDimmed(id: string): boolean {
    if (matches && !matches.has(id)) return true;
    if (focusSet && !focusSet.has(id)) return true;
    return false;
  }

  function onWheel(event: React.WheelEvent) {
    event.preventDefault();
    setView((prev) => {
      const k = Math.min(4, Math.max(0.35, prev.k * (event.deltaY < 0 ? 1.12 : 0.89)));
      return { ...prev, k };
    });
  }

  function onPointerDown(event: React.PointerEvent) {
    if ((event.target as Element).closest('.graph-node')) return;
    dragRef.current = { x: event.clientX, y: event.clientY, vx: view.x, vy: view.y };
    (event.currentTarget as Element).setPointerCapture(event.pointerId);
  }

  function onPointerMove(event: React.PointerEvent) {
    const drag = dragRef.current;
    if (!drag) return;
    setView((prev) => ({ ...prev, x: drag.vx + (event.clientX - drag.x), y: drag.vy + (event.clientY - drag.y) }));
  }

  function onPointerUp(event: React.PointerEvent) {
    dragRef.current = null;
    const target = event.currentTarget as Element;
    if (target.hasPointerCapture(event.pointerId)) target.releasePointerCapture(event.pointerId);
  }

  const selectedNode: MemoryTopologyNode | null = selected ? nodes.find((node) => node.id === selected) ?? null : null;
  const selectedNeighbours = selected ? Array.from(neighbours.get(selected) ?? []) : [];

  // Labelling every node turns the dense core into a smear. Only the hubs carry
  // a standing label; everything else names itself on hover, selection, or a
  // search hit.
  const standingLabels = useMemo(() => {
    const ranked = [...nodes].sort((a, b) => b.degree - a.degree);
    const cap = nodes.length > 120 ? 14 : nodes.length > 40 ? 20 : nodes.length;
    return new Set(ranked.slice(0, cap).filter((node) => node.degree > 0).map((node) => node.id));
  }, [nodes]);

  return (
    <div className="memory-graph">
      <header className="page-head">
        <div className="page-head-text">
          <span className="eyebrow">Memory</span>
          <h2>Knowledge graph</h2>
          <p className="settings-desc">
            Notes in the memory vault, linked by the Obsidian wikilinks the recall path walks. Node size and
            shade both carry link count; a dashed outline is a link with no note behind it yet.
          </p>
        </div>
        <div className="panel-actions">
          <button className="ghost" type="button" onClick={() => void loadGraph()} disabled={loading}>
            {loading ? 'Loading' : 'Refresh'}
          </button>
        </div>
      </header>

      {topology ? (
        <section className="graph-stats" aria-label="Graph summary">
          <div>
            <span>Notes shown</span>
            <strong>{nodes.length}</strong>
          </div>
          <div>
            <span>Links</span>
            <strong>{edges.length}</strong>
          </div>
          <div>
            <span>Isolated</span>
            <strong>{topology.isolated_count}</strong>
          </div>
          <div>
            <span>Unresolved</span>
            <strong>{topology.unresolved}</strong>
          </div>
          <div>
            <span>Vault total</span>
            <strong>{topology.total_notes}</strong>
          </div>
        </section>
      ) : null}

      <section className="graph-toolbar" aria-label="Graph controls">
        <label className="graph-search">
          <span>Search</span>
          <input
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            placeholder="Title, category, or tag"
            spellCheck={false}
          />
        </label>
        <label className="auto-scroll-toggle" title="Notes with no links at all">
          <input
            type="checkbox"
            checked={includeIsolated}
            onChange={(event) => {
              setIncludeIsolated(event.target.checked);
              void loadGraph(event.target.checked);
            }}
          />
          <span>Show isolated notes</span>
        </label>
        <div className="graph-zoom">
          <button className="mini-button" type="button" onClick={() => setView((v) => ({ ...v, k: Math.min(4, v.k * 1.2) }))}>+</button>
          <button className="mini-button" type="button" onClick={() => setView((v) => ({ ...v, k: Math.max(0.35, v.k / 1.2) }))}>−</button>
          <button className="mini-button" type="button" onClick={() => setView({ x: 0, y: 0, k: 1 })}>Reset</button>
        </div>
      </section>

      {error ? <div className="trajectory-empty error-text">{error}</div> : null}
      {!error && !loading && nodes.length === 0 ? (
        <div className="trajectory-empty">
          <strong>No linked notes yet</strong>
          <span>The graph is built from `[[wikilinks]]` between memory notes. It fills in as memories cross-reference each other.</span>
        </div>
      ) : null}

      {nodes.length > 0 ? (
        <div className="graph-body">
          <div className="graph-canvas">
            <svg
              ref={svgRef}
              viewBox={`0 0 ${W} ${H}`}
              onWheel={onWheel}
              onPointerDown={onPointerDown}
              onPointerMove={onPointerMove}
              onPointerUp={onPointerUp}
              onPointerLeave={onPointerUp}
              role="img"
              aria-label={`Memory graph with ${nodes.length} notes and ${edges.length} links`}
            >
              <g transform={`translate(${view.x} ${view.y}) scale(${view.k})`}>
                <g className="graph-edges">
                  {edges.map((edge, index) => {
                    const from = indexByID.get(edge.source);
                    const to = indexByID.get(edge.target);
                    if (from === undefined || to === undefined) return null;
                    const a = positions[from];
                    const b = positions[to];
                    if (!a || !b) return null;
                    const active = focusSet ? focusSet.has(edge.source) && focusSet.has(edge.target) : false;
                    return (
                      <line
                        key={`${edge.source}-${edge.target}-${index}`}
                        x1={a.x}
                        y1={a.y}
                        x2={b.x}
                        y2={b.y}
                        className={`graph-edge ${focusSet ? (active ? 'active' : 'dim') : ''}`}
                      />
                    );
                  })}
                </g>
                <g className="graph-nodes">
                  {nodes.map((node, index) => {
                    const point = positions[index];
                    if (!point) return null;
                    const r = radiusFor(node.degree);
                    const dim = isDimmed(node.id);
                    const showLabel =
                      standingLabels.has(node.id) ||
                      focus === node.id ||
                      (focusSet?.has(node.id) ?? false) ||
                      (matches?.has(node.id) ?? false);
                    return (
                      <g
                        key={node.id}
                        className={`graph-node ${dim ? 'dim' : ''} ${selected === node.id ? 'selected' : ''}`}
                        transform={`translate(${point.x} ${point.y})`}
                        onMouseEnter={() => setHovered(node.id)}
                        onMouseLeave={() => setHovered(null)}
                        onClick={() => setSelected((prev) => (prev === node.id ? null : node.id))}
                      >
                        <circle
                          r={r}
                          fill={node.resolved ? fillStep(node.degree) : 'transparent'}
                          className={node.resolved ? '' : 'unresolved'}
                        />
                        {showLabel ? (
                          <text y={r + 13} textAnchor="middle" className="graph-label">
                            {node.title.length > 22 ? `${node.title.slice(0, 21)}…` : node.title}
                          </text>
                        ) : null}
                      </g>
                    );
                  })}
                </g>
              </g>
            </svg>

            <div className="graph-legend">
              <span><i className="swatch s1" /> 1–2 links</span>
              <span><i className="swatch s2" /> 3–5</span>
              <span><i className="swatch s3" /> 6–11</span>
              <span><i className="swatch s4" /> 12+</span>
              <span><i className="swatch dashed" /> unresolved</span>
              {settling ? <span className="graph-settling">settling…</span> : null}
            </div>
          </div>

          <aside className="graph-detail">
            {selectedNode ? (
              <>
                <div className="graph-detail-head">
                  <h3>{selectedNode.title}</h3>
                  <button className="icon-button tiny" type="button" title="Clear selection" onClick={() => setSelected(null)}>
                    ×
                  </button>
                </div>
                <div className="kv-list">
                  <div className="kv"><span>Links</span><strong>{selectedNode.degree}</strong></div>
                  {selectedNode.category ? <div className="kv"><span>Category</span><strong>{selectedNode.category}</strong></div> : null}
                  {selectedNode.tier ? <div className="kv"><span>Tier</span><strong>{selectedNode.tier}</strong></div> : null}
                  <div className="kv"><span>Importance</span><strong>{selectedNode.importance.toFixed(2)}</strong></div>
                  {selectedNode.resolved ? null : <div className="kv"><span>Status</span><strong>No note yet</strong></div>}
                </div>
                {selectedNode.path ? <p className="graph-path">{selectedNode.path}</p> : null}
                {selectedNode.tags?.length ? (
                  <div className="graph-tags">
                    {selectedNode.tags.map((tag) => <span key={tag}>#{tag}</span>)}
                  </div>
                ) : null}
                <div className="graph-neighbours">
                  <h4>Connected to {selectedNeighbours.length}</h4>
                  {selectedNeighbours.slice(0, 40).map((id) => {
                    const neighbour = nodes.find((node) => node.id === id);
                    if (!neighbour) return null;
                    return (
                      <button key={id} type="button" className="graph-neighbour" onClick={() => setSelected(id)}>
                        <span>{neighbour.title}</span>
                        <small>{neighbour.degree}</small>
                      </button>
                    );
                  })}
                </div>
              </>
            ) : (
              <div className="graph-detail-empty">
                <p>Select a node to inspect the note behind it.</p>
                <p className="muted">Drag to pan, scroll to zoom, hover to isolate a neighbourhood.</p>
              </div>
            )}
          </aside>
        </div>
      ) : null}
    </div>
  );
}
