package report

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/go-pdf/fpdf"
)

// GraphSection renders the dependency/conflict graph with actual structure visualization.
type GraphSection struct {
	Repo        string
	GeneratedAt time.Time
	Nodes       []GraphNodeData
	Edges       []GraphEdgeData
}

// GraphNodeData represents a node in the graph for rendering.
type GraphNodeData struct {
	PRNumber  int
	Title     string
	CiStatus  string
	ClusterID string
}

// GraphEdgeData represents an edge in the graph for rendering.
type GraphEdgeData struct {
	FromPR   int
	ToPR     int
	EdgeType string // "depends_on" or "conflicts_with"
	Reason   string
}

const (
	edgeTypeConflict = "conflicts_with"
)

// Render draws the dependency graph section using fpdf primitives.
// It falls back to text-based rendering when graphviz dot is unavailable.
func (s *GraphSection) Render(pdf *fpdf.Fpdf) {
	pdf.AddPage()

	// Header
	pdf.SetFillColor(26, 82, 118) // dark blue
	pdf.Rect(0, 0, 210, 35, "F")

	pdf.SetTextColor(255, 255, 255)
	pdf.SetFont("Arial", "B", 18)
	pdf.SetXY(15, 12)
	pdf.Cell(180, 10, "Dependency Graph")

	// Repository info
	pdf.SetTextColor(0, 0, 0)
	pdf.SetFont("Arial", "", 10)
	pdf.SetXY(15, 40)
	pdf.Cell(180, 6, fmt.Sprintf("Repository: %s | Generated: %s", s.Repo, s.GeneratedAt.Format(time.RFC1123)))

	// Graph statistics
	pdf.SetFont("Arial", "B", 12)
	pdf.SetXY(15, 52)
	pdf.Cell(180, 8, "Graph Statistics")

	pdf.SetFont("Arial", "", 10)
	pdf.SetXY(15, 62)
	pdf.Cell(180, 6, fmt.Sprintf("Total Nodes: %d | Total Edges: %d", len(s.Nodes), len(s.Edges)))

	// Count edge types
	depCount := 0
	conflictCount := 0
	for _, e := range s.Edges {
		if e.EdgeType == edgeTypeConflict {
			conflictCount++
		} else {
			depCount++
		}
	}

	pdf.SetXY(15, 70)
	pdf.Cell(180, 6, fmt.Sprintf("Dependencies: %d | Conflicts: %d", depCount, conflictCount))

	// Render the actual graph visualization
	yPos := pdf.GetY() + 15

	// Check if we have a reasonable number of nodes to visualize
	if len(s.Nodes) > 50 {
		// Too many nodes - render a text-based summary with top connected nodes
		s.renderLargeGraphSummary(pdf, yPos)
	} else if len(s.Nodes) > 0 {
		// Render the graph using fpdf primitives
		s.renderGraphVisualization(pdf, yPos)
	} else {
		// No graph data
		pdf.SetFont("Arial", "I", 11)
		pdf.SetXY(15, yPos)
		pdf.Cell(180, 10, "No graph data available")
	}
}

// renderLargeGraphSummary renders a summary view for large graphs (>50 nodes).
func (s *GraphSection) renderLargeGraphSummary(pdf *fpdf.Fpdf, yPos float64) {
	pdf.SetFont("Arial", "B", 12)
	pdf.SetXY(15, yPos)
	pdf.Cell(180, 8, "Graph Overview (Top Connected Nodes)")

	// Calculate connectivity (how many edges each node has)
	connectivity := make(map[int]int)
	for _, e := range s.Edges {
		connectivity[e.FromPR]++
		connectivity[e.ToPR]++
	}

	// Sort nodes by connectivity
	type nodeWithConn struct {
		node  GraphNodeData
		conn  int
		prNum int
	}
	nodesWithConn := make([]nodeWithConn, 0, len(s.Nodes))
	for _, n := range s.Nodes {
		nodesWithConn = append(nodesWithConn, nodeWithConn{
			node:  n,
			conn:  connectivity[n.PRNumber],
			prNum: n.PRNumber,
		})
	}
	sort.Slice(nodesWithConn, func(i, j int) bool {
		if nodesWithConn[i].conn != nodesWithConn[j].conn {
			return nodesWithConn[i].conn > nodesWithConn[j].conn
		}
		return nodesWithConn[i].prNum < nodesWithConn[j].prNum
	})

	// Take top 20 most connected nodes
	topN := min(20, len(nodesWithConn))

	// Create a set of top PR numbers for edge filtering
	topPRs := make(map[int]bool)
	for i := 0; i < topN; i++ {
		topPRs[nodesWithConn[i].prNum] = true
	}

	// Render top nodes table
	pdf.SetFont("Arial", "B", 10)
	pdf.SetFillColor(52, 73, 94)
	pdf.SetTextColor(255, 255, 255)
	pdf.SetXY(15, yPos+12)
	pdf.CellFormat(25, 8, "PR #", "1", 0, "C", true, 0, "")
	pdf.CellFormat(15, 8, "Conn.", "1", 0, "C", true, 0, "")
	pdf.CellFormat(15, 8, "CI", "1", 0, "C", true, 0, "")
	pdf.CellFormat(125, 8, "Title (truncated)", "1", 1, "L", true, 0, "")

	pdf.SetTextColor(0, 0, 0)
	pdf.SetFont("Arial", "", 9)

	rowY := yPos + 20
	for i := 0; i < topN; i++ {
		n := nodesWithConn[i].node
		conn := nodesWithConn[i].conn

		// CI status color
		ciColor := map[string]struct{ r, g, b int }{
			"success": {39, 174, 96},
			"failure": {231, 76, 60},
			"pending": {230, 126, 34},
		}
		color := ciColor["pending"]
		if c, ok := ciColor[n.CiStatus]; ok {
			color = c
		}

		pdf.SetFillColor(color.r, color.g, color.b)
		pdf.SetXY(15, rowY)
		pdf.CellFormat(25, 7, fmt.Sprintf("#%d", n.PRNumber), "1", 0, "C", false, 0, "")
		pdf.CellFormat(15, 7, fmt.Sprintf("%d", conn), "1", 0, "C", false, 0, "")

		// CI status badge
		pdf.SetFillColor(color.r, color.g, color.b)
		pdf.CellFormat(15, 7, n.CiStatus, "1", 0, "C", true, 0, "")

		pdf.SetFillColor(236, 240, 241)
		title := truncate(n.Title, 60)
		pdf.CellFormat(125, 7, title, "1", 1, "L", false, 0, "")

		rowY += 7
		if rowY > 270 {
			break
		}
	}

	// Render edges among top nodes
	pdf.SetFont("Arial", "B", 11)
	edgeY := rowY + 10
	if edgeY < 200 {
		pdf.SetXY(15, edgeY)
		pdf.Cell(180, 8, fmt.Sprintf("Edges Among Top %d Nodes:", topN))
		edgeY += 10

		// Count edges between top nodes
		depEdges := 0
		confEdges := 0
		for _, e := range s.Edges {
			if topPRs[e.FromPR] && topPRs[e.ToPR] {
				if e.EdgeType == edgeTypeConflict {
					confEdges++
				} else {
					depEdges++
				}
			}
		}

		pdf.SetFont("Arial", "", 10)
		pdf.SetXY(15, edgeY)
		pdf.Cell(180, 6, fmt.Sprintf("Dependency edges: %d | Conflict edges: %d", depEdges, confEdges))
		edgeY += 10

		// List a sample of edges
		pdf.SetFont("Arial", "B", 10)
		pdf.SetXY(15, edgeY)
		pdf.Cell(180, 6, "Sample Edges:")

		pdf.SetFont("Arial", "", 8)
		edgeY += 8
		sampleCount := 0
		for _, e := range s.Edges {
			if topPRs[e.FromPR] && topPRs[e.ToPR] {
				edgeType := "→"
				color := "blue"
				if e.EdgeType == edgeTypeConflict {
					edgeType = "⇢"
					color = "red"
				}
				pdf.SetXY(15, edgeY)
				pdf.Cell(180, 5, fmt.Sprintf("  #%d %s #%d (%s) - %s", e.FromPR, edgeType, e.ToPR, color, truncate(e.Reason, 50)))
				edgeY += 6
				sampleCount++
				if sampleCount >= 10 || edgeY > 280 {
					break
				}
			}
		}
	}
}

// renderGraphVisualization renders a visual graph using fpdf primitives.
// This is used for smaller graphs (≤50 nodes).
func (s *GraphSection) renderGraphVisualization(pdf *fpdf.Fpdf, yPos float64) {
	// Calculate node positions in a simple grid layout
	nodeCount := len(s.Nodes)
	if nodeCount == 0 {
		return
	}

	// Build node index for position lookup
	nodeIndex := make(map[int]int)
	for i, n := range s.Nodes {
		nodeIndex[n.PRNumber] = i
	}

	// Simple grid layout
	cols := 5
	if nodeCount <= 10 {
		cols = 2
	} else if nodeCount <= 20 {
		cols = 3
	} else if nodeCount <= 30 {
		cols = 4
	}

	rows := (nodeCount + cols - 1) / cols
	nodeWidth := 35.0
	nodeHeight := 20.0
	startX := 15.0
	startY := yPos + 10
	gapX := 5.0
	gapY := 8.0

	// Store positions for edge drawing
	type nodePos struct {
		x, y float64
		pr   int
	}
	positions := make([]nodePos, nodeCount)

	for i, n := range s.Nodes {
		col := i % cols
		row := i / cols
		x := startX + float64(col)*(nodeWidth+gapX)
		y := startY + float64(row)*(nodeHeight+gapY)
		positions[i] = nodePos{x: x, y: y, pr: n.PRNumber}
	}

	// First pass: draw edges behind nodes
	// Color mapping
	depColor := struct{ r, g, b int }{52, 152, 219} // Blue for dependency
	confColor := struct{ r, g, b int }{231, 76, 60} // Red for conflict

	for _, e := range s.Edges {
		fromIdx, fromOk := nodeIndex[e.FromPR]
		toIdx, toOk := nodeIndex[e.ToPR]
		if !fromOk || !toOk {
			continue
		}

		from := positions[fromIdx]
		to := positions[toIdx]

		// Calculate edge endpoints (from center of nodes)
		fromX := from.x + nodeWidth/2
		fromY := from.y + nodeHeight/2
		toX := to.x + nodeWidth/2
		toY := to.y + nodeHeight/2

		if e.EdgeType == edgeTypeConflict {
			pdf.SetDrawColor(confColor.r, confColor.g, confColor.b)
			pdf.SetLineWidth(0.5)
			pdf.SetDashPattern([]float64{2, 2}, 0) // Dashed line for conflicts
		} else {
			pdf.SetDrawColor(depColor.r, depColor.g, depColor.b)
			pdf.SetLineWidth(0.3)
			pdf.SetDashPattern([]float64{}, 0) // Solid line for dependencies
		}

		pdf.Line(fromX, fromY, toX, toY)
	}

	// Reset dash pattern
	pdf.SetDashPattern([]float64{}, 0)

	// Second pass: draw nodes on top
	for i, n := range s.Nodes {
		pos := positions[i]
		x := pos.x
		y := pos.y

		// Determine node fill color based on CI status
		var fillColor struct{ r, g, b int }
		switch n.CiStatus {
		case "success":
			fillColor = struct{ r, g, b int }{39, 174, 96} // Green
		case "failure":
			fillColor = struct{ r, g, b int }{231, 76, 60} // Red
		default:
			fillColor = struct{ r, g, b int }{230, 126, 34} // Orange
		}

		// Draw node background
		pdf.SetFillColor(fillColor.r, fillColor.g, fillColor.b)
		pdf.SetDrawColor(52, 73, 94)
		pdf.SetLineWidth(0.5)
		pdf.Rect(x, y, nodeWidth, nodeHeight, "FD")

		// Draw PR number and truncated title
		pdf.SetTextColor(255, 255, 255)
		pdf.SetFont("Arial", "B", 7)
		pdf.SetXY(x+2, y+2)
		pdf.Cell(nodeWidth-4, 5, fmt.Sprintf("#%d", n.PRNumber))

		pdf.SetFont("Arial", "", 5)
		pdf.SetXY(x+2, y+8)
		title := truncate(n.Title, 20)
		pdf.Cell(nodeWidth-4, 5, title)

		pdf.SetTextColor(0, 0, 0)
	}

	// Legend
	legendY := startY + float64(rows)*(nodeHeight+gapY) + 10
	if legendY < 260 {
		pdf.SetFont("Arial", "B", 10)
		pdf.SetXY(15, legendY)
		pdf.Cell(180, 6, "Legend:")

		legendY += 8

		// Dependency legend
		pdf.SetDrawColor(depColor.r, depColor.g, depColor.b)
		pdf.SetLineWidth(0.5)
		pdf.Line(15, legendY+2, 35, legendY+2)
		pdf.SetFont("Arial", "", 9)
		pdf.SetXY(40, legendY)
		pdf.Cell(60, 6, "Dependency (solid)")

		// Conflict legend
		pdf.SetDrawColor(confColor.r, confColor.g, confColor.b)
		pdf.SetDashPattern([]float64{2, 2}, 0)
		pdf.Line(100, legendY+2, 120, legendY+2)
		pdf.SetDashPattern([]float64{}, 0)
		pdf.SetXY(125, legendY)
		pdf.Cell(60, 6, "Conflict (dashed)")
	}
}

