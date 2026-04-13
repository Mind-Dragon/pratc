// Package report provides PDF report generation for prATC scalability reports.
package report

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/go-pdf/fpdf"
)

// PDFComposer generates PDF reports with multiple sections.
type PDFComposer struct {
	repo     string
	title    string
	sections []PDFSection
}

// PDFSection is implemented by each report section.
type PDFSection interface {
	Render(pdf *fpdf.Fpdf)
}

// NewPDFComposer creates a new PDF composer for the given repository.
func NewPDFComposer(repo, title string) *PDFComposer {
	return &PDFComposer{
		repo:  repo,
		title: title,
	}
}

// AddSection adds a section to the report.
func (c *PDFComposer) AddSection(section PDFSection) {
	c.sections = append(c.sections, section)
}

// Compose generates the PDF and returns it as bytes.
func (c *PDFComposer) Compose() ([]byte, error) {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetAutoPageBreak(true, 15)

	for _, section := range c.sections {
		section.Render(pdf)
	}

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, fmt.Errorf("failed to output PDF: %w", err)
	}

	return buf.Bytes(), nil
}

// CoverSection renders the report cover page.
type CoverSection struct {
	Repo        string
	Title       string
	GeneratedAt time.Time
	Summary     string
}

// Render draws the cover page.
func (s *CoverSection) Render(pdf *fpdf.Fpdf) {
	pdf.AddPage()
	pdf.SetFillColor(26, 82, 118) // dark blue
	pdf.Rect(0, 0, 210, 60, "F")

	pdf.SetTextColor(255, 255, 255)
	pdf.SetFont("Arial", "B", 28)
	pdf.SetXY(15, 20)
	pdf.Cell(180, 15, "prATC Report")

	pdf.SetFont("Arial", "", 14)
	pdf.SetXY(15, 40)
	pdf.Cell(180, 10, s.Title)

	pdf.SetTextColor(0, 0, 0)
	pdf.SetFont("Arial", "", 12)
	pdf.SetXY(15, 70)
	pdf.Cell(180, 8, fmt.Sprintf("Repository: %s", s.Repo))
	pdf.SetXY(15, 80)
	pdf.Cell(180, 8, fmt.Sprintf("Generated: %s", s.GeneratedAt.Format(time.RFC1123)))

	pdf.SetFont("Arial", "B", 14)
	pdf.SetXY(15, 100)
	pdf.Cell(180, 8, "Executive Summary")

	pdf.SetFont("Arial", "", 11)
	pdf.SetXY(15, 112)
	pdf.MultiCell(180, 6, s.Summary, "", "L", false)
}

// MetricsDashboard holds all metrics from analyze, graph, and plan artifacts.
type MetricsDashboard struct {
	// From analyze.json
	TotalPRs       int
	ClusterCount   int
	DuplicateCount int
	OverlapCount   int
	ConflictCount  int
	StalePRCount   int

	// From graph.json
	GraphNodes int
	GraphEdges int

	// From plan.json
	SelectedCount  int
	RejectedCount  int
	TargetPRs      int
	CandidateCount int
}

// MetricsSection renders a visual dashboard of key metrics.
type MetricsSection struct {
	Dashboard   MetricsDashboard
	Repo        string
	GeneratedAt time.Time
}

// Render draws the metrics dashboard section.
func (s *MetricsSection) Render(pdf *fpdf.Fpdf) {
	pdf.AddPage()

	// Header with gradient-style background
	pdf.SetFillColor(26, 82, 118) // dark blue
	pdf.Rect(0, 0, 210, 35, "F")

	pdf.SetTextColor(255, 255, 255)
	pdf.SetFont("Arial", "B", 20)
	pdf.SetXY(15, 12)
	pdf.Cell(180, 12, "Metrics Dashboard")

	// Repository info below header
	pdf.SetTextColor(0, 0, 0)
	pdf.SetFont("Arial", "", 10)
	pdf.SetXY(15, 38)
	pdf.Cell(180, 6, fmt.Sprintf("Repository: %s | Generated: %s", s.Repo, s.GeneratedAt.Format(time.RFC1123)))

	// Main metrics grid (3 columns)
	s.renderMainMetrics(pdf)

	// Secondary metrics section
	s.renderSecondaryMetrics(pdf)

	// Plan summary box
	s.renderPlanSummary(pdf)
}

