import { test, expect } from "@playwright/test";

test.describe("prATC Web Dashboard E2E Tests", () => {
  test.beforeEach(async ({ page }) => {
    await page.goto("/");
  });

  test("should display the prATC branding in sidebar", async ({ page }) => {
    await expect(page.locator(".sidebar-brand p")).toHaveText("prATC");
    await expect(page.locator(".sidebar-brand span")).toHaveText("PR air traffic control");
  });

  test("should have navigation links for all routes", async ({ page }) => {
    const navLinks = page.locator(".sidebar ul li");
    await expect(navLinks).toHaveCount(5);

    await expect(page.getByRole("link", { name: "Dashboard" })).toHaveAttribute("href", "/");
    await expect(page.getByRole("link", { name: "Inbox" })).toHaveAttribute("href", "/inbox");
    await expect(page.getByRole("link", { name: "Graph" })).toHaveAttribute("href", "/graph");
    await expect(page.getByRole("link", { name: "Plan" })).toHaveAttribute("href", "/plan");
    await expect(page.getByRole("link", { name: "Monitor" })).toHaveAttribute("href", "/monitor");
  });

  test("should navigate to Dashboard page", async ({ page }) => {
    await expect(page.locator("h1")).toContainText("prATC Dashboard");
    await expect(page.locator(".page-header .hero-kicker")).toHaveText("Air Traffic Control");
  });

  test("should display disconnected state when API is unavailable", async ({ page }) => {
    await expect(page.getByText("Disconnected")).toBeVisible();
    await expect(page.getByText("No analysis payload available.")).toBeVisible();
  });

  test("should display SyncStatusPanel", async ({ page }) => {
    await expect(page.getByRole("button", { name: "Sync Now" })).toBeVisible();
  });
});

test.describe("Navigation E2E Tests", () => {
  test("should navigate to Inbox page", async ({ page }) => {
    await page.goto("/");
    await page.getByRole("link", { name: "Inbox" }).click();
    await expect(page).toHaveURL(/\/inbox/);
    await expect(page.locator("h1")).toContainText("Inbox");
  });

  test("should navigate to Graph page", async ({ page }) => {
    await page.goto("/");
    await page.getByRole("link", { name: "Graph" }).click();
    await expect(page).toHaveURL(/\/graph/);
    await expect(page.locator("h1")).toContainText("Dependency Graph");
  });

  test("should navigate to Plan page", async ({ page }) => {
    await page.goto("/");
    await page.getByRole("link", { name: "Plan" }).click();
    await expect(page).toHaveURL(/\/plan/);
    await expect(page.locator("h1")).toContainText("Merge Plan");
  });

  test("should navigate to Monitor page", async ({ page }) => {
    await page.goto("/");
    await page.getByRole("link", { name: "Monitor" }).click();
    await expect(page).toHaveURL(/\/monitor/);
    await expect(page.locator("h1")).toContainText("prATC Monitor");
    await expect(page.locator(".live-indicator")).toContainText("● Live");
  });

  test("should highlight active navigation item", async ({ page }) => {
    await page.goto("/");
    const dashboardLink = page.getByRole("link", { name: "Dashboard" });
    await expect(dashboardLink).toHaveClass(/nav-link--active/);

    await page.getByRole("link", { name: "Plan" }).click();
    await expect(page.getByRole("link", { name: "Plan" })).toHaveClass(/nav-link--active/);
    await expect(page.getByRole("link", { name: "Dashboard" })).not.toHaveClass(/nav-link--active/);
  });
});

test.describe("Inbox Page E2E Tests", () => {
  test("should display Inbox page with correct title", async ({ page }) => {
    await page.goto("/inbox");
    await expect(page.locator(".page-header .hero-kicker")).toHaveText("PR Triage");
  });

  test("should show disconnected state when no API", async ({ page }) => {
    await page.goto("/inbox");
    await expect(page.getByText("Live API is unavailable")).toBeVisible();
  });

  test("should display triage heading when API is unavailable", async ({ page }) => {
    await page.goto("/inbox");
    await expect(page.getByText("Start `pratc serve --port=8080` to load triage data.")).toBeVisible();
  });
});

test.describe("Triage Page E2E Tests", () => {
  test("should display Triage page with correct title", async ({ page }) => {
    await page.goto("/triage");
    await expect(page.locator(".page-header .hero-kicker")).toHaveText("Sequential Review");
    await expect(page.locator("h1")).toContainText("Triage Inbox");
  });

  test("should show disconnected state when API unavailable", async ({ page }) => {
    await page.goto("/triage");
    await expect(page.getByText("Live API is unavailable")).toBeVisible();
  });
});

