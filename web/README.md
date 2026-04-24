# prATC Web

**Deprecated:** This directory is retained for historical reference only.

The Next.js web dashboard is not part of the active v2.0 product surface. The live dashboard/control surface is TUI-first. Active product surfaces are:

- **CLI** (`pratc analyze`, `pratc cluster`, `pratc graph`, `pratc plan`, v2.0 `pratc actions`)
- **HTTP API** (`pratc serve` on port 7400)
- **TUI Dashboard** (`pratc monitor`, v2.0 action lanes / queue / executor / audit stream)
- **PDF Snapshot Reports** (`pratc report`)

Historical scaffold notes:

Wave 1 provided a static Next.js shell with:

- a dashboard landing page
- a triage inbox placeholder
- shared layout and navigation
- local vitest coverage for the shell components

Commands (deprecated):

- `bun install`
- `bun run dev`
- `bun run build`
- `bun run test`