// renderMainMetrics draws the primary metrics in a 3-column grid.
func (s *MetricsSection) renderMainMetrics(pdf *fpdf.Fpdf) {
	y := 55.0

	// Header for main metrics
	pdf.SetFont("Arial", "B", 14)
	pdf.SetXY(15, y)
	pdf.Cell(180, 8, "Key Metrics")

	y += 12

	// Define box dimensions
	boxWidth := 58.0
	boxHeight := 25.0
	gap := 2.0
	startX := 15.0

	// Row 1: Total PRs, Clusters, Conflicts
	metrics1 := []struct {
		label string
		value int
		color struct{ r, g, b int }
	}{
		{"Total PRs", s.Dashboard.TotalPRs, struct{ r, g, b int }{52, 152, 219}},
		{"PR Clusters", s.Dashboard.ClusterCount, struct{ r, g, b int }{155, 89, 182}},
		{"Conflict Pairs", s.Dashboard.ConflictCount, struct{ r, g, b int }{231, 76, 60}},
	}

	for i, m := range metrics1 {
		x := startX + float64(i)*(boxWidth+gap)
		s.renderMetricBox(pdf, x, y, boxWidth, boxHeight, m.label, m.value, m.color.r, m.color.g, m.color.b)
	}

	y += boxHeight + 10

	// Row 2: Duplicates, Overlaps, Stale
	metrics2 := []struct {
		label string
		value int
		color struct{ r, g, b int }
	}{
		{"Duplicate Groups", s.Dashboard.DuplicateCount, struct{ r, g, b int }{241, 196, 15}},
		{"Overlap Groups", s.Dashboard.OverlapCount, struct{ r, g, b int }{230, 126, 34}},
		{"Stale PRs", s.Dashboard.StalePRCount, struct{ r, g, b int }{127, 140, 141}},
	}

	for i, m := range metrics2 {
		x := startX + float64(i)*(boxWidth+gap)
		s.renderMetricBox(pdf, x, y, boxWidth, boxHeight, m.label, m.value, m.color.r, m.color.g, m.color.b)
	}
}

// renderMetricBox draws a single metric box with colored background.
func (s *MetricsSection) renderMetricBox(pdf *fpdf.Fpdf, x, y, w, h float64, label string, value int, r, g, b int) {
	// Background
	pdf.SetFillColor(r, g, b)
	pdf.Rect(x, y, w, h, "F")

	// Label (top, smaller)
	pdf.SetTextColor(255, 255, 255)
	pdf.SetFont("Arial", "", 9)
	pdf.SetXY(x+3, y+2)
	pdf.CellFormat(w-6, 6, label, "", 0, "C", false, 0, "")

	// Value (center, large and bold)
	pdf.SetFont("Arial", "B", 18)
	pdf.SetXY(x+3, y+10)
	pdf.CellFormat(w-6, 12, fmt.Sprintf("%d", value), "", 0, "C", false, 0, "")
}

// renderSecondaryMetrics draws graph and structural metrics.
func (s *MetricsSection) renderSecondaryMetrics(pdf *fpdf.Fpdf) {
	y := 155.0

	// Section header
	pdf.SetFont("Arial", "B", 14)
	pdf.SetXY(15, y)
	pdf.Cell(180, 8, "Dependency Graph")

	y += 12

	// Graph metrics side-by-side
	pdf.SetFillColor(44, 62, 80)
	pdf.SetTextColor(255, 255, 255)

	// Nodes box
	pdf.Rect(15, y, 88, 30, "F")
	pdf.SetFont("Arial", "", 10)
	pdf.SetXY(18, y+3)
	pdf.Cell(82, 6, "Total Nodes")
	pdf.SetFont("Arial", "B", 20)
	pdf.SetXY(18, y+12)
	pdf.Cell(82, 14, fmt.Sprintf("%d", s.Dashboard.GraphNodes))

	// Edges box
	pdf.Rect(107, y, 88, 30, "F")
	pdf.SetXY(110, y+3)
	pdf.Cell(82, 6, "Total Edges")
	pdf.SetFont("Arial", "B", 20)
	pdf.SetXY(110, y+12)
	pdf.Cell(82, 14, fmt.Sprintf("%d", s.Dashboard.GraphEdges))

	// Edge-to-node ratio
	y += 35
	pdf.SetTextColor(0, 0, 0)
	pdf.SetFont("Arial", "", 10)
	pdf.SetXY(15, y)

	var edgeRatio float64
	if s.Dashboard.GraphNodes > 0 {
		edgeRatio = float64(s.Dashboard.GraphEdges) / float64(s.Dashboard.GraphNodes)
	}
	pdf.Cell(180, 6, fmt.Sprintf("Edge Density: %.2f edges per node", edgeRatio))
}