test.describe("Graph Page E2E Tests", () => {
  test("should display Graph page with correct title", async ({ page }) => {
    await page.goto("/graph");
    await expect(page.locator(".page-header .hero-kicker")).toHaveText("Merge Dependencies");
    await expect(page.locator("h1")).toContainText("Dependency Graph");
  });

  test("should show disconnected state when API unavailable", async ({ page }) => {
    await page.goto("/graph");
    await expect(page.getByText("Disconnected")).toBeVisible();
    await expect(page.getByText("Unable to load graph payload.")).toBeVisible();
  });
});

test.describe("Plan Page E2E Tests", () => {
  test("should display Plan page with correct title", async ({ page }) => {
    await page.goto("/plan");
    await expect(page.locator(".page-header .hero-kicker")).toHaveText("Planner Output");
    await expect(page.locator("h1")).toContainText("Merge Plan");
  });

  test("should show omni-batch toggle", async ({ page }) => {
    await page.goto("/plan");
    await expect(page.getByText("Omni-Batch Mode")).toBeVisible();
    const toggleButton = page.locator("button", { hasText: "OFF" });
    await expect(toggleButton).toBeVisible();
  });

  test("should show disconnected state when API unavailable", async ({ page }) => {
    await page.goto("/plan");
    await expect(page.getByText("Disconnected")).toBeVisible();
    await expect(page.getByText("Unable to load merge plan payload.")).toBeVisible();
  });

  test("should toggle omni-batch mode", async ({ page }) => {
    await page.goto("/plan");
    const toggleButton = page.locator("button", { hasText: "OFF" });
    await toggleButton.click();
    await expect(page.locator("button", { hasText: "ON" })).toBeVisible();
    await expect(page.getByText("Omni-Batch Selector")).toBeVisible();
  });
});

test.describe("Settings Page E2E Tests", () => {
  test("should display Settings page with correct structure", async ({ page }) => {
    await page.goto("/settings");
    await expect(page.locator(".page-header .hero-kicker")).toHaveText("Configuration");
    await expect(page.locator("h1")).toContainText("Settings");
  });

  test("should display export and import buttons", async ({ page }) => {
    await page.goto("/settings");
    await expect(page.getByRole("button", { name: "Export YAML" })).toBeVisible();
    await expect(page.getByRole("button", { name: "Import YAML" })).toBeVisible();
  });

  test("should display all settings sections", async ({ page }) => {
    await page.goto("/settings");
    await expect(page.getByRole("heading", { name: "Formula Weights" })).toBeVisible();
    await expect(page.getByRole("heading", { name: "Thresholds" })).toBeVisible();
    await expect(page.getByRole("heading", { name: "Clustering" })).toBeVisible();
    await expect(page.getByRole("heading", { name: "Sync" })).toBeVisible();
    await expect(page.getByRole("heading", { name: "PR Window" })).toBeVisible();
    await expect(page.getByRole("heading", { name: "Priority Weights" })).toBeVisible();
    await expect(page.getByRole("heading", { name: "Pool Time Decay" })).toBeVisible();
    await expect(page.getByRole("heading", { name: "Priority Pool" })).toBeVisible();
    await expect(page.getByRole("heading", { name: "Legacy Time Decay" })).toBeVisible();
  });

  test("should have save and reset buttons per section", async ({ page }) => {
    await page.goto("/settings");
    const resetButtons = page.getByRole("button", { name: "Reset" });
    const saveButtons = page.getByRole("button", { name: "Save" });
    await expect(resetButtons.first()).toBeVisible();
    await expect(saveButtons.first()).toBeVisible();
  });
});

test.describe("Monitor Page E2E Tests", () => {
  test("should display Monitor page with correct title", async ({ page }) => {
    await page.goto("/monitor");
    await expect(page.locator(".page-header .hero-kicker")).toHaveText("Real-time Dashboard");
    await expect(page.locator("h1")).toContainText("prATC Monitor");
  });

  test("should display live indicator", async ({ page }) => {
    await page.goto("/monitor");
    await expect(page.locator(".live-indicator")).toContainText("Live");
  });

  test("should display monitor panels", async ({ page }) => {
    await page.goto("/monitor");
    await expect(page.locator(".monitor-grid")).toBeVisible();
    await expect(page.locator(".monitor-console")).toBeVisible();
  });
});