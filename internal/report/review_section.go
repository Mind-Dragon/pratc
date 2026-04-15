package report

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-pdf/fpdf"
)

// ReviewDashboard summarizes the review payload for PDF rendering.
type ReviewDashboard struct {
	TotalPRs         int
	ReviewedPRs      int
	MergeNow         int
	FocusedReview    int
	Duplicate        int
	Problematic      int
	Escalate         int
	SecurityRisk     int
	ReliabilityRisk  int
	PerformanceRisk  int
	FastMerge        int
	ReviewRequired   int
	Blocked          int
}

// ReviewSection renders review buckets and priority tiers.
type ReviewSection struct {
	Repo        string
	GeneratedAt time.Time
	Dashboard   ReviewDashboard
}

// Render draws the review section.
func (s *ReviewSection) Render(pdf *fpdf.Fpdf) {
	pdf.AddPage()

	pdf.SetFillColor(26, 82, 118)
	pdf.Rect(0, 0, 210, 35, "F")

	pdf.SetTextColor(255, 255, 255)
	pdf.SetFont("Arial", "B", 20)
	pdf.SetXY(15, 12)
	pdf.Cell(180, 12, "Review Buckets")

	pdf.SetTextColor(0, 0, 0)
	pdf.SetFont("Arial", "", 10)
	pdf.SetXY(15, 38)
	pdf.Cell(180, 6, fmt.Sprintf("Repository: %s | Generated: %s", s.Repo, s.GeneratedAt.Format(time.RFC1123)))

	pdf.SetFont("Arial", "B", 14)
	pdf.SetXY(15, 50)
	pdf.Cell(180, 8, "Coverage and bucket counts")

	coverageText := fmt.Sprintf("Review coverage: %d of %d PRs", s.Dashboard.ReviewedPRs, s.Dashboard.TotalPRs)
	pdf.SetFont("Arial", "", 10)
	pdf.SetXY(15, 60)
	pdf.Cell(180, 6, coverageText)

	s.renderBucketRows(pdf)
	s.renderRiskRow(pdf)
	s.renderPriorityRow(pdf)
}

func (s *ReviewSection) renderBucketRows(pdf *fpdf.Fpdf) {
	topY := 72.0
	boxWidth := 58.0
	boxHeight := 25.0
	gap := 2.0
	startX := 15.0

	bucketCards := []struct {
		label   string
		value   int
		r, g, b int
	}{
		{"now", s.Dashboard.MergeNow, 52, 152, 219},
		{"future", s.Dashboard.FocusedReview, 46, 204, 113},
		{"duplicate", s.Dashboard.Duplicate, 241, 196, 15},
		{"junk", s.Dashboard.Problematic, 231, 76, 60},
		{"blocked", s.Dashboard.Escalate, 142, 68, 173},
	}

	for i, card := range bucketCards {
		row := 0
		col := i
		if i >= 3 {
			row = 1
			col = i - 3
		}
		x := startX + float64(col)*(boxWidth+gap)
		y := topY + float64(row)*(boxHeight+10)
		renderReviewMetricBox(pdf, x, y, boxWidth, boxHeight, card.label, card.value, card.r, card.g, card.b)
	}
}

func (s *ReviewSection) renderRiskRow(pdf *fpdf.Fpdf) {
	boxWidth := 58.0
	boxHeight := 25.0
	gap := 2.0
	startX := 15.0
	y := 135.0

	pdf.SetFont("Arial", "B", 14)
	pdf.SetXY(15, y-12)
	pdf.Cell(180, 8, "Risk buckets")

	riskCards := []struct {
		label   string
		value   int
		r, g, b int
	}{
		{"security_risk", s.Dashboard.SecurityRisk, 155, 89, 182},
		{"reliability_risk", s.Dashboard.ReliabilityRisk, 230, 126, 34},
		{"performance_risk", s.Dashboard.PerformanceRisk, 231, 76, 60},
	}

	for i, card := range riskCards {
		x := startX + float64(i)*(boxWidth+gap)
		renderReviewMetricBox(pdf, x, y, boxWidth, boxHeight, card.label, card.value, card.r, card.g, card.b)
	}
}