// renderPlanSummary draws the merge plan summary box.
func (s *MetricsSection) renderPlanSummary(pdf *fpdf.Fpdf) {
	y := 205.0

	// Section header
	pdf.SetFont("Arial", "B", 14)
	pdf.SetXY(15, y)
	pdf.Cell(180, 8, "Merge Plan Summary")

	y += 12

	// Background for plan summary
	pdf.SetFillColor(236, 240, 241)
	pdf.Rect(15, y, 180, 55, "F")
	pdf.SetDrawColor(52, 73, 94)
	pdf.Rect(15, y, 180, 55, "D")

	// Plan metrics in rows
	pdf.SetTextColor(0, 0, 0)

	// Row 1: Target vs Selected
	pdf.SetFont("Arial", "", 11)
	pdf.SetXY(20, y+8)
	pdf.CellFormat(60, 10, "Target PRs:", "0", 0, "R", false, 0, "")
	pdf.SetFont("Arial", "B", 14)
	pdf.CellFormat(40, 10, fmt.Sprintf("%d", s.Dashboard.TargetPRs), "0", 0, "L", false, 0, "")

	pdf.SetFont("Arial", "", 11)
	pdf.SetXY(100, y+8)
	pdf.CellFormat(60, 10, "Selected:", "0", 0, "R", false, 0, "")
	pdf.SetFont("Arial", "B", 14)
	pdf.SetTextColor(39, 174, 96) // green
	pdf.CellFormat(40, 10, fmt.Sprintf("%d", s.Dashboard.SelectedCount), "0", 0, "L", false, 0, "")

	// Row 2: Candidate pool
	y += 12
	pdf.SetTextColor(0, 0, 0)
	pdf.SetFont("Arial", "", 11)
	pdf.SetXY(20, y+8)
	pdf.CellFormat(60, 10, "Candidate Pool:", "0", 0, "R", false, 0, "")
	pdf.SetFont("Arial", "B", 14)
	pdf.CellFormat(40, 10, fmt.Sprintf("%d", s.Dashboard.CandidateCount), "0", 0, "L", false, 0, "")

	pdf.SetFont("Arial", "", 11)
	pdf.SetXY(100, y+8)
	pdf.CellFormat(60, 10, "Rejected:", "0", 0, "R", false, 0, "")
	pdf.SetFont("Arial", "B", 14)
	pdf.SetTextColor(231, 76, 60) // red
	pdf.CellFormat(40, 10, fmt.Sprintf("%d", s.Dashboard.RejectedCount), "0", 0, "L", false, 0, "")

	// Row 3: Selection rate
	y += 12
	pdf.SetTextColor(0, 0, 0)
	pdf.SetFont("Arial", "", 10)
	pdf.SetXY(20, y+8)

	var selectionRate float64
	if s.Dashboard.CandidateCount > 0 {
		selectionRate = float64(s.Dashboard.SelectedCount) / float64(s.Dashboard.CandidateCount) * 100
	}
	pdf.CellFormat(160, 10, fmt.Sprintf("Selection Rate: %.1f%% (%d of %d candidates)", selectionRate, s.Dashboard.SelectedCount, s.Dashboard.CandidateCount), "0", 0, "C", false, 0, "")
}

// PoolCompositionData holds merge plan data for the report.
type PoolCompositionData struct {
	Selected []CandidateRow
	Rejected []RejectedRow
	Clusters []ClusterRow
}