// LoadGraphSection loads graph data from step-4-graph.json and returns a GraphSection.
func LoadGraphSection(inputDir, repo string) (*GraphSection, error) {
	section := &GraphSection{
		Repo:        repo,
		GeneratedAt: time.Now(),
	}

	graphPath := inputDir + "/step-4-graph.json"
	graphData, err := os.ReadFile(graphPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read graph file: %w", err)
	}

	var graphResult struct {
		Repo        string `json:"repo"`
		GeneratedAt string `json:"generatedAt"`
		Nodes       []struct {
			PRNumber  int    `json:"pr_number"`
			Title     string `json:"title"`
			ClusterID string `json:"cluster_id"`
			CiStatus  string `json:"ci_status"`
		} `json:"nodes"`
		Edges []struct {
			FromPR   int    `json:"from_pr"`
			ToPR     int    `json:"to_pr"`
			EdgeType string `json:"edge_type"`
			Reason   string `json:"reason"`
		} `json:"edges"`
	}

	if err := json.Unmarshal(graphData, &graphResult); err != nil {
		return nil, fmt.Errorf("failed to parse graph JSON: %w", err)
	}

	// Parse generatedAt timestamp
	if graphResult.GeneratedAt != "" {
		if t, err := time.Parse(time.RFC3339, graphResult.GeneratedAt); err == nil {
			section.GeneratedAt = t
		}
	}

	// Map nodes
	for _, n := range graphResult.Nodes {
		section.Nodes = append(section.Nodes, GraphNodeData{
			PRNumber:  n.PRNumber,
			Title:     n.Title,
			CiStatus:  n.CiStatus,
			ClusterID: n.ClusterID,
		})
	}

	// Map edges
	for _, e := range graphResult.Edges {
		section.Edges = append(section.Edges, GraphEdgeData{
			FromPR:   e.FromPR,
			ToPR:     e.ToPR,
			EdgeType: e.EdgeType,
			Reason:   e.Reason,
		})
	}

	return section, nil
}

// Ensure GraphSection implements PDFSection interface
var _ PDFSection = (*GraphSection)(nil)
