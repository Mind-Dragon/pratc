package report

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/go-pdf/fpdf"
	"github.com/jeffersonnunn/pratc/internal/types"
)

// PlanSection renders the merge plan with selected PRs and rejections.
type PlanSection struct {
	Repo              string
	GeneratedAt       time.Time
	Target            int
	CandidatePoolSize int
	Strategy          string
	PoolSizeBefore    int
	PoolSizeAfter     int
	CollapsedGroups   int
	TotalSuperseded   int
	Selected          []SelectedPRData
	Ordering          []OrderedPRData
	Rejections        []RejectionData
}

// SelectedPRData represents a selected PR for rendering.
type SelectedPRData struct {
	Number    int
	Title     string
	Score     float64
	Rationale string
}

// OrderedPRData represents a PR in merge order.
type OrderedPRData struct {
	Number    int
	Title     string
	Score     float64
	Order     int
	Rationale string
}

// RejectionData represents a rejected PR.
type RejectionData struct {
	Number int
	Title  string
	Reason string
}

func (s *PlanSection) Render(pdf *fpdf.Fpdf) {
	pdf.AddPage()

	pdf.SetFillColor(26, 82, 118)
	pdf.Rect(0, 0, 210, 35, "F")

	pdf.SetTextColor(255, 255, 255)
	pdf.SetFont("Arial", "B", 18)
	pdf.SetXY(15, 12)
	pdf.Cell(180, 10, "Merge Plan")

	pdf.SetTextColor(0, 0, 0)
	pdf.SetFont("Arial", "", 10)
	pdf.SetXY(15, 40)
	pdf.Cell(180, 6, fmt.Sprintf("Repository: %s | Generated: %s", s.Repo, s.GeneratedAt.Format(time.RFC1123)))

	pdf.SetFont("Arial", "B", 12)
	pdf.SetXY(15, 52)
	pdf.Cell(180, 8, "Plan Summary")

	pdf.SetFont("Arial", "", 10)
	pdf.SetXY(15, 62)
	pdf.Cell(180, 6, fmt.Sprintf("Target: %d PRs | Candidate Pool: %d | Strategy: %s", s.Target, s.CandidatePoolSize, s.Strategy))
	pdf.SetXY(15, 70)
	pdf.Cell(180, 6, fmt.Sprintf("Selected: %d PRs | Rejected: %d PRs", len(s.Selected), len(s.Rejections)))

	yPos := pdf.GetY() + 12
	pdf.SetFont("Arial", "B", 12)
	pdf.SetXY(15, yPos)
	pdf.Cell(180, 8, "Expanded Plan Surface")
	yPos += 10
	pdf.SetFont("Arial", "", 10)
	for _, line := range s.expansionSummaryLines() {
		pdf.SetXY(15, yPos)
		pdf.MultiCell(180, 6, line, "", "L", false)
		yPos = pdf.GetY()
	}

	yPos += 9

	pdf.SetFont("Arial", "B", 12)
	pdf.SetXY(15, yPos)
	pdf.Cell(180, 8, "Selected PRs (Merge Order)")

	pdf.SetFont("Arial", "B", 10)
	pdf.SetFillColor(52, 73, 94)
	pdf.SetTextColor(255, 255, 255)
	pdf.SetXY(15, yPos+10)
	pdf.CellFormat(15, 8, "Order", "1", 0, "C", true, 0, "")
	pdf.CellFormat(25, 8, "PR #", "1", 0, "C", true, 0, "")
	pdf.CellFormat(100, 8, "Title", "1", 0, "L", true, 0, "")
	pdf.CellFormat(30, 8, "Score", "1", 0, "R", true, 0, "")
	pdf.CellFormat(20, 8, "Status", "1", 1, "C", true, 0, "")

	pdf.SetTextColor(0, 0, 0)
	pdf.SetFont("Arial", "", 9)

	rowY := yPos + 18
	maxRows := 20
	rowsRendered := 0

	for _, pr := range s.Ordering {
		if rowsRendered >= maxRows {
			break
		}

		pdf.SetFillColor(236, 240, 241)
		pdf.SetXY(15, rowY)

		pdf.CellFormat(15, 7, fmt.Sprintf("%d", pr.Order), "1", 0, "C", false, 0, "")
		pdf.CellFormat(25, 7, fmt.Sprintf("#%d", pr.Number), "1", 0, "C", false, 0, "")

		title := truncate(pr.Title, 50)
		pdf.CellFormat(100, 7, title, "1", 0, "L", false, 0, "")

		pdf.CellFormat(30, 7, fmt.Sprintf("%.4f", pr.Score), "1", 0, "R", false, 0, "")

		status := "Ready"
		if pr.Rationale != "" {
			if containsIgnoreCase(pr.Rationale, "CI passing") {
				status = "CI OK"
			} else if containsIgnoreCase(pr.Rationale, "mergeable") {
				status = "OK"
			}
		}
		pdf.CellFormat(20, 7, status, "1", 1, "C", false, 0, "")

		rowY += 7
		rowsRendered++

		if rowY > 270 && rowsRendered < len(s.Ordering) {
			pdf.SetFont("Arial", "I", 9)
			pdf.SetXY(15, rowY-5)
			pdf.Cell(180, 6, fmt.Sprintf("(Showing %d of %d selected PRs - continued)", rowsRendered, len(s.Ordering)))

			pdf.AddPage()
			rowY = 20

			pdf.SetFont("Arial", "B", 10)
			pdf.SetFillColor(52, 73, 94)
			pdf.SetTextColor(255, 255, 255)
			pdf.SetXY(15, rowY)
			pdf.CellFormat(15, 8, "Order", "1", 0, "C", true, 0, "")
			pdf.CellFormat(25, 8, "PR #", "1", 0, "C", true, 0, "")
			pdf.CellFormat(100, 8, "Title", "1", 0, "L", true, 0, "")
			pdf.CellFormat(30, 8, "Score", "1", 0, "R", true, 0, "")
			pdf.CellFormat(20, 8, "Status", "1", 1, "C", true, 0, "")

			pdf.SetTextColor(0, 0, 0)
			pdf.SetFont("Arial", "", 9)
			rowY += 8
		}
	}

	if len(s.Ordering) > maxRows {
		pdf.SetFont("Arial", "I", 9)
		pdf.SetXY(15, rowY+5)
		pdf.Cell(180, 6, fmt.Sprintf("Note: Showing first %d of %d selected PRs. Full data in step-5-plan.json", maxRows, len(s.Ordering)))
	}

	if len(s.Rejections) > 0 {
		rejectionStartY := rowY + 15

		if rejectionStartY > 200 {
			pdf.AddPage()
			rejectionStartY = 20
		}

		pdf.SetFont("Arial", "B", 12)
		pdf.SetXY(15, rejectionStartY)
		pdf.Cell(180, 8, fmt.Sprintf("Rejected PRs (%d)", len(s.Rejections)))

		pdf.SetFont("Arial", "B", 10)
		pdf.SetFillColor(192, 57, 43)
		pdf.SetTextColor(255, 255, 255)
		pdf.SetXY(15, rejectionStartY+10)
		pdf.CellFormat(25, 8, "PR #", "1", 0, "C", true, 0, "")
		pdf.CellFormat(120, 8, "Title", "1", 0, "L", true, 0, "")
		pdf.CellFormat(55, 8, "Reason", "1", 1, "L", true, 0, "")

		pdf.SetTextColor(0, 0, 0)
		pdf.SetFont("Arial", "", 9)

		rejY := rejectionStartY + 18
		maxRejRows := 15
		rejRowsRendered := 0

		for _, rej := range s.Rejections {
			if rejRowsRendered >= maxRejRows {
				break
			}

			pdf.SetFillColor(250, 230, 230)
			pdf.SetXY(15, rejY)

			pdf.CellFormat(25, 7, fmt.Sprintf("#%d", rej.Number), "1", 0, "C", false, 0, "")

			title := truncate(rej.Title, 60)
			pdf.CellFormat(120, 7, title, "1", 0, "L", false, 0, "")

			reason := truncate(rej.Reason, 25)
			pdf.CellFormat(55, 7, reason, "1", 1, "L", false, 0, "")

			rejY += 7
			rejRowsRendered++

			if rejY > 270 && rejRowsRendered < len(s.Rejections) {
				pdf.SetFont("Arial", "I", 9)
				pdf.SetXY(15, rejY-5)
				pdf.Cell(180, 6, fmt.Sprintf("(Showing %d of %d rejections - continued)", rejRowsRendered, len(s.Rejections)))

				pdf.AddPage()
				rejY = 20

				pdf.SetFont("Arial", "B", 10)
				pdf.SetFillColor(192, 57, 43)
				pdf.SetTextColor(255, 255, 255)
				pdf.SetXY(15, rejY)
				pdf.CellFormat(25, 8, "PR #", "1", 0, "C", true, 0, "")
				pdf.CellFormat(120, 8, "Title", "1", 0, "L", true, 0, "")
				pdf.CellFormat(55, 8, "Reason", "1", 1, "L", true, 0, "")

				pdf.SetTextColor(0, 0, 0)
				pdf.SetFont("Arial", "", 9)
				rejY += 8
			}
		}

		if len(s.Rejections) > maxRejRows {
			pdf.SetFont("Arial", "I", 9)
			pdf.SetXY(15, rejY+5)
			pdf.Cell(180, 6, fmt.Sprintf("Note: Showing first %d of %d rejections. Full data in step-5-plan.json", maxRejRows, len(s.Rejections)))
		}
	}
}