func (s *ReviewSection) renderPriorityRow(pdf *fpdf.Fpdf) {
	boxWidth := 58.0
	boxHeight := 25.0
	gap := 2.0
	startX := 15.0
	y := 180.0

	pdf.SetFont("Arial", "B", 14)
	pdf.SetXY(15, y-12)
	pdf.Cell(180, 8, "Operational priority tiers")

	priorityCards := []struct {
		label   string
		value   int
		r, g, b int
	}{
		{"fast_merge", s.Dashboard.FastMerge, 46, 204, 113},
		{"review_required", s.Dashboard.ReviewRequired, 52, 152, 219},
		{"blocked", s.Dashboard.Blocked, 231, 76, 60},
	}

	for i, card := range priorityCards {
		x := startX + float64(i)*(boxWidth+gap)
		renderReviewMetricBox(pdf, x, y, boxWidth, boxHeight, card.label, card.value, card.r, card.g, card.b)
	}

	pdf.SetFont("Arial", "", 10)
	pdf.SetXY(15, y+35)
	pdf.MultiCell(180, 6, "The review engine remains advisory-only: it ranks work, keeps operational tiers internal, and exposes the v1.4 decision map through the analyst/report layers without mutating GitHub state.", "", "L", false)
}

func renderReviewMetricBox(pdf *fpdf.Fpdf, x, y, w, h float64, label string, value int, r, g, b int) {
	pdf.SetFillColor(r, g, b)
	pdf.Rect(x, y, w, h, "F")

	pdf.SetTextColor(255, 255, 255)
	pdf.SetFont("Arial", "", 9)
	pdf.SetXY(x+3, y+2)
	pdf.CellFormat(w-6, 6, label, "", 0, "C", false, 0, "")

	pdf.SetFont("Arial", "B", 18)
	pdf.SetXY(x+3, y+10)
	pdf.CellFormat(w-6, 12, fmt.Sprintf("%d", value), "", 0, "C", false, 0, "")
}

// LoadReviewSection loads review data from step-2-analyze.json.
func LoadReviewSection(inputDir, repo string) (*ReviewSection, error) {
	section := &ReviewSection{
		Repo:        repo,
		GeneratedAt: time.Now(),
		Dashboard:   ReviewDashboard{},
	}

	analyzeData, err := readAnalyzeArtifact(inputDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read analyze file: %w", err)
	}

	var analyzeResult struct {
		GeneratedAt   string `json:"generatedAt"`
		ReviewPayload *struct {
			TotalPRs    int `json:"total_prs"`
			ReviewedPRs int `json:"reviewed_prs"`
			Categories  []struct {
				Category string `json:"category"`
				Count    int    `json:"count"`
			} `json:"categories"`
			Buckets []struct {
				Bucket string `json:"bucket"`
				Count  int    `json:"count"`
			} `json:"buckets"`
			RiskBuckets []struct {
				Bucket string `json:"bucket"`
				Count  int    `json:"count"`
			} `json:"risk_buckets"`
			PriorityTiers []struct {
				Tier  string `json:"tier"`
				Count int    `json:"count"`
			} `json:"priority_tiers"`
		} `json:"review_payload"`
	}

	if err := json.Unmarshal(analyzeData, &analyzeResult); err != nil {
		return nil, fmt.Errorf("failed to parse analyze JSON: %w", err)
	}

	if analyzeResult.GeneratedAt != "" {
		if t, err := time.Parse(time.RFC3339, analyzeResult.GeneratedAt); err == nil {
			section.GeneratedAt = t
		}
	}

	if analyzeResult.ReviewPayload == nil {
		return section, nil
	}

	review := analyzeResult.ReviewPayload
	section.Dashboard.TotalPRs = review.TotalPRs
	section.Dashboard.ReviewedPRs = review.ReviewedPRs

	for _, bucket := range review.Buckets {
		switch bucket.Bucket {
		case "now":
			section.Dashboard.MergeNow = bucket.Count
		case "future":
			section.Dashboard.FocusedReview = bucket.Count
		case "duplicate":
			section.Dashboard.Duplicate = bucket.Count
		case "junk":
			section.Dashboard.Problematic = bucket.Count
		case "blocked":
			section.Dashboard.Escalate = bucket.Count
		}
	}

	for _, bucket := range review.RiskBuckets {
		switch bucket.Bucket {
		case "security_risk":
			section.Dashboard.SecurityRisk = bucket.Count
		case "reliability_risk":
			section.Dashboard.ReliabilityRisk = bucket.Count
		case "performance_risk":
			section.Dashboard.PerformanceRisk = bucket.Count
		}
	}

	for _, tier := range review.PriorityTiers {
		switch tier.Tier {
		case "fast_merge":
			section.Dashboard.FastMerge = tier.Count
		case "review_required":
			section.Dashboard.ReviewRequired = tier.Count
		case "blocked":
			section.Dashboard.Blocked = tier.Count
		}
	}

	if section.Dashboard.TotalPRs == 0 {
		section.Dashboard.TotalPRs = len(review.Categories)
	}

	return section, nil
}

// Ensure ReviewSection implements PDFSection.
var _ PDFSection = (*ReviewSection)(nil)
