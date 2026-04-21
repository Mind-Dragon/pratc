# prATC Web

**Deprecated:** This directory is retained for historical reference only.

The Next.js web dashboard is not part of the v1.6 product surface. The active
product surfaces are:

- **CLI** (`pratc analyze`, `pratc cluster`, `pratc graph`, `pratc plan`)
- **HTTP API** (`pratc serve` on port 7400)
- **PDF Reports** (`pratc report`)
- **TUI Monitor** (`pratc monitor`)

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