// CandidateRow represents a selected merge candidate.
type CandidateRow struct {
	PRNumber int
	Title    string
	Cluster  string
	Score    float64
}

// RejectedRow represents a rejected candidate.
type RejectedRow struct {
	PRNumber int
	Reason   string
}

// ClusterRow represents cluster distribution data.
type ClusterRow struct {
	ClusterID  string
	Label      string
	PRCount    int
	Percentage float64
}

// PoolCompositionSection renders the merge pool composition.
type PoolCompositionSection struct {
	Data PoolCompositionData
}

// Render draws the pool composition section.
func (s *PoolCompositionSection) Render(pdf *fpdf.Fpdf) {
	pdf.AddPage()
	pdf.SetFont("Arial", "B", 16)
	pdf.SetXY(15, 20)
	pdf.Cell(180, 10, "Pool Composition")

	if len(s.Data.Selected) > 0 {
		pdf.SetFont("Arial", "B", 13)
		pdf.SetXY(15, 35)
		pdf.Cell(180, 8, "Selected Merge Candidates")

		s.renderSelectedTable(pdf)
	}

	if len(s.Data.Rejected) > 0 {
		pdf.SetFont("Arial", "B", 13)
		pdf.SetXY(15, pdf.GetY()+10)
		pdf.Cell(180, 8, "Rejected Candidates")

		s.renderRejectedTable(pdf)
	}

	if len(s.Data.Clusters) > 0 {
		pdf.SetFont("Arial", "B", 13)
		pdf.SetXY(15, pdf.GetY()+10)
		pdf.Cell(180, 8, "Cluster Distribution")

		s.renderClusterTable(pdf)
	}
}

func (s *PoolCompositionSection) renderSelectedTable(pdf *fpdf.Fpdf) {
	// Header
	pdf.SetFillColor(200, 220, 200)
	pdf.SetFont("Arial", "B", 10)
	pdf.SetXY(15, pdf.GetY()+5)
	pdf.CellFormat(20, 8, "#", "1", 0, "C", true, 0, "")
	pdf.CellFormat(90, 8, "Title", "1", 0, "L", true, 0, "")
	pdf.CellFormat(30, 8, "Cluster", "1", 0, "L", true, 0, "")
	pdf.CellFormat(40, 8, "Score", "1", 1, "L", true, 0, "")

	pdf.SetFont("Arial", "", 10)
	for _, row := range s.Data.Selected {
		y := pdf.GetY()
		pdf.SetXY(15, y)
		pdf.CellFormat(20, 8, fmt.Sprintf("#%d", row.PRNumber), "1", 0, "C", false, 0, "")
		pdf.CellFormat(90, 8, truncate(row.Title, 45), "1", 0, "L", false, 0, "")
		pdf.CellFormat(30, 8, row.Cluster, "1", 0, "L", false, 0, "")
		pdf.CellFormat(40, 8, fmt.Sprintf("%.3f", row.Score), "1", 1, "L", false, 0, "")
	}
}

func (s *PoolCompositionSection) renderRejectedTable(pdf *fpdf.Fpdf) {
	// Header
	pdf.SetFillColor(220, 200, 200)
	pdf.SetFont("Arial", "B", 10)
	pdf.SetXY(15, pdf.GetY()+5)
	pdf.CellFormat(30, 8, "PR Number", "1", 0, "C", true, 0, "")
	pdf.CellFormat(150, 8, "Reason", "1", 1, "L", true, 0, "")

	pdf.SetFont("Arial", "", 10)
	for _, row := range s.Data.Rejected {
		pdf.SetXY(15, pdf.GetY())
		pdf.CellFormat(30, 8, fmt.Sprintf("#%d", row.PRNumber), "1", 0, "C", false, 0, "")
		pdf.CellFormat(150, 8, truncate(row.Reason, 70), "1", 1, "L", false, 0, "")
	}
}

