// Package report provides PDF report generation for prATC scalability reports.
package report

import (
	"bytes"
	"fmt"
	"io"
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

// ScalabilityMetrics holds metrics data for the report.
type ScalabilityMetrics struct {
	TotalPRs        int
	OpenPRs         int
	AverageAgeDays  float64
	SyncThroughput  float64 // PRs per minute
	ConflictDensity float64 // conflicts per PR
	ClusterCount    int
	DuplicateCount  int
	StalePRCount    int
}

// MetricsSection renders the scalability metrics section.
type MetricsSection struct {
	Metrics ScalabilityMetrics
}

// Render draws the metrics section.
func (s *MetricsSection) Render(pdf *fpdf.Fpdf) {
	pdf.AddPage()
	pdf.SetFont("Arial", "B", 16)
	pdf.SetXY(15, 20)
	pdf.Cell(180, 10, "Scalability Metrics")

	s.renderTable(pdf)
}

// renderTable draws the metrics in a table layout.
func (s *MetricsSection) renderTable(pdf *fpdf.Fpdf) {
	// Header
	pdf.SetFillColor(230, 230, 230)
	pdf.SetFont("Arial", "B", 11)
	pdf.SetXY(15, 35)
	pdf.CellFormat(90, 10, "Metric", "1", 0, "L", true, 0, "")
	pdf.CellFormat(90, 10, "Value", "1", 1, "L", true, 0, "")

	// Rows
	pdf.SetFont("Arial", "", 11)
	rows := []struct {
		label string
		value string
	}{
		{"Total PRs", fmt.Sprintf("%d", s.Metrics.TotalPRs)},
		{"Open PRs", fmt.Sprintf("%d", s.Metrics.OpenPRs)},
		{"Average Age (days)", fmt.Sprintf("%.1f", s.Metrics.AverageAgeDays)},
		{"Sync Throughput (PRs/min)", fmt.Sprintf("%.2f", s.Metrics.SyncThroughput)},
		{"Conflict Density (conflicts/PR)", fmt.Sprintf("%.3f", s.Metrics.ConflictDensity)},
		{"Cluster Count", fmt.Sprintf("%d", s.Metrics.ClusterCount)},
		{"Duplicate Groups", fmt.Sprintf("%d", s.Metrics.DuplicateCount)},
		{"Stale PRs", fmt.Sprintf("%d", s.Metrics.StalePRCount)},
	}

	y := 45.0
	for _, row := range rows {
		pdf.SetXY(15, y)
		pdf.CellFormat(90, 8, row.label, "1", 0, "L", false, 0, "")
		pdf.CellFormat(90, 8, row.value, "1", 1, "L", false, 0, "")
		y += 8
	}
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

// SectionFromAnalysis creates a metrics section from analysis response.
func SectionFromAnalysis(metrics ScalabilityMetrics) *MetricsSection {
	return &MetricsSection{Metrics: metrics}
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
