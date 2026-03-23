import React, { useMemo, useRef, useCallback, useEffect, useState } from "react";
import type { GetServerSideProps } from "next";
import dynamic from "next/dynamic";

import Layout from "../components/Layout";
import { fetchGraph } from "../lib/api";
import type { GraphResponse, GraphNode, GraphEdge } from "../types/api";

const DEFAULT_REPO = "opencode-ai/opencode";

type GraphProps = {
  graph: GraphResponse | null;
};

type ForceGraphNode = {
  id: string;
  pr_number: number;
  title: string;
  cluster_id: string;
  ci_status: string;
};

type ForceGraphLink = {
  source: string;
  target: string;
  edge_type: string;
  reason: string;
};

type ForceGraphData = {
  nodes: ForceGraphNode[];
  links: ForceGraphLink[];
};

const ForceGraph2D = dynamic(
  () => import("react-force-graph-2d").then((mod) => mod.default),
  {
    ssr: false,
    loading: () => (
      <div className="graph-loading">
        <p>Loading graph visualization...</p>
      </div>
    ),
  }
);

function transformGraphData(graph: GraphResponse | null): ForceGraphData | null {
  if (!graph || !graph.nodes || !graph.edges) {
    return null;
  }

  if (graph.nodes.length === 0) {
    return { nodes: [], links: [] };
  }

  const nodes: ForceGraphNode[] = graph.nodes.map((n: GraphNode) => ({
    id: String(n.pr_number),
    pr_number: n.pr_number,
    title: n.title,
    cluster_id: n.cluster_id,
    ci_status: n.ci_status,
  }));

  const links: ForceGraphLink[] = graph.edges.map((e: GraphEdge) => ({
    source: String(e.from_pr),
    target: String(e.to_pr),
    edge_type: e.edge_type,
    reason: e.reason,
  }));

  return { nodes, links };
}

function getNodeColor(ciStatus: string): string {
  switch (ciStatus.toLowerCase()) {
    case "success":
    case "passing":
      return "#2c7a62";
    case "failure":
    case "failing":
    case "failed":
      return "#b44a3f";
    case "pending":
    case "running":
      return "#b9d7f2";
    default:
      return "#e7d3ab";
  }
}

function getEdgeColor(edgeType: string): string {
  switch (edgeType.toLowerCase()) {
    case "conflict":
      return "rgba(180, 74, 63, 0.6)";
    case "dependency":
    case "depends_on":
      return "rgba(44, 122, 98, 0.6)";
    case "overlap":
      return "rgba(231, 211, 171, 0.8)";
    default:
      return "rgba(23, 34, 53, 0.3)";
  }
}

export const getServerSideProps: GetServerSideProps<GraphProps> = async (context) => {
  const rawRepo = context.query.repo;
  const repo = typeof rawRepo === "string" && rawRepo.length > 0 ? rawRepo : DEFAULT_REPO;
  const graph = await fetchGraph(repo);
  return {
    props: {
      graph,
    },
  };
}

type NodeDetailPanelProps = {
  node: ForceGraphNode | null;
  onClose: () => void;
};

function NodeDetailPanel({ node, onClose }: NodeDetailPanelProps) {
  if (!node) return null;

  return (
    <div className="graph-detail-panel">
      <div className="graph-detail-header">
        <h4>PR #{node.pr_number}</h4>
        <button className="action-btn" onClick={onClose} aria-label="Close detail panel">
          ×
        </button>
      </div>
      <dl>
        <dt>Title</dt>
        <dd>{node.title}</dd>
        <dt>Cluster</dt>
        <dd>{node.cluster_id || "Unclustered"}</dd>
        <dt>CI Status</dt>
        <dd>
          <span className={`pill pill--${node.ci_status.toLowerCase() === "success" || node.ci_status.toLowerCase() === "passing" ? "passing" : node.ci_status.toLowerCase() === "failure" || node.ci_status.toLowerCase() === "failing" ? "failing" : ""}`}>
            {node.ci_status}
          </span>
        </dd>
      </dl>
    </div>
  );
}

function GraphLegend() {
  return (
    <div className="graph-legend">
      <div className="graph-legend-item">
        <span className="graph-legend-dot" style={{ backgroundColor: "#2c7a62" }}></span>
        <span>CI Passing</span>
      </div>
      <div className="graph-legend-item">
        <span className="graph-legend-dot" style={{ backgroundColor: "#b44a3f" }}></span>
        <span>CI Failing</span>
      </div>
      <div className="graph-legend-item">
        <span className="graph-legend-dot" style={{ backgroundColor: "#b9d7f2" }}></span>
        <span>Pending</span>
      </div>
      <div className="graph-legend-item">
        <span className="graph-legend-dot" style={{ backgroundColor: "#e7d3ab" }}></span>
        <span>Unknown</span>
      </div>
      <div className="graph-legend-divider"></div>
      <div className="graph-legend-item">
        <span className="graph-legend-line" style={{ backgroundColor: "rgba(44, 122, 98, 0.6)" }}></span>
        <span>Dependency</span>
      </div>
      <div className="graph-legend-item">
        <span className="graph-legend-line" style={{ backgroundColor: "rgba(180, 74, 63, 0.6)" }}></span>
        <span>Conflict</span>
      </div>
    </div>
  );
}

type GraphCanvasProps = {
  data: ForceGraphData;
  onNodeClick: (node: ForceGraphNode | null) => void;
};

function GraphCanvas({ data, onNodeClick }: GraphCanvasProps) {
  const graphRef = useRef<any>(null);

  const handleNodeClick = useCallback((node: any) => {
    onNodeClick(node as ForceGraphNode);
  }, [onNodeClick]);

  useEffect(() => {
    if (graphRef.current && data.nodes.length > 0) {
      const timer = setTimeout(() => {
        graphRef.current?.zoomToFit(400, 50);
      }, 500);
      return () => clearTimeout(timer);
    }
  }, [data.nodes.length]);

  const paintNode = useCallback((node: any, ctx: CanvasRenderingContext2D, globalScale: number) => {
    const size = 6;
    const fontSize = Math.min(10, 10 / globalScale);

    ctx.beginPath();
    ctx.arc(node.x, node.y, size, 0, 2 * Math.PI);
    ctx.fillStyle = getNodeColor(node.ci_status);
    ctx.fill();
    ctx.strokeStyle = "rgba(23, 34, 53, 0.3)";
    ctx.lineWidth = 1;
    ctx.stroke();

    if (globalScale > 0.8) {
      ctx.font = `${fontSize}px "Avenir Next", "Segoe UI", sans-serif`;
      ctx.fillStyle = "rgba(23, 34, 53, 0.9)";
      ctx.textAlign = "center";
      ctx.textBaseline = "top";
      ctx.fillText(`#${node.pr_number}`, node.x, node.y + size + 2);
    }
  }, []);

  const paintLink = useCallback((link: any, ctx: CanvasRenderingContext2D) => {
    ctx.beginPath();
    ctx.moveTo(link.source.x, link.source.y);
    ctx.lineTo(link.target.x, link.target.y);
    ctx.strokeStyle = getEdgeColor(link.edge_type);
    ctx.lineWidth = 1.5;
    ctx.stroke();
  }, []);

  return (
    <ForceGraph2D
      ref={graphRef}
      graphData={data}
      nodeRelSize={6}
      linkDirectionalParticles={2}
      linkDirectionalParticleSpeed={0.005}
      onNodeClick={handleNodeClick}
      nodeCanvasObject={paintNode}
      linkCanvasObject={paintLink}
      linkCanvasObjectMode={() => "replace"}
      cooldownTicks={100}
      d3AlphaDecay={0.02}
      d3VelocityDecay={0.3}
      width={800}
      height={500}
    />
  );
}

export default function GraphPage({ graph }: GraphProps) {
  const [selectedNode, setSelectedNode] = useState<ForceGraphNode | null>(null);
  const graphData = useMemo(() => transformGraphData(graph), [graph]);

  const handleNodeClick = useCallback((node: ForceGraphNode | null) => {
    setSelectedNode(node);
  }, []);

  if (!graph) {
    return (
      <Layout
        title="Dependency Graph"
        eyebrow="Merge Dependencies"
        description="Graph edges from the live API (dependency and conflict relationships)."
      >
        <section className="hero-panel">
          <div>
            <p className="hero-kicker">API status</p>
            <h2>Disconnected</h2>
            <p>Unable to load graph payload.</p>
          </div>
        </section>
      </Layout>
    );
  }

  if (!graphData) {
    return (
      <Layout
        title="Dependency Graph"
        eyebrow="Merge Dependencies"
        description="Graph edges from the live API (dependency and conflict relationships)."
      >
        <section className="hero-panel">
          <div>
            <p className="hero-kicker">Data error</p>
            <h2>Invalid Graph Data</h2>
            <p>The graph payload is malformed or unreadable.</p>
          </div>
        </section>
      </Layout>
    );
  }

  if (graphData.nodes.length === 0) {
    return (
      <Layout
        title="Dependency Graph"
        eyebrow="Merge Dependencies"
        description="Graph edges from the live API (dependency and conflict relationships)."
      >
        <section className="cluster-section">
          <div className="section-heading">
            <div>
              <p className="hero-kicker">Graph summary</p>
              <h3>{graph.repo}</h3>
            </div>
            <p>No PR nodes available</p>
          </div>
          <div className="hero-panel">
            <div>
              <p className="hero-kicker">Empty graph</p>
              <h2>No Data to Display</h2>
              <p>There are no pull requests with dependencies or conflicts to visualize.</p>
            </div>
          </div>
        </section>
      </Layout>
    );
  }

  return (
    <Layout
      title="Dependency Graph"
      eyebrow="Merge Dependencies"
      description="Interactive visualization of PR dependencies and conflicts."
    >
      <section className="cluster-section">
        <div className="section-heading">
          <div>
            <p className="hero-kicker">Graph summary</p>
            <h3>{graph.repo}</h3>
          </div>
          <p>
            {graphData.nodes.length} nodes / {graphData.links.length} edges
          </p>
        </div>

        <div className="graph-container">
          <GraphLegend />
          <div className="graph-canvas-wrapper">
            <GraphCanvas data={graphData} onNodeClick={handleNodeClick} />
          </div>
          <NodeDetailPanel node={selectedNode} onClose={() => setSelectedNode(null)} />
        </div>
      </section>
    </Layout>
  );
}