func (s *PoolCompositionSection) renderClusterTable(pdf *fpdf.Fpdf) {
	// Header
	pdf.SetFillColor(200, 200, 220)
	pdf.SetFont("Arial", "B", 10)
	pdf.SetXY(15, pdf.GetY()+5)
	pdf.CellFormat(40, 8, "Cluster ID", "1", 0, "L", true, 0, "")
	pdf.CellFormat(50, 8, "Label", "1", 0, "L", true, 0, "")
	pdf.CellFormat(40, 8, "PR Count", "1", 0, "L", true, 0, "")
	pdf.CellFormat(50, 8, "Percentage", "1", 1, "L", true, 0, "")

	pdf.SetFont("Arial", "", 10)
	for _, row := range s.Data.Clusters {
		pdf.SetXY(15, pdf.GetY())
		pdf.CellFormat(40, 8, row.ClusterID, "1", 0, "L", false, 0, "")
		pdf.CellFormat(50, 8, truncate(row.Label, 25), "1", 0, "L", false, 0, "")
		pdf.CellFormat(40, 8, fmt.Sprintf("%d", row.PRCount), "1", 0, "L", false, 0, "")
		pdf.CellFormat(50, 8, fmt.Sprintf("%.1f%%", row.Percentage), "1", 1, "L", false, 0, "")
	}
}

// ChartsSection renders placeholder chart areas.
type ChartsSection struct {
	Charts []ChartPlaceholder
}

// ChartPlaceholder describes a chart placeholder.
type ChartPlaceholder struct {
	Title  string
	Width  float64
	Height float64
}

// Render draws placeholder rectangles for charts.
func (s *ChartsSection) Render(pdf *fpdf.Fpdf) {
	pdf.AddPage()
	pdf.SetFont("Arial", "B", 16)
	pdf.SetXY(15, 20)
	pdf.Cell(180, 10, "Charts")

	for i, chart := range s.Charts {
		y := 35.0 + float64(i)*50
		pdf.SetFont("Arial", "", 11)
		pdf.SetXY(15, y)
		pdf.Cell(180, 8, chart.Title)

		pdf.SetDrawColor(180, 180, 180)
		pdf.SetFillColor(245, 245, 245)
		pdf.Rect(15, y+10, chart.Width, chart.Height, "FD")

		pdf.SetFont("Arial", "I", 9)
		pdf.SetXY(15+chart.Width/2-20, y+10+chart.Height/2-4)
		pdf.SetTextColor(150, 150, 150)
		pdf.Cell(40, 8, "[Chart Placeholder]")
		pdf.SetTextColor(0, 0, 0)
	}
}

// RecommendationItem is a single recommendation.
type RecommendationItem struct {
	Priority string
	Text     string
}

// RecommendationsSection renders actionable insights.
type RecommendationsSection struct {
	Recommendations []RecommendationItem
}

// Render draws the recommendations section.
func (s *RecommendationsSection) Render(pdf *fpdf.Fpdf) {
	pdf.AddPage()
	pdf.SetFont("Arial", "B", 16)
	pdf.SetXY(15, 20)
	pdf.Cell(180, 10, "Recommendations")

	y := 35.0
	for _, rec := range s.Recommendations {
		// Priority badge
		pdf.SetFillColor(priorityColor(rec.Priority))
		pdf.SetTextColor(255, 255, 255)
		pdf.SetFont("Arial", "B", 9)
		pdf.SetXY(15, y)
		pdf.CellFormat(20, 7, rec.Priority, "1", 0, "C", true, 0, "")

		pdf.SetTextColor(0, 0, 0)
		pdf.SetFont("Arial", "", 10)
		pdf.SetXY(38, y)
		pdf.MultiCell(157, 7, rec.Text, "", "L", false)

		y += 14.0
	}
}