func (s *PlanSection) expansionSummaryLines() []string {
	lines := []string{}
	if s.PoolSizeBefore > 0 || s.PoolSizeAfter > 0 {
		if s.PoolSizeBefore > 0 && s.PoolSizeAfter > 0 {
			lines = append(lines, fmt.Sprintf("• viable pool moved from %d to %d after planning filters", s.PoolSizeBefore, s.PoolSizeAfter))
		} else {
			lines = append(lines, fmt.Sprintf("• viable pool after filters: %d", maxInt(s.PoolSizeBefore, s.PoolSizeAfter)))
		}
	}
	if s.CollapsedGroups > 0 || s.TotalSuperseded > 0 {
		lines = append(lines, fmt.Sprintf("• duplicate collapse contributed %d canonical groups and removed %d superseded PRs from the working set", s.CollapsedGroups, s.TotalSuperseded))
	}
	if s.CandidatePoolSize > 0 {
		lines = append(lines, fmt.Sprintf("• final candidate pool was %d PRs; dynamic target resolved to %d", s.CandidatePoolSize, s.Target))
	}
	if len(lines) == 0 {
		lines = append(lines, fmt.Sprintf("• final candidate pool was %d PRs; target resolved to %d", s.CandidatePoolSize, s.Target))
	}
	return lines
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// containsIgnoreCase checks if a string contains a substring (case-insensitive).
func containsIgnoreCase(s, substr string) bool {
	if len(s) < len(substr) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if equalIgnoreCase(s[i:i+len(substr)], substr) {
			return true
		}
	}
	return false
}

