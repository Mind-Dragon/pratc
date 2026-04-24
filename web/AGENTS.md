# AGENTS.md — Next.js Dashboard

**Stack:** Next.js 15 + React 19 + bun

## Component Patterns

**Page = thin wrapper:**
- Fetch data in `getServerSideProps`
- Pass to `Layout` component
- No state in pages

**Component = full implementation:**
- All state: `useState`, `useMemo`, `useCallback`
- No Context, no Redux
- `TriageView.tsx`: TanStack Table + Virtual for Outlook-style scrolling
- `SyncStatusPanel.tsx`: Polling sync progress

**Dynamic imports (SSR disabled):**
```typescript
const ForceGraph = dynamic(() => import("react-force-graph-2d"), { ssr: false });
```

## Data Fetching

**All pages use `getServerSideProps`:**
- No SWR, no React Query
- Default repo: `opencode-ai/opencode`
- API URL: `NEXT_PUBLIC_PRATC_API_URL` or `http://localhost:7400`

**API client (`lib/api.ts`):**
- `fetchJSON<T>(path, fallback)` — returns fallback on error, never throws
- All functions return `null` on error (never throw)
- Functions: `fetchAnalysis`, `fetchCluster`, `fetchGraph`, `fetchPlan`, `fetchSettings`, `postSetting`, `deleteSetting`, `exportSettingsYAML`, `importSettingsYAML`

## Styling

**Single global CSS:** `styles/globals.css`
- `:root` CSS variables for theming
- BEM-like naming: `stat-card--sky`, `cluster-card--red`
- No Tailwind, no CSS modules, no styled-components
- `_app.tsx` injects global CSS

## Testing

**Framework:** vitest + @testing-library/react

**Patterns:**
- Mock factories: `createMockPR()`, `createMockAnalysis()`
- `vi.mock("next/router")` — mock router
- `vi.mock("react-force-graph-2d")` — mock force graph
- Assertions: `screen.getByText()`, `screen.getByRole()`
- Run: `bun run test`

## Navigation

`Navigation.tsx` — sidebar with active state
- Routes: `/`, `/inbox`, `/triage`, `/plan`, `/graph`, `/settings`
- `inbox.tsx` is alias to `TriageView`

## Routes

| File | Purpose |
|------|---------|
| `index.tsx` | Dashboard: stats cards + cluster preview |
| `triage.tsx` | Sequential PR review with actions |
| `graph.tsx` | Force-directed graph (dynamic import) |
| `plan.tsx` | Merge plan panel |
| `settings.tsx` | 9 config sections (weights, thresholds, sync) |

## Gotchas

**Settings API mismatch:**
- Web client uses RESTful paths: `/api/settings`
- Server uses query params: `/api/settings?repo=`
- Fix in progress

**No loading states:**
- Renders "Disconnected" when API unavailable
- No skeletons, no spinners

**Force graph SSR:**
- Must use `ssr: false` dynamic import
- `next.config.ts` has SSR disabled globally for this

**Type consistency:**
- `types/api.ts` mirrors Go/Python types
- All JSON keys are `snake_case`
- Response always includes `repo` + `generatedAt`
