# prATC UI Wiring Guide

**Last Updated:** 2026-03-23  
**System Version:** v0.1  
**Stack:** Next.js 15 + React 19 + bun

## Table of Contents

1. [Architecture Overview](#architecture-overview)
2. [Directory Structure](#directory-structure)
3. [Page Routes](#page-routes)
4. [Component Hierarchy](#component-hierarchy)
5. [State Management](#state-management)
6. [Data Fetching Patterns](#data-fetching-patterns)
7. [Styling System](#styling-system)
8. [Testing Patterns](#testing-patterns)
9. [API Client](#api-client)
10. [Configuration](#configuration)

---

## Architecture Overview

The prATC web dashboard follows a **thin page, fat component** architecture:

- **Pages** handle data fetching only (via `getServerSideProps`)
- **Components** contain all state, logic, and markup
- **No global state** (no Context, no Redux)
- **No client-side data fetching libraries** (no SWR, no React Query)

```
Request
   │
   ▼
┌─────────────┐
│  Next.js    │ ─── Routes to page
│  Router     │
└──────┬──────┘
       │
       ▼
┌─────────────┐
│   Page      │ ─── getServerSideProps fetches data
│  (pages/)   │
└──────┬──────┘
       │
       ▼
┌─────────────┐
│   Layout    │ ─── Shared layout wrapper
│  components │
└──────┬──────┘
       │
       ▼
┌─────────────┐
│ Components  │ ─── Full implementation with hooks
│(components/)│
└─────────────┘
```

---

## Directory Structure

```
web/
├── src/
│   ├── pages/              # Next.js pages (routes)
│   │   ├── index.tsx       # Dashboard overview
│   │   ├── inbox.tsx       # PR inbox (alias to TriageView)
│   │   ├── triage.tsx      # Sequential triage workflow
│   │   ├── plan.tsx        # Merge plan panel
│   │   ├── graph.tsx       # Interactive dependency graph
│   │   ├── settings.tsx    # Configuration
│   │   └── _app.tsx        # App wrapper (global CSS)
│   │
│   ├── components/         # React components
│   │   ├── Layout.tsx      # Shared layout with navigation
│   │   ├── Navigation.tsx  # Sidebar navigation
│   │   ├── TriageView.tsx  # PR triage with TanStack Table
│   │   ├── SyncStatusPanel.tsx  # Sync progress polling
│   │   ├── StatCard.tsx    # Dashboard stat cards
│   │   ├── ClusterCard.tsx # Cluster preview cards
│   │   └── ...
│   │
│   ├── lib/                # Utility functions
│   │   └── api.ts          # API client
│   │
│   ├── types/              # TypeScript definitions
│   │   └── api.ts          # API type mirrors
│   │
│   ├── styles/             # Styling
│   │   └── globals.css     # Single global CSS file
│   │
│   └── __tests__/          # Test files
│       └── *.test.tsx
│
├── next.config.ts          # Next.js config
├── package.json
└── tsconfig.json
```

---

## Page Routes

| Route | File | Purpose | Data Fetching |
|-------|------|---------|---------------|
| `/` | `index.tsx` | Dashboard: stats cards + cluster preview | `fetchAnalysis()`, `fetchCluster()` |
| `/inbox` | `inbox.tsx` | PR inbox view | Alias to `/triage` |
| `/triage` | `triage.tsx` | Sequential PR review workflow | `fetchAnalysis()` |
| `/plan` | `plan.tsx` | Merge plan panel | `fetchPlan()` |
| `/graph` | `graph.tsx` | Interactive dependency graph | `fetchGraph()` |
| `/settings` | `settings.tsx` | Configuration (9 sections) | `fetchSettings()` |

### Route Details

#### `/` — Dashboard (index.tsx)

**Purpose:** High-level overview with statistics and cluster preview.

**Fetches:**
- Analysis data (counts, staleness signals)
- Cluster data (cluster list, health status)

**Components rendered:**
- `Layout`
- `StatCard` (multiple instances)
- `ClusterCard` (for each cluster)

#### `/inbox` and `/triage` — Triage Workflow

**Purpose:** Sequential PR review with actions.

**Note:** `inbox.tsx` is an alias to `TriageView` component.

**Key features:**
- TanStack Table with virtual scrolling (Outlook-style)
- Row selection and bulk actions
- PR detail sidebar

**Components:**
- `Layout`
- `TriageView` (main component with full state)

#### `/plan` — Merge Planning

**Purpose:** Generate and view merge plans.

**Features:**
- Target PR count selector
- Strategy mode selector (combination/permutation/with_replacement)
- Candidate list with scores
- Ordering visualization

#### `/graph` — Dependency Graph

**Purpose:** Interactive force-directed graph visualization.

**Special handling:**
- Dynamic import with `ssr: false` (force graph requires DOM)
- Uses `react-force-graph-2d`

```typescript
const ForceGraph = dynamic(() => import("react-force-graph-2d"), { ssr: false });
```

#### `/settings` — Configuration

**Purpose:** 9 configuration sections:

1. Weights and thresholds
2. Duplicate detection settings
3. Overlap detection settings
4. Conflict detection settings
5. Staleness scoring
6. Sync configuration
7. Bot detection
8. Merge planning
9. General settings

---

## Component Hierarchy

### Layout Structure

```
Layout (Layout.tsx)
├── Navigation (Navigation.tsx)
│   ├── Logo
│   └── NavLinks: /, /inbox, /triage, /plan, /graph, /settings
│
└── Main Content (children)
    └── Page-specific content
```

### Dashboard Hierarchy

```
Layout
├── Navigation
└── main
    ├── h1 "ATC Overview"
    ├── section.stats-grid
    │   └── StatCard (x6)
    │       ├── stat-card--sky
    │       ├── stat-card--amber
    │       ├── stat-card--emerald
    │       ├── stat-card--rose
    │       ├── stat-card--violet
    │       └── stat-card--slate
    │
    └── section.clusters-grid
        └── ClusterCard (for each cluster)
            ├── cluster-card--red
            ├── cluster-card--amber
            ├── cluster-card--emerald
            └── cluster-card--sky
```

### Triage Hierarchy

```
Layout
├── Navigation
└── main
    └── TriageView
        ├── header (title + actions)
        ├── TanStack Table
        │   ├── Virtualizer (for large lists)
        │   ├── Column headers
        │   └── Row selection
        └── Detail panel (selected PR)
```

### Graph Hierarchy

```
Layout
├── Navigation
└── main
    └── GraphPage
        ├── Controls (zoom, filters)
        └── ForceGraph (dynamic import, ssr: false)
            ├── Nodes (PRs)
            └── Edges (dependencies/conflicts)
```

### Key Components

#### `Layout.tsx`

Wrapper providing consistent page structure.

```typescript
interface LayoutProps {
  children: React.ReactNode;
  title?: string;
}
```

#### `Navigation.tsx`

Sidebar with active route highlighting.

**Routes:**
- `/` — Dashboard
- `/inbox` — Inbox
- `/triage` — Triage
- `/plan` — Plan
- `/graph` — Graph
- `/settings` — Settings

#### `TriageView.tsx`

Complex component with full state management.

**State:**
- Selected PRs
- Sort configuration
- Filter state
- Detail panel visibility

**Libraries:**
- `@tanstack/react-table` — Table logic
- `@tanstack/react-virtual` — Virtual scrolling

#### `SyncStatusPanel.tsx`

Polling component for sync progress.

**Behavior:**
- Polls `/api/repos/{owner}/{repo}/sync/status`
- Shows progress bar
- Displays last sync time
- Auto-refresh interval

---

## State Management

### Philosophy

**No global state.** All state is local to components using React hooks.

### Patterns

#### useState for Component State

```typescript
const [selectedPRs, setSelectedPRs] = useState<Set<number>>(new Set());
const [sortConfig, setSortConfig] = useState<SortConfig>({ key: 'score', dir: 'desc' });
const [isDetailOpen, setIsDetailOpen] = useState(false);
```

#### useMemo for Computed Values

```typescript
const sortedPRs = useMemo(() => {
  return [...prs].sort((a, b) => {
    if (sortConfig.dir === 'asc') {
      return a[sortConfig.key] - b[sortConfig.key];
    }
    return b[sortConfig.key] - a[sortConfig.key];
  });
}, [prs, sortConfig]);
```

#### useCallback for Event Handlers

```typescript
const handlePRSelect = useCallback((prNumber: number) => {
  setSelectedPRs(prev => {
    const next = new Set(prev);
    if (next.has(prNumber)) {
      next.delete(prNumber);
    } else {
      next.add(prNumber);
    }
    return next;
  });
}, []);
```

#### No Context

Do NOT use React Context for state management. Pass props down or lift state to common parent.

---

## Data Fetching Patterns

### getServerSideProps (All Pages)

Every page uses `getServerSideProps` for initial data load.

```typescript
// Example: pages/index.tsx
export async function getServerSideProps() {
  const apiUrl = process.env.NEXT_PUBLIC_PRATC_API_URL || 'http://localhost:8080';
  const defaultRepo = 'opencode-ai/opencode';
  
  const [analysis, cluster] = await Promise.all([
    fetchAnalysis(apiUrl, defaultRepo),
    fetchCluster(apiUrl, defaultRepo),
  ]);
  
  return {
    props: {
      analysis,
      cluster,
    },
  };
}
```

### Error Handling

API client returns `null` on error (never throws). Pages handle null gracefully.

```typescript
export async function getServerSideProps() {
  const analysis = await fetchAnalysis(apiUrl, repo);
  
  if (!analysis) {
    return {
      props: {
        error: 'Failed to load analysis',
      },
    };
  }
  
  return {
    props: { analysis },
  };
}
```

### Polling (Sync Status)

For live sync progress, use `useEffect` with interval.

```typescript
useEffect(() => {
  const interval = setInterval(async () => {
    const status = await fetchSyncStatus(apiUrl, repo);
    setSyncStatus(status);
  }, 5000);
  
  return () => clearInterval(interval);
}, [apiUrl, repo]);
```

---

## Styling System

### Single Global CSS

All styles in `styles/globals.css`. No CSS modules, no Tailwind, no styled-components.

### CSS Variables (Theming)

```css
:root {
  --color-sky-500: #0ea5e9;
  --color-amber-500: #f59e0b;
  --color-emerald-500: #10b981;
  --color-rose-500: #f43f5e;
  --color-violet-500: #8b5cf6;
  --color-slate-500: #64748b;
  
  --bg-primary: #ffffff;
  --bg-secondary: #f8fafc;
  --text-primary: #0f172a;
  --text-secondary: #475569;
  
  --border-radius: 8px;
  --spacing-unit: 8px;
}
```

### BEM-like Naming

```css
/* Block */
.stat-card { }

/* Element */
.stat-card__title { }
.stat-card__value { }

/* Modifier */
.stat-card--sky { border-left: 4px solid var(--color-sky-500); }
.stat-card--amber { border-left: 4px solid var(--color-amber-500); }
```

### Component Styles

Components receive modifier classes via props:

```typescript
interface StatCardProps {
  title: string;
  value: number;
  variant: 'sky' | 'amber' | 'emerald' | 'rose' | 'violet' | 'slate';
}

function StatCard({ title, value, variant }: StatCardProps) {
  return (
    <div className={`stat-card stat-card--${variant}`}>
      <div className="stat-card__title">{title}</div>
      <div className="stat-card__value">{value}</div>
    </div>
  );
}
```

---

## Testing Patterns

### Framework

- **vitest** — Test runner
- **@testing-library/react** — Component testing
- **vi.mock** — Module mocking

### Mock Factories

```typescript
// Create mock PR
export function createMockPR(overrides?: Partial<PR>): PR {
  return {
    id: '123',
    repo: 'owner/repo',
    number: 1,
    title: 'Test PR',
    body: 'Description',
    // ... other fields
    ...overrides,
  };
}

// Create mock analysis
export function createMockAnalysis(overrides?: Partial<AnalysisResponse>): AnalysisResponse {
  return {
    repo: 'owner/repo',
    generatedAt: new Date().toISOString(),
    counts: {
      total_prs: 10,
      cluster_count: 3,
      duplicate_groups: 1,
      overlap_groups: 2,
      conflict_pairs: 0,
      stale_prs: 1,
    },
    // ... other fields
    ...overrides,
  };
}
```

### Mocking Next.js Router

```typescript
vi.mock('next/router', () => ({
  useRouter: () => ({
    pathname: '/',
    query: {},
    push: vi.fn(),
  }),
}));
```

### Mocking Force Graph

```typescript
vi.mock('react-force-graph-2d', () => ({
  default: vi.fn(() => null),
}));
```

### Test Example

```typescript
import { render, screen } from '@testing-library/react';
import { describe, it, expect, vi } from 'vitest';
import Dashboard from '../pages/index';
import { createMockAnalysis } from './mocks';

describe('Dashboard', () => {
  it('renders stat cards', () => {
    const analysis = createMockAnalysis();
    render(<Dashboard analysis={analysis} cluster={null} />);
    
    expect(screen.getByText('Total PRs')).toBeInTheDocument();
    expect(screen.getByText('10')).toBeInTheDocument();
  });
  
  it('shows disconnected when API unavailable', () => {
    render(<Dashboard analysis={null} cluster={null} error="API unavailable" />);
    
    expect(screen.getByText('Disconnected')).toBeInTheDocument();
  });
});
```

---

## API Client

### Location

`src/lib/api.ts`

### Pattern

All functions return `null` on error (never throw).

```typescript
export async function fetchJSON<T>(
  url: string,
  fallback: T | null = null
): Promise<T | null> {
  try {
    const res = await fetch(url);
    if (!res.ok) return fallback;
    return await res.json();
  } catch {
    return fallback;
  }
}
```

### Available Functions

```typescript
// Analysis
fetchAnalysis(apiUrl: string, repo: string): Promise<AnalysisResponse | null>

// Clustering
fetchCluster(apiUrl: string, repo: string): Promise<ClusterResponse | null>

// Graph
fetchGraph(apiUrl: string, repo: string): Promise<GraphResponse | null>
fetchGraphDOT(apiUrl: string, repo: string): Promise<string | null>

// Planning
fetchPlan(
  apiUrl: string,
  repo: string,
  params?: PlanParams
): Promise<PlanResponse | null>

// Settings
fetchSettings(apiUrl: string, repo: string): Promise<Record<string, any> | null>
postSetting(apiUrl: string, scope: string, repo: string, key: string, value: any): Promise<boolean>
deleteSetting(apiUrl: string, scope: string, repo: string, key: string): Promise<boolean>
exportSettingsYAML(apiUrl: string, scope: string, repo: string): Promise<string | null>
importSettingsYAML(apiUrl: string, scope: string, repo: string, content: string): Promise<boolean>

// Sync
fetchSyncStatus(apiUrl: string, repo: string): Promise<SyncStatus | null>
triggerSync(apiUrl: string, repo: string): Promise<boolean>
```

### Type Consistency

All types in `src/types/api.ts` mirror Go/Python types with `snake_case` keys.

---

## Configuration

### Environment Variables

| Variable | Required | Default | Purpose |
|----------|----------|---------|---------|
| `NEXT_PUBLIC_PRATC_API_URL` | No | http://localhost:8080 | API base URL |

### Next.js Config

```typescript
// next.config.ts
const nextConfig = {
  // Disable SSR for force graph compatibility
  reactStrictMode: true,
  swcMinify: true,
};

export default nextConfig;
```

### Default Repository

Hardcoded in pages: `opencode-ai/opencode`

---

## Known Issues

### Settings API Mismatch

- **Web client expects:** `/api/repos/{owner}/{repo}/settings/{scope}/{key}`
- **Server provides:** `/api/settings?repo={owner/repo}`
- **Status:** Fix in progress

### No Loading States

- Renders "Disconnected" when API unavailable
- No skeletons, no spinners

### Force Graph SSR

Must use dynamic import with `ssr: false`:

```typescript
const ForceGraph = dynamic(() => import("react-force-graph-2d"), { ssr: false });
```

---

## Commands

```bash
# Development
bun install
bun run dev

# Production build
bun run build

# Testing
bun run test

# Type checking
bun run type-check
```