func priorityColor(priority string) (r, g, b int) {
	switch priority {
	case "HIGH":
		return 200, 50, 50
	case "MEDIUM":
		return 230, 180, 50
	default:
		return 50, 150, 50
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// PDFExporter provides utility methods for PDF export.
type PDFExporter struct {
	composer *PDFComposer
}

// NewPDFExporter creates a new PDF exporter.
func NewPDFExporter(repo, title string) *PDFExporter {
	return &PDFExporter{
		composer: NewPDFComposer(repo, title),
	}
}

// AddSection adds a section to the export.
func (e *PDFExporter) AddSection(section PDFSection) {
	e.composer.AddSection(section)
}

// Export generates and returns the PDF bytes.
func (e *PDFExporter) Export() ([]byte, error) {
	return e.composer.Compose()
}

// LoadMetricsSection loads metrics from all JSON artifact files.
func LoadMetricsSection(inputDir, repo string) (*MetricsSection, error) {
	section := &MetricsSection{
		Repo:        repo,
		GeneratedAt: time.Now(),
		Dashboard:   MetricsDashboard{},
	}

	// Load analyze.json
	analyzeData, err := readAnalyzeArtifact(inputDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read analyze file: %w", err)
	}

	var analyzeResult struct {
		Counts struct {
			TotalPRs        int `json:"total_prs"`
			ClusterCount    int `json:"cluster_count"`
			DuplicateGroups int `json:"duplicate_groups"`
			OverlapGroups   int `json:"overlap_groups"`
			ConflictPairs   int `json:"conflict_pairs"`
			StalePRs        int `json:"stale_prs"`
		} `json:"counts"`
		GeneratedAt string `json:"generatedAt"`
	}

	if err := json.Unmarshal(analyzeData, &analyzeResult); err != nil {
		return nil, fmt.Errorf("failed to parse analyze JSON: %w", err)
	}

	section.Dashboard.TotalPRs = analyzeResult.Counts.TotalPRs
	section.Dashboard.ClusterCount = analyzeResult.Counts.ClusterCount
	section.Dashboard.DuplicateCount = analyzeResult.Counts.DuplicateGroups
	section.Dashboard.OverlapCount = analyzeResult.Counts.OverlapGroups
	section.Dashboard.ConflictCount = analyzeResult.Counts.ConflictPairs
	section.Dashboard.StalePRCount = analyzeResult.Counts.StalePRs

	// Parse generatedAt timestamp
	if analyzeResult.GeneratedAt != "" {
		if t, err := time.Parse(time.RFC3339, analyzeResult.GeneratedAt); err == nil {
			section.GeneratedAt = t
		}
	}

	// Load graph.json
	graphPath := inputDir + "/step-4-graph.json"
	graphData, err := os.ReadFile(graphPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read graph file: %w", err)
	}

	var graphResult struct {
		Nodes []struct {
			PRNumber int `json:"pr_number"`
		} `json:"nodes"`
		Edges []struct {
			Source int `json:"source"`
			Target int `json:"target"`
		} `json:"edges"`
	}

	if err := json.Unmarshal(graphData, &graphResult); err != nil {
		return nil, fmt.Errorf("failed to parse graph JSON: %w", err)
	}

	section.Dashboard.GraphNodes = len(graphResult.Nodes)
	section.Dashboard.GraphEdges = len(graphResult.Edges)

	// Load plan.json
	planPath := inputDir + "/step-5-plan.json"
	planData, err := os.ReadFile(planPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read plan file: %w", err)
	}

	var planResult struct {
		Selected []struct {
			PRNumber int `json:"pr_number"`
		} `json:"selected"`
		Rejections []struct {
			PRNumber int `json:"pr_number"`
		} `json:"rejections"`
		Target            int `json:"target"`
		CandidatePoolSize int `json:"candidatePoolSize"`
	}

	if err := json.Unmarshal(planData, &planResult); err != nil {
		return nil, fmt.Errorf("failed to parse plan JSON: %w", err)
	}

	section.Dashboard.SelectedCount = len(planResult.Selected)
	section.Dashboard.RejectedCount = len(planResult.Rejections)
	section.Dashboard.TargetPRs = planResult.Target
	section.Dashboard.CandidateCount = planResult.CandidatePoolSize

	return section, nil
}

// SectionFromPlan creates a pool composition section from plan response.
func SectionFromPlan(selected []CandidateRow, rejected []RejectedRow, clusters []ClusterRow) *PoolCompositionSection {
	return &PoolCompositionSection{
		Data: PoolCompositionData{
			Selected: selected,
			Rejected: rejected,
			Clusters: clusters,
		},
	}
}

// Ensure io.Writer is satisfied by bytes.Buffer
var _ io.Writer = (*bytes.Buffer)(nil)
