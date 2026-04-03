package report

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/go-pdf/fpdf"
)

// SummarySection renders the executive summary with key metrics.
type SummarySection struct {
	Repo        string
	GeneratedAt time.Time
	TotalPRs    int
	Clusters    int
	Conflicts   int
	SelectedPRs int
	Target      int
}

// Render draws the executive summary section.
func (s *SummarySection) Render(pdf *fpdf.Fpdf) {
	pdf.AddPage()

	// Header
	pdf.SetFillColor(26, 82, 118) // dark blue
	pdf.Rect(0, 0, 210, 40, "F")

	pdf.SetTextColor(255, 255, 255)
	pdf.SetFont("Arial", "B", 18)
	pdf.SetXY(15, 15)
	pdf.Cell(180, 10, "Executive Summary")

	// Repository and metadata
	pdf.SetTextColor(0, 0, 0)
	pdf.SetFont("Arial", "", 11)
	pdf.SetXY(15, 50)
	pdf.Cell(180, 6, fmt.Sprintf("Repository: %s", s.Repo))
	pdf.SetXY(15, 58)
	pdf.Cell(180, 6, fmt.Sprintf("Generated: %s", s.GeneratedAt.Format(time.RFC1123)))
	pdf.SetXY(15, 66)
	pdf.Cell(180, 6, fmt.Sprintf("Target PRs: %d", s.Target))

	// Key metrics section
	pdf.SetFont("Arial", "B", 14)
	pdf.SetXY(15, 85)
	pdf.Cell(180, 8, "Analysis Overview")

	// Metrics grid
	pdf.SetFont("Arial", "", 11)
	y := 100.0

	// Row 1: Total PRs and Clusters
	pdf.SetXY(15, y)
	pdf.CellFormat(90, 12, "Total PRs Analyzed", "1", 0, "L", false, 0, "")
	pdf.CellFormat(90, 12, fmt.Sprintf("%d", s.TotalPRs), "1", 1, "C", false, 0, "")

	y += 12
	pdf.SetXY(15, y)
	pdf.CellFormat(90, 12, "PR Clusters", "1", 0, "L", false, 0, "")
	pdf.CellFormat(90, 12, fmt.Sprintf("%d", s.Clusters), "1", 1, "C", false, 0, "")

	y += 12
	pdf.SetXY(15, y)
	pdf.CellFormat(90, 12, "Conflict Pairs", "1", 0, "L", false, 0, "")
	pdf.CellFormat(90, 12, fmt.Sprintf("%d", s.Conflicts), "1", 1, "C", false, 0, "")

	y += 12
	pdf.SetXY(15, y)
	pdf.CellFormat(90, 12, "Selected for Merge", "1", 0, "L", false, 0, "")
	pdf.CellFormat(90, 12, fmt.Sprintf("%d", s.SelectedPRs), "1", 1, "C", false, 0, "")

	// High-level summary text
	pdf.SetFont("Arial", "B", 12)
	pdf.SetXY(15, y+20)
	pdf.Cell(180, 7, "Merge Plan Summary")

	pdf.SetFont("Arial", "", 10)
	summaryText := fmt.Sprintf(
		"Analyzed %d pull requests across %d clusters. "+
			"Identified %d potential conflict pairs. "+
			"Selected %d PRs for merge based on target of %d.",
		s.TotalPRs, s.Clusters, s.Conflicts, s.SelectedPRs, s.Target,
	)

	pdf.SetXY(15, y+32)
	pdf.MultiCell(180, 6, summaryText, "", "L", false)
}

// LoadSummarySection loads data from JSON files and returns a SummarySection.
func LoadSummarySection(inputDir, repo string) (*SummarySection, error) {
	section := &SummarySection{
		Repo:        repo,
		GeneratedAt: time.Now(),
	}

	// Load analyze data (step-2-analyze.json)
	analyzePath := inputDir + "/step-2-analyze.json"
	analyzeData, err := os.ReadFile(analyzePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read analyze file: %w", err)
	}

	var analyzeResult struct {
		Counts struct {
			TotalPRs      int `json:"total_prs"`
			ClusterCount  int `json:"cluster_count"`
			ConflictPairs int `json:"conflict_pairs"`
		} `json:"counts"`
		GeneratedAt string `json:"generatedAt"`
	}

	if err := json.Unmarshal(analyzeData, &analyzeResult); err != nil {
		return nil, fmt.Errorf("failed to parse analyze JSON: %w", err)
	}

	section.TotalPRs = analyzeResult.Counts.TotalPRs
	section.Clusters = analyzeResult.Counts.ClusterCount
	section.Conflicts = analyzeResult.Counts.ConflictPairs

	// Parse generatedAt timestamp
	if analyzeResult.GeneratedAt != "" {
		if t, err := time.Parse(time.RFC3339, analyzeResult.GeneratedAt); err == nil {
			section.GeneratedAt = t
		}
	}

	// Load plan data (step-5-plan.json)
	planPath := inputDir + "/step-5-plan.json"
	planData, err := os.ReadFile(planPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read plan file: %w", err)
	}

	var planResult struct {
		Selected []struct {
			PRNumber int `json:"pr_number"`
		} `json:"selected"`
		Target int `json:"target"`
	}

	if err := json.Unmarshal(planData, &planResult); err != nil {
		return nil, fmt.Errorf("failed to parse plan JSON: %w", err)
	}

	section.SelectedPRs = len(planResult.Selected)
	section.Target = planResult.Target

	return section, nil
}

// Ensure SummarySection implements PDFSection interface
var _ PDFSection = (*SummarySection)(nil)