func equalIgnoreCase(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		c1 := a[i]
		c2 := b[i]
		if c1 >= 'A' && c1 <= 'Z' {
			c1 += 'a' - 'A'
		}
		if c2 >= 'A' && c2 <= 'Z' {
			c2 += 'a' - 'A'
		}
		if c1 != c2 {
			return false
		}
	}
	return true
}

// LoadPlanSection loads plan data from step-5-plan.json and returns a PlanSection.
func LoadPlanSection(inputDir, repo string) (*PlanSection, error) {
	section := &PlanSection{
		Repo:        repo,
		GeneratedAt: time.Now(),
	}

	planPath := inputDir + "/step-5-plan.json"
	planData, err := os.ReadFile(planPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read plan file: %w", err)
	}

	var planResult types.PlanResponse
	if err := json.Unmarshal(planData, &planResult); err != nil {
		return nil, fmt.Errorf("failed to parse plan JSON: %w", err)
	}

	if planResult.GeneratedAt != "" {
		if t, err := time.Parse(time.RFC3339, planResult.GeneratedAt); err == nil {
			section.GeneratedAt = t
		}
	}

	section.Target = planResult.Target
	section.CandidatePoolSize = planResult.CandidatePoolSize
	section.Strategy = planResult.Strategy
	if planResult.Telemetry != nil {
		section.PoolSizeBefore = planResult.Telemetry.PoolSizeBefore
		section.PoolSizeAfter = planResult.Telemetry.PoolSizeAfter
	}
	if planResult.CollapsedCorpus != nil {
		section.CollapsedGroups = planResult.CollapsedCorpus.CollapsedGroupCount
		section.TotalSuperseded = planResult.CollapsedCorpus.TotalSuperseded
	}

	for _, pr := range planResult.Selected {
		section.Selected = append(section.Selected, SelectedPRData{
			Number:    pr.PRNumber,
			Title:     pr.Title,
			Score:     pr.Score,
			Rationale: pr.Rationale,
		})
	}

	for i, pr := range planResult.Ordering {
		section.Ordering = append(section.Ordering, OrderedPRData{
			Number:    pr.PRNumber,
			Title:     pr.Title,
			Score:     pr.Score,
			Order:     i + 1,
			Rationale: pr.Rationale,
		})
	}

	prTitles := make(map[int]string)
	for _, pr := range planResult.Selected {
		prTitles[pr.PRNumber] = pr.Title
	}
	for _, pr := range planResult.Ordering {
		prTitles[pr.PRNumber] = pr.Title
	}

	for _, rej := range planResult.Rejections {
		title := prTitles[rej.PRNumber]
		if title == "" {
			title = fmt.Sprintf("PR #%d", rej.PRNumber)
		}
		section.Rejections = append(section.Rejections, RejectionData{
			Number: rej.PRNumber,
			Title:  title,
			Reason: rej.Reason,
		})
	}

	sort.Slice(section.Rejections, func(i, j int) bool {
		return section.Rejections[i].Number < section.Rejections[j].Number
	})

	return section, nil
}

// Ensure PlanSection implements PDFSection interface
var _ PDFSection = (*PlanSection)(nil)
