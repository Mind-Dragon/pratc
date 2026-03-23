import React from "react";
import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

afterEach(() => {
  cleanup();
});

vi.mock("next/router", () => ({
  useRouter: () => ({
    pathname: "/graph"
  })
}));

vi.mock("next/link", () => ({
  default: ({ href, className, children }: { href: string; className?: string; children: React.ReactNode }) => (
    <a className={className} href={href}>
      {children}
    </a>
  )
}));

vi.mock("../../lib/api", () => ({
  fetchGraph: vi.fn()
}));

vi.mock("react-force-graph-2d", () => ({
  __esModule: true,
  default: vi.fn(() => <div data-testid="force-graph-mock">ForceGraph2D Mock</div>)
}));

import GraphPage from "../../pages/graph";

const mockGraphData = {
  repo: "test/repo",
  generatedAt: "2024-01-01T00:00:00Z",
  nodes: [
    { pr_number: 1, title: "PR One", cluster_id: "cluster-a", ci_status: "success" },
    { pr_number: 2, title: "PR Two", cluster_id: "cluster-a", ci_status: "failure" },
    { pr_number: 3, title: "PR Three", cluster_id: "cluster-b", ci_status: "pending" },
  ],
  edges: [
    { from_pr: 1, to_pr: 2, edge_type: "conflict", reason: "Same file touched" },
    { from_pr: 2, to_pr: 3, edge_type: "dependency", reason: "Depends on changes" },
  ],
  dot: "digraph { 1 -> 2; 2 -> 3; }"
};

describe("GraphPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders disconnected panel when graph is null", () => {
    render(<GraphPage graph={null} />);

    expect(screen.getByText("Disconnected")).toBeTruthy();
    expect(screen.getByText("Unable to load graph payload.")).toBeTruthy();
  });

  it("renders invalid data panel for malformed graph (missing nodes)", () => {
    const malformedGraph = {
      repo: "test/repo",
      generatedAt: "2024-01-01T00:00:00Z",
      nodes: null as unknown as typeof mockGraphData.nodes,
      edges: [],
      dot: ""
    };

    render(<GraphPage graph={malformedGraph} />);

    expect(screen.getByText("Invalid Graph Data")).toBeTruthy();
    expect(screen.getByText("The graph payload is malformed or unreadable.")).toBeTruthy();
  });

  it("renders invalid data panel for malformed graph (missing edges)", () => {
    const malformedGraph = {
      repo: "test/repo",
      generatedAt: "2024-01-01T00:00:00Z",
      nodes: [],
      edges: null as unknown as typeof mockGraphData.edges,
      dot: ""
    };

    render(<GraphPage graph={malformedGraph} />);

    expect(screen.getByText("Invalid Graph Data")).toBeTruthy();
  });

  it("renders empty graph panel when nodes array is empty", () => {
    const emptyGraph = {
      repo: "test/repo",
      generatedAt: "2024-01-01T00:00:00Z",
      nodes: [],
      edges: [],
      dot: "digraph {}"
    };

    render(<GraphPage graph={emptyGraph} />);

    expect(screen.getByText("No Data to Display")).toBeTruthy();
    expect(screen.getByText("There are no pull requests with dependencies or conflicts to visualize.")).toBeTruthy();
  });

  it("renders interactive graph with valid data", () => {
    render(<GraphPage graph={mockGraphData} />);

    expect(screen.getByRole("heading", { name: "test/repo" })).toBeTruthy();
    expect(screen.getByText("3 nodes / 2 edges")).toBeTruthy();
    expect(screen.getByText("CI Passing")).toBeTruthy();
    expect(screen.getByText("CI Failing")).toBeTruthy();
    expect(screen.getByText("Pending")).toBeTruthy();
    expect(screen.getByText("Dependency")).toBeTruthy();
    expect(screen.getByText("Conflict")).toBeTruthy();
  });

  it("renders graph legend with all legend items", () => {
    render(<GraphPage graph={mockGraphData} />);

    expect(screen.getByText("CI Passing")).toBeTruthy();
    expect(screen.getByText("CI Failing")).toBeTruthy();
    expect(screen.getByText("Pending")).toBeTruthy();
    expect(screen.getByText("Unknown")).toBeTruthy();
    expect(screen.getByText("Dependency")).toBeTruthy();
    expect(screen.getByText("Conflict")).toBeTruthy();
  });

  it("renders layout with correct title and description", () => {
    render(<GraphPage graph={mockGraphData} />);

    expect(screen.getByRole("heading", { name: "Dependency Graph" })).toBeTruthy();
    expect(screen.getByText("Interactive visualization of PR dependencies and conflicts.")).toBeTruthy();
  });

  it("displays node count and edge count in summary", () => {
    render(<GraphPage graph={mockGraphData} />);

    expect(screen.getByText("3 nodes / 2 edges")).toBeTruthy();
  });

  it("handles graph with only nodes and no edges", () => {
    const noEdgesGraph = {
      ...mockGraphData,
      edges: []
    };

    render(<GraphPage graph={noEdgesGraph} />);

    expect(screen.getByRole("heading", { name: "test/repo" })).toBeTruthy();
    expect(screen.getByText("3 nodes / 0 edges")).toBeTruthy();
  });

  it("renders loading state for force graph dynamically", async () => {
    const { container } = render(<GraphPage graph={mockGraphData} />);
    
    const graphContainer = container.querySelector(".graph-container");
    expect(graphContainer).toBeTruthy();
  });
});
