# prATC Research Learnings

## D3.js + Bun + TypeScript (5.5k nodes)
- **SVG is NOT viable for 5.5k nodes** - each node = DOM element = catastrophic perf
- **Canvas rendering is required** - 10-100x faster than SVG for large datasets
- Use `react-force-graph-2d` (Canvas-based, wraps d3-force-3d, React 19 compatible)
- Or: Use `d3-force` + `d3-selection` with Canvas via `useRef<HTMLCanvasElement>`
- Key React integration pattern: React owns DOM, D3 does math/scales/layout
- D3 v7 has modular force modules: `d3.forceSimulation()`, `d3.forceManyBody()`, `d3.forceLink()`, `d3.forceCollide()`
- Barnes-Hut approximation (`theta`) is critical for 5.5k nodes - reduces O(n²) to O(n log n)
- For pan/zoom: use `d3-zoom` with Canvas transform (not SVG overlay)
- Bundle: d3 is tree-shakeable, import only needed submodules

## TanStack Table + Virtual (large-scale tables)
- TanStack Table v8 handles 100k+ rows client-side
- TanStack Virtual (`useVirtualizer`) renders only visible rows - essential for 5.5k PRs
- Integration: `useVirtualizer` manages scroll container, table rows are virtual items
- Key config: `estimateSize: () => 35`, `overscan: 5`
- For dynamic row heights: use `measureElement` ref callback
- TanStack Table v9 is in alpha (2026) - v8 is stable; v9 has new API with `useTable` instead of `useReactTable`
- Horizontal virtualization for columns is supported via `horizontal: true`

## React 19 Compatibility (current web stack)
- `@tanstack/react-table` v8: works with React 19 (no peer dep conflict)
- `@tanstack/react-virtual` v3: works with React 19
- `react-force-graph-2d` v1.48+: React 19 compatible
- `d3` v7: works with Bun (no browser-specific APIs)
- Next.js 15 (current web stack): Canvas components should use `'use client'` + `useEffect` pattern
- SSR caution: Canvas/WebGL cannot SSR - use dynamic import with `{ ssr: false }` or `useEffect` mounting

## Web Structure Compatibility
- `web/src/charts/` exists - ideal for D3 force graph components
- `web/src/components/` exists - ideal for TanStack Table components
- Next.js `dev --port 3000` already configured
- Bun as package manager confirmed
- Existing vitest setup for testing

## Version Recommendations (as of 2026)
- `d3` v7.x (latest is 7.9.0) - no @types/d3 needed (built-in types)
- `@tanstack/react-table` v8.x (stable)
- `@tanstack/react-virtual` v3.x
- `react-force-graph-2d` v1.48.x (latest)
- Consider `@types/d3-force` only if using isolated d3-force package
