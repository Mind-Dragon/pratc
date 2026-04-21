# prATC Dashboard User Guide

**Version:** v1.0  
**Last Updated:** 2026-04-09

---

## Table of Contents

1. [Overview](#overview)
2. [Quick Start](#quick-start)
3. [TUI Dashboard](#tui-dashboard)
   - [Three-Zone Layout](#three-zone-layout)
   - [Keyboard Shortcuts](#keyboard-shortcuts)
   - [Color Legend](#color-legend)
4. [Web Dashboard](#web-dashboard)
   - [Accessing the Dashboard](#accessing-the-dashboard)
   - [Panel Descriptions](#panel-descriptions)
5. [Troubleshooting](#troubleshooting)
6. [Quick Reference](#quick-reference)

---

## Overview

The prATC dashboard provides real-time monitoring of your pull request analysis and sync operations. It comes in two flavors:

- **TUI (Terminal User Interface):** A lightweight terminal-based dashboard that runs in your console. Perfect for server environments or quick checks.
- **Web Dashboard:** A browser-based interface with responsive design. Ideal for daily monitoring and team visibility.

Both dashboards show the same information: active sync jobs, timeline of activity, rate limit status, and system logs.

---

## Quick Start

### Starting the TUI Dashboard

```bash
# Start monitoring from the CLI
pratc monitor

# Or start the server and TUI together
pratc serve --port=7400 --monitor
```

### Starting the Web Dashboard

```bash
# Start the API server
pratc serve --port=7400

# In another terminal, start the web dashboard
cd web && bun run dev
```

Then open your browser to `http://localhost:3000/monitor`.

---

## TUI Dashboard

The TUI dashboard provides a terminal-based interface for monitoring prATC operations. It uses a dark cockpit design optimized for long viewing sessions.

### Three-Zone Layout

The TUI is organized into three main zones displayed side by side:

```
┌─────────────┬───────────────┬─────────────┐
│    JOBS     │    TIMELINE   │ RATE LIMIT  │
│   (30%)     │     (40%)     │   (30%)     │
├─────────────┼───────────────┼─────────────┤
│             │               │             │
│ Active sync │ Activity      │ API budget  │
│ jobs and    │ visualization │ status and  │
│ their       │ over time     │ reset time  │
│ statuses    │               │             │
│             │               │             │
└─────────────┴───────────────┴─────────────┘
┌─────────────────────────────────────────┐
│               CONSOLE                   │
│         (Full-width log output)         │
└─────────────────────────────────────────┘
```

#### Jobs Zone (Left Panel)

Shows all active sync jobs with their current status:

- **Job ID:** Unique identifier for each sync operation
- **Repository:** Which repo is being synced
- **Status:** Current state (see [Color Legend](#color-legend))
- **Progress:** Percentage complete for active jobs
- **Last Updated:** Timestamp of most recent activity

Use the up and down arrow keys to navigate between jobs. Press Enter to view detailed job information.

#### Timeline Zone (Center Panel)

Visualizes sync activity over time:

- **Time blocks:** Shows activity intensity in 15-minute intervals
- **Scrolling:** Use left and right arrows to scroll through time
- **Activity levels:** Higher blocks indicate more API calls or processing

This helps you identify patterns in your sync operations and spot unusual activity spikes.

#### Rate Limit Zone (Right Panel)

Displays your GitHub API budget status:

- **Remaining requests:** How many API calls you have left
- **Total budget:** Your hourly limit (typically 5,000 for GitHub)
- **Reset time:** When your quota refreshes
- **Status indicator:** Color-coded based on remaining budget

The panel updates every 30 seconds to keep you informed of your API usage.

#### Console Zone (Bottom Panel)

A full-width log viewer showing:

- **System events:** Startup, shutdown, configuration changes
- **Sync progress:** Detailed step-by-step sync operations
- **Errors and warnings:** Any issues that need attention
- **API calls:** Rate limit warnings and retry events

Use up and down arrows to scroll through log history. The console keeps the last 1,000 log entries.

### Keyboard Shortcuts

#### Global Shortcuts

These work from anywhere in the TUI:

| Key | Action |
|-----|--------|
| `Tab` | Switch between zones (Jobs → Timeline → Rate Limit → Console) |
| `?` | Toggle help overlay |
| `q` | Quit the dashboard |
| `Ctrl+C` | Quit (alternative) |
| `Esc` | Quit (alternative) |

#### Navigation Shortcuts

| Key | Zone | Action |
|-----|------|--------|
| `↑` | Jobs | Move to previous job |
| `↓` | Jobs | Move to next job |
| `↑` | Console | Scroll up in logs |
| `↓` | Console | Scroll down in logs |
| `←` | Timeline | Scroll to earlier time |
| `→` | Timeline | Scroll to later time |

#### Action Shortcuts

| Key | Action | When Available |
|-----|--------|----------------|
| `Enter` | View job details | When a job is selected in Jobs zone |
| `p` | Pause monitoring | Always (pauses all sync operations) |
| `r` | Resume monitoring | Only when paused |
| `s` | Restart sync | Always (starts a fresh sync cycle) |

#### Context-Sensitive Footer

The bottom of the screen shows available shortcuts based on your current zone:

- **Jobs zone:** `Tab: Switch | q: Quit | ?: Help | ↑↓: Navigate | Enter: Details`
- **Timeline zone:** `Tab: Switch | q: Quit | ?: Help | ←→: Scroll time`
- **Rate Limit zone:** `Tab: Switch | q: Quit | ?: Help | p: Pause | r: Resume`
- **Console zone:** `Tab: Switch | q: Quit | ?: Help | ↑↓: Scroll logs`

### Color Legend

The TUI uses a consistent color scheme across all panels:

#### Status Colors

| Color | Meaning | When You See It |
|-------|---------|-----------------|
| **Cyan** (#00D9FF) | Active/Primary | Jobs currently running, active operations |
| **Green** (#00FF9D) | Success/Completed | Finished jobs, healthy rate limits |
| **Amber** (#FFB946) | Warning/Paused | Paused jobs, rate limit getting low (500-2000 remaining) |
| **Red** (#FF4D4D) | Critical/Failed | Failed jobs, errors, rate limit critical (<500 remaining) |
| **Gray** (#8B92A0) | Queued/Inactive | Jobs waiting to start, muted text |

#### Rate Limit Colors

The rate limit panel changes color based on your remaining API budget:

- **Green:** More than 2,000 requests remaining (healthy)
- **Amber:** 500 to 2,000 requests remaining (caution)
- **Red:** Fewer than 500 requests remaining (critical, sync will pause)

#### Log Level Colors

Console logs are color-coded by severity:

- **Red:** Error messages
- **Amber:** Warning messages
- **Cyan:** Debug messages
- **White:** Info messages

---

## Web Dashboard

The web dashboard provides a browser-based interface with the same information as the TUI, plus additional visual polish and responsive design.

### Accessing the Dashboard

#### Development Mode

```bash
# Terminal 1: Start the API server
pratc serve --port=7400

# Terminal 2: Start the web dashboard
cd web
bun install  # First time only
bun run dev
```

Access at: `http://localhost:3000/monitor`

#### Production Mode

```bash
# Build the web dashboard
cd web && bun run build

# Start the server (serves web dashboard statically)
pratc serve --port=7400
```

Access at: `http://localhost:7400` (or your configured port)

### Panel Descriptions

The web dashboard mirrors the TUI layout with three panels and a console:

#### Jobs Panel

- Lists all sync jobs in a card-based layout
- Shows job status with color-coded badges
- Click any job to expand and see details
- Auto-refreshes every 10 seconds

#### Timeline Panel

- Interactive timeline visualization
- Hover over time blocks to see activity details
- Click and drag to scroll through time
- Shows 24 hours of activity by default

#### Rate Limit Panel

- Large, easy-to-read gauge showing remaining API calls
- Visual countdown to rate limit reset
- Warning indicators when budget is low
- Historical usage graph (last 6 hours)

#### Console Panel

- Expandable log viewer at the bottom
- Filter logs by level (Error, Warning, Info, Debug)
- Search functionality for finding specific events
- Auto-scrolls to show latest entries

### Responsive Design

The web dashboard adapts to your screen size:

- **Desktop (>1439px):** Three-column grid layout
- **Tablet (768px-1439px):** Two-column grid (Jobs + Timeline, Rate Limit below)
- **Mobile (<768px):** Single column, stacked vertically

---

## Troubleshooting

### Common Issues and Solutions

#### TUI Dashboard Won't Start

**Symptom:** Running `pratc monitor` shows an error or blank screen.

**Solutions:**
1. Check that your terminal supports ANSI colors: `echo $TERM` should show `xterm-256color` or similar
2. Try resizing your terminal to at least 100x30 characters
3. Ensure the API server is running: `curl http://localhost:7400/healthz`

#### Web Dashboard Shows "Disconnected"

**Symptom:** All panels show "Disconnected" or "Unable to fetch data".

**Solutions:**
1. Verify the API server is running: `pratc serve --port=7400`
2. Check the API URL in your browser console (should match your server port)
3. For Docker deployments, ensure ports are mapped correctly
4. Check browser console for CORS errors (if accessing from different origin)

#### Rate Limit Shows Red (Critical)

**Symptom:** Rate limit panel is red and shows fewer than 500 remaining requests.

**What happens:** Sync operations automatically pause when rate limit is critical.

**Solutions:**
1. Wait for the rate limit to reset (shown in the panel)
2. Press `r` (TUI) or click Resume (Web) to continue after reset
3. Consider reducing sync frequency in settings if this happens often

#### Jobs Stuck in "Paused" State

**Symptom:** Jobs show amber "paused" status and won't resume.

**Solutions:**
1. Press `r` in TUI or click Resume in Web dashboard
2. Check if rate limit is critical (red) and wait for reset
3. Check console logs for error messages preventing resume
4. Try pressing `s` to restart sync if resume fails

#### Console Shows Many Errors

**Symptom:** Console panel is flooded with red error messages.

**Solutions:**
1. Check your GitHub token is valid: `pratc config get github_token`
2. Verify repository access: ensure the token can access the configured repo
3. Check network connectivity: `curl https://api.github.com`
4. Look for specific error patterns in the logs

#### Timeline Shows No Activity

**Symptom:** Timeline panel is empty or shows flat line.

**Solutions:**
1. Ensure sync jobs have been started: press `s` to start a sync
2. Check that the repository has pull requests to analyze
3. Verify the timeline time range (use arrow keys to scroll)
4. Check Jobs panel to see if any jobs are active

#### Keyboard Shortcuts Not Working

**Symptom:** Pressing keys has no effect in TUI.

**Solutions:**
1. Make sure the TUI has focus (click on it if running in a window)
2. Check if you're in a specific zone that overrides shortcuts
3. Try `?` to see the help overlay with available shortcuts
4. Some terminal emulators intercept certain keys (try a different terminal)

### Getting More Help

If issues persist:

1. Check the [API Documentation](./api-contracts.md) for endpoint details
2. Review [Architecture Documentation](./architecture.md) for system design
3. Run with debug logging: `pratc serve --port=7400 --log-level=debug`
4. Check the audit log: `pratc audit --limit=50`

---

## Quick Reference

### CLI Commands for Dashboard

```bash
# Start TUI dashboard
pratc monitor

# Start server with monitoring
pratc serve --port=7400 --monitor

# Check system health
curl http://localhost:7400/healthz

# View recent audit entries
pratc audit --limit=20
```

### Default Ports

| Service | Default Port | Range |
|---------|--------------|-------|
| prATC API | 7400 | 7400-7500 |
| Web Dashboard | 3000 (dev) | - |
| Web Dashboard | 7400 (prod) | 7400-7500 |

### Keyboard Shortcuts Cheat Sheet

```
Global:
  Tab     - Switch zones
  ?       - Toggle help
  q       - Quit

Navigation:
  ↑/↓     - Navigate jobs / scroll logs
  ←/→     - Scroll timeline

Actions:
  Enter   - View job details
  p       - Pause
  r       - Resume (when paused)
  s       - Restart sync
```

### Color Quick Reference

```
Cyan    = Active/Running
Green   = Success/Completed
Amber   = Warning/Paused
Red     = Critical/Failed
Gray    = Queued/Waiting
```

---

## Related Documentation

- [API Contracts](./api-contracts.md) - Detailed API endpoint documentation
- [Architecture](./architecture.md) - System design and component overview
- [UI Wiring](./ui-wiring.md) - Web dashboard implementation details

---

*For support or feature requests, please refer to the project repository.*
