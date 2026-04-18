package report

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/go-pdf/fpdf"
	"github.com/jeffersonnunn/pratc/internal/types"
)

type AnalystRow struct {
	PRNumber       int
	Title          string
	Author         string
	Age            string
	Cluster        string
	Classification string
	Action         string
	ProblemType    string
	Score          float64
	Reasons        []string
}

type AnalystDuplicateEntry struct {
	CanonicalPRNumber int
	CanonicalTitle    string
	Similarity        float64
	Reason            string
	DuplicatePRs      []AnalystRow
}

type analystDataset struct {
	Repo             string
	GeneratedAt      time.Time
	Rows             []AnalystRow
	JunkRows         []AnalystRow
	TopUsefulRows    []AnalystRow
	Duplicates       []AnalystDuplicateEntry
	CategoryCounts   map[string]int
	CategoryExamples map[string][]AnalystRow
	PlanSelected     []AnalystRow
	PlanRejected     []AnalystRow
}

func LoadAnalystDataset(inputDir, repo string) (*analystDataset, error) {
	return loadAnalystDataset(inputDir, repo)
}

func loadAnalystDataset(inputDir, repo string) (*analystDataset, error) {
	analyzeData, err := readAnalyzeArtifact(inputDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read analyze artifact: %w", err)
	}

	var analyze types.AnalysisResponse
	if err := json.Unmarshal(analyzeData, &analyze); err != nil {
		return nil, fmt.Errorf("parse analyze artifact: %w", err)
	}

	dataset := &analystDataset{
		Repo:             repo,
		GeneratedAt:      time.Now(),
		CategoryCounts:   make(map[string]int),
		CategoryExamples: make(map[string][]AnalystRow),
	}
	if analyze.GeneratedAt != "" {
		if t, err := time.Parse(time.RFC3339, analyze.GeneratedAt); err == nil {
			dataset.GeneratedAt = t
		}
	}

	staleByPR := make(map[int]types.StalenessReport)
	for _, stale := range analyze.StalenessSignals {
		staleByPR[stale.PRNumber] = stale
	}

	dupByPR := make(map[int][]types.DuplicateGroup)
	for _, dup := range analyze.Duplicates {
		dupByPR[dup.CanonicalPRNumber] = append(dupByPR[dup.CanonicalPRNumber], dup)
		for _, prNum := range dup.DuplicatePRNums {
			dupByPR[prNum] = append(dupByPR[prNum], dup)
		}
	}
	for _, dup := range analyze.Overlaps {
		dupByPR[dup.CanonicalPRNumber] = append(dupByPR[dup.CanonicalPRNumber], dup)
		for _, prNum := range dup.DuplicatePRNums {
			dupByPR[prNum] = append(dupByPR[prNum], dup)
		}
	}

	rowsByPR := make(map[int]AnalystRow)
	for idx, pr := range analyze.PRs {
		result := resultForPR(analyze.ReviewPayload, analyze.PRs, pr.Number, idx)
		stale := staleByPR[pr.Number]
		classification := classifyAnalystRow(result, stale)
		reasons := mergeAnalystReasonLists(result.Reasons, result.Blockers, stale.Reasons, duplicateReasons(dupByPR[pr.Number]))
		row := AnalystRow{
			PRNumber:       pr.Number,
			Title:          pr.Title,
			Author:         pr.Author,
			Age:            relativeAge(pr.UpdatedAt, pr.CreatedAt, dataset.GeneratedAt),
			Cluster:        clusterLabel(pr),
			Classification: classification,
			Action:         analystAction(result, classification),
			ProblemType:    result.ProblemType,
			Score:          result.Confidence,
			Reasons:        mergeAnalystReasonLists(reasons, decisionLayerSummaries(result.DecisionLayers)),
		}
		rowsByPR[row.PRNumber] = row
		dataset.Rows = append(dataset.Rows, row)
		dataset.CategoryCounts[classification]++
		if len(dataset.CategoryExamples[classification]) < 3 {
			dataset.CategoryExamples[classification] = append(dataset.CategoryExamples[classification], row)
		}
		if classification == "junk" || result.ProblemType == "spam" || result.ProblemType == "junk" {
			dataset.JunkRows = append(dataset.JunkRows, row)
		}
	}

	// Include PRs from the outer peel garbage classifier
	for _, g := range analyze.GarbagePRs {
		dataset.JunkRows = append(dataset.JunkRows, AnalystRow{
			PRNumber:       g.PRNumber,
			Title:          fmt.Sprintf("(PR #%d — outer peel)", g.PRNumber),
			Age:            "",
			Cluster:        "",
			Classification: "junk",
			Action:         "close",
			Score:          0,
			Reasons:        []string{g.Reason},
		})
		dataset.CategoryCounts["junk"]++
	}

	sort.Slice(dataset.Rows, func(i, j int) bool {
		if dataset.Rows[i].Classification != dataset.Rows[j].Classification {
			return analystRank(dataset.Rows[i].Classification) < analystRank(dataset.Rows[j].Classification)
		}
		if dataset.Rows[i].Score != dataset.Rows[j].Score {
			return dataset.Rows[i].Score > dataset.Rows[j].Score
		}
		return dataset.Rows[i].PRNumber < dataset.Rows[j].PRNumber
	})
	if len(dataset.JunkRows) > 0 {
		sort.Slice(dataset.JunkRows, func(i, j int) bool {
			if dataset.JunkRows[i].Score != dataset.JunkRows[j].Score {
				return dataset.JunkRows[i].Score > dataset.JunkRows[j].Score
			}
			return dataset.JunkRows[i].PRNumber < dataset.JunkRows[j].PRNumber
		})
	}

	for _, row := range dataset.Rows {
		if row.Classification == "merge_candidate" || row.Classification == "high_value" || row.Classification == "needs_review" {
			dataset.TopUsefulRows = append(dataset.TopUsefulRows, row)
		}
	}
	if len(dataset.TopUsefulRows) > 25 {
		dataset.TopUsefulRows = dataset.TopUsefulRows[:25]
	}

	prByNumber := make(map[int]types.PR)
	for _, pr := range analyze.PRs {
		prByNumber[pr.Number] = pr
	}
	for _, dup := range analyze.Duplicates {
		entry := AnalystDuplicateEntry{
			CanonicalPRNumber: dup.CanonicalPRNumber,
			CanonicalTitle:    prByNumber[dup.CanonicalPRNumber].Title,
			Similarity:        dup.Similarity,
			Reason:            dup.Reason,
		}
		for _, n := range dup.DuplicatePRNums {
			if row, ok := rowsByPR[n]; ok {
				entry.DuplicatePRs = append(entry.DuplicatePRs, row)
			}
		}
		sort.Slice(entry.DuplicatePRs, func(i, j int) bool { return entry.DuplicatePRs[i].PRNumber < entry.DuplicatePRs[j].PRNumber })
		dataset.Duplicates = append(dataset.Duplicates, entry)
	}
	sort.Slice(dataset.Duplicates, func(i, j int) bool {
		if dataset.Duplicates[i].Similarity != dataset.Duplicates[j].Similarity {
			return dataset.Duplicates[i].Similarity > dataset.Duplicates[j].Similarity
		}
		return dataset.Duplicates[i].CanonicalPRNumber < dataset.Duplicates[j].CanonicalPRNumber
	})

	planPath := inputDir + "/step-5-plan.json"
	if data, err := os.ReadFile(planPath); err == nil {
		var plan types.PlanResponse
		if err := json.Unmarshal(data, &plan); err == nil {
			for _, sel := range plan.Selected {
				if row, ok := rowsByPR[sel.PRNumber]; ok {
					row.Score = sel.Score
					row.Reasons = mergeAnalystReasonLists(row.Reasons, sel.Reasons)
					dataset.PlanSelected = append(dataset.PlanSelected, row)
				}
			}
			for _, rej := range plan.Rejections {
				if row, ok := rowsByPR[rej.PRNumber]; ok {
					row.Reasons = mergeAnalystReasonLists(row.Reasons, []string{rej.Reason})
					dataset.PlanRejected = append(dataset.PlanRejected, row)
				}
			}
		}
	}

	return dataset, nil
}

func clusterLabel(pr types.PR) string {
	if strings.TrimSpace(pr.ClusterID) == "" {
		return "—"
	}
	return pr.ClusterID
}

func relativeAge(updatedAt, createdAt string, now time.Time) string {
	ts := strings.TrimSpace(updatedAt)
	if ts == "" {
		ts = strings.TrimSpace(createdAt)
	}
	if ts == "" {
		return "unknown"
	}
	parsed, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		return "unknown"
	}
	delta := now.Sub(parsed)
	if delta < 24*time.Hour {
		return fmt.Sprintf("%dh", int(delta.Hours()))
	}
	if delta < 30*24*time.Hour {
		return fmt.Sprintf("%dd", int(delta.Hours()/24))
	}
	if delta < 365*24*time.Hour {
		return fmt.Sprintf("%dmo", int(delta.Hours()/(24*30)))
	}
	return fmt.Sprintf("%dy", int(delta.Hours()/(24*365)))
}

func classifyAnalystRow(result types.ReviewResult, stale types.StalenessReport) string {
	// Trust the review engine's temporal bucket when available.
	if result.TemporalBucket != "" {
		switch result.TemporalBucket {
		case "now":
			if result.Category == types.ReviewCategoryMergeNow {
				return "merge_candidate"
			}
			if result.Category == types.ReviewCategoryMergeAfterFocusedReview {
				if stale.Score >= 75 {
					return "high_value"
				}
				return "needs_review"
			}
			return "needs_review"
		case "future":
			if stale.Score >= 75 {
				return "re_engage"
			}
			return "low_value"
		case "blocked":
			if result.Category == types.ReviewCategoryProblematicQuarantine {
				if result.ProblemType == "spam" || result.ProblemType == "junk" {
					return "junk"
				}
				if stale.Score >= 75 {
					return "stale"
				}
				return "junk"
			}
			return "blocked"
		}
	}

	// Fallback: classify from review category.
	switch result.Category {
	case types.ReviewCategoryMergeNow:
		return "merge_candidate"
	case types.ReviewCategoryMergeAfterFocusedReview:
		if stale.Score >= 75 {
			return "high_value"
		}
		return "needs_review"
	case types.ReviewCategoryDuplicateSuperseded:
		return "duplicate"
	case types.ReviewCategoryProblematicQuarantine:
		if result.ProblemType == "spam" || result.ProblemType == "junk" {
			return "junk"
		}
		if stale.Score >= 75 {
			return "stale"
		}
		return "junk"
	case types.ReviewCategoryUnknownEscalate:
		if stale.Score >= 75 {
			return "re_engage"
		}
		return "low_value"
	default:
		if stale.Score >= 75 {
			return "stale"
		}
		return "low_value"
	}
}

func decisionLayerSummaries(layers []types.DecisionLayer) []string {
	if len(layers) == 0 {
		return nil
	}
	summaries := make([]string, 0, len(layers))
	for _, layer := range layers {
		// Skip layers with empty status (no observation).
		if layer.Status == "" || layer.Status == "skip" {
			continue
		}
		// Show layers that peeled, flagged, or have non-trivial reasons.
		if layer.Status == "peeled" || layer.Status == "flagged" || len(layer.Reasons) > 0 {
			if len(layer.Reasons) == 0 {
				summaries = append(summaries, fmt.Sprintf("L%d %s (%s)", layer.Layer, layer.Name, layer.Status))
				continue
			}
			// Trim verbose reasons to keep the trail readable.
			trimmed := layer.Reasons
			if len(trimmed) > 3 {
				trimmed = trimmed[:3]
			}
			summaries = append(summaries, fmt.Sprintf("L%d %s: %s", layer.Layer, layer.Name, strings.Join(trimmed, "; ")))
		}
	}
	return summaries
}

func analystAction(result types.ReviewResult, classification string) string {
	switch {
	case result.NextAction == "merge":
		return "merge_soon"
	case result.NextAction == "duplicate":
		return "close_duplicate"
	case result.NextAction == "close":
		return "close_junk"
	case result.NextAction == "quarantine":
		return "inspect_now"
	case result.NextAction == "escalate":
		return "review_later"
	case classification == "re_engage":
		return "re_engage"
	case classification == "low_value":
		return "defer"
	default:
		return "inspect_now"
	}
}

func duplicateReasons(groups []types.DuplicateGroup) []string {
	reasons := make([]string, 0, len(groups))
	for _, group := range groups {
		if strings.TrimSpace(group.Reason) != "" {
			reasons = append(reasons, group.Reason)
		}
	}
	return reasons
}

func mergeAnalystReasonLists(parts ...[]string) []string {
	seen := make(map[string]struct{})
	merged := make([]string, 0)
	for _, group := range parts {
		for _, item := range group {
			item = strings.TrimSpace(item)
			if item == "" {
				continue
			}
			if _, ok := seen[item]; ok {
				continue
			}
			seen[item] = struct{}{}
			merged = append(merged, item)
		}
	}
	return merged
}

func analystRank(classification string) int {
	switch classification {
	case "merge_candidate":
		return 0
	case "high_value":
		return 1
	case "needs_review":
		return 2
	case "duplicate":
		return 3
	case "blocked":
		return 4
	case "re_engage":
		return 5
	case "stale":
		return 6
	case "low_value":
		return 7
	case "junk":
		return 8
	default:
		return 9
	}
}

func getResultsSafe(r *types.ReviewResponse, prs []types.PR) []types.ReviewResult {
	if r == nil {
		return nil
	}
	results := make([]types.ReviewResult, 0, len(r.Results))
	for idx, result := range r.Results {
		if result.PRNumber == 0 && idx < len(prs) {
			result.PRNumber = prs[idx].Number
			if result.Title == "" {
				result.Title = prs[idx].Title
			}
			if result.Author == "" {
				result.Author = prs[idx].Author
			}
			if result.ClusterID == "" {
				result.ClusterID = prs[idx].ClusterID
			}
		}
		results = append(results, result)
	}
	return results
}

func resultForPR(r *types.ReviewResponse, prs []types.PR, prNumber, idx int) types.ReviewResult {
	if r == nil {
		return types.ReviewResult{PRNumber: prNumber, NextAction: "review", Reasons: []string{}, Blockers: []string{}, EvidenceReferences: []string{}, AnalyzerFindings: []types.AnalyzerFinding{}}
	}
	for _, result := range getResultsSafe(r, prs) {
		if result.PRNumber == prNumber {
			return ensureReviewResultDefaults(result, prNumber)
		}
	}
	if idx >= 0 && idx < len(r.Results) {
		return ensureReviewResultDefaults(r.Results[idx], prNumber)
	}
	return types.ReviewResult{PRNumber: prNumber, NextAction: "review", Reasons: []string{}, Blockers: []string{}, EvidenceReferences: []string{}, AnalyzerFindings: []types.AnalyzerFinding{}}
}

func ensureReviewResultDefaults(result types.ReviewResult, prNumber int) types.ReviewResult {
	if result.PRNumber == 0 {
		result.PRNumber = prNumber
	}
	if result.Reasons == nil {
		result.Reasons = []string{}
	}
	if result.Blockers == nil {
		result.Blockers = []string{}
	}
	if result.EvidenceReferences == nil {
		result.EvidenceReferences = []string{}
	}
	if result.AnalyzerFindings == nil {
		result.AnalyzerFindings = []types.AnalyzerFinding{}
	}
	if result.NextAction == "" {
		result.NextAction = "review"
	}
	return result
}

type AnalystSummarySection struct {
	Repo           string
	GeneratedAt    time.Time
	CategoryCounts map[string]int
	Examples       map[string][]AnalystRow
}

func LoadAnalystSummarySection(inputDir, repo string) (*AnalystSummarySection, error) {
	data, err := loadAnalystDataset(inputDir, repo)
	if err != nil {
		return nil, err
	}
	return &AnalystSummarySection{Repo: repo, GeneratedAt: data.GeneratedAt, CategoryCounts: data.CategoryCounts, Examples: data.CategoryExamples}, nil
}

func (s *AnalystSummarySection) Render(pdf *fpdf.Fpdf) {
	pdf.AddPage()
	pdf.SetFillColor(26, 82, 118)
	pdf.Rect(0, 0, 210, 35, "F")
	pdf.SetTextColor(255, 255, 255)
	pdf.SetFont("Arial", "B", 18)
	pdf.SetXY(15, 12)
	pdf.Cell(180, 10, "Analyst Summary")
	pdf.SetTextColor(0, 0, 0)
	pdf.SetFont("Arial", "", 10)
	pdf.SetXY(15, 40)
	pdf.Cell(180, 6, fmt.Sprintf("Repository: %s | Generated: %s", s.Repo, s.GeneratedAt.Format(time.RFC1123)))

	// Compute total for percentages.
	total := 0
	for _, count := range s.CategoryCounts {
		total += count
	}

	cats := sortedCategoryKeys(s.CategoryCounts)
	y := 55.0
	for _, cat := range cats {
		count := s.CategoryCounts[cat]
		pct := float64(0)
		if total > 0 {
			pct = float64(count) / float64(total) * 100
		}

		r, g, b := classificationColor(cat)

		// Color badge.
		pdf.SetFillColor(r, g, b)
		pdf.SetTextColor(255, 255, 255)
		pdf.SetFont("Arial", "B", 11)
		badgeW := 55.0
		pdf.Rect(15, y, badgeW, 7, "F")
		pdf.SetXY(15, y)
		pdf.CellFormat(badgeW, 7, cat, "", 0, "C", false, 0, "")

		// Count + percentage.
		pdf.SetTextColor(0, 0, 0)
		pdf.SetFont("Arial", "", 10)
		pdf.SetXY(75, y)
		pdf.Cell(40, 7, fmt.Sprintf("%d (%.1f%%)", count, pct))
		y += 9

		// Progress bar.
		barMaxW := 180.0
		barW := barMaxW * pct / 100
		pdf.SetFillColor(236, 240, 241)
		pdf.Rect(15, y, barMaxW, 3, "F")
		pdf.SetFillColor(r, g, b)
		if barW > 0 {
			pdf.Rect(15, y, barW, 3, "F")
		}
		y += 6

		// Examples.
		pdf.SetFont("Arial", "", 9)
		examples := s.Examples[cat]
		for _, ex := range examples {
			pdf.SetXY(20, y)
			pdf.Cell(175, 5, fmt.Sprintf("#%d %s — %s", ex.PRNumber, truncate(ex.Title, 70), truncate(strings.Join(ex.Reasons, "; "), 60)))
			y += 5
		}
		y += 4
		if y > 260 {
			pdf.AddPage()
			y = 20
		}
	}
}

type DecisionTrailRow struct {
	PRNumber       int
	Title          string
	Classification string
	Action         string
	Confidence     float64
	Layers         []string
}

type DecisionTrailSection struct {
	Repo        string
	GeneratedAt time.Time
	Rows        []DecisionTrailRow
	TotalRows   int
	Grouped     map[string][]DecisionTrailRow // classification → rows
}

func LoadDecisionTrailSection(inputDir, repo string) (*DecisionTrailSection, error) {
	data, err := loadAnalystDataset(inputDir, repo)
	if err != nil {
		return nil, err
	}
	// Build grouped rows by classification.
	grouped := make(map[string][]DecisionTrailRow)
	allRows := make([]DecisionTrailRow, 0, len(data.Rows))
	for _, row := range data.Rows {
		trailRow := DecisionTrailRow{
			PRNumber:       row.PRNumber,
			Title:          row.Title,
			Classification: row.Classification,
			Action:         row.Action,
			Confidence:     row.Score,
			Layers:         row.Reasons,
		}
		allRows = append(allRows, trailRow)
		grouped[row.Classification] = append(grouped[row.Classification], trailRow)
	}
	return &DecisionTrailSection{
		Repo:        repo,
		GeneratedAt: data.GeneratedAt,
		Rows:        allRows,
		TotalRows:   len(allRows),
		Grouped:     grouped,
	}, nil
}

// renderOrder defines the display order for classification groups.
var renderOrder = []string{"merge_candidate", "high_value", "needs_review", "duplicate", "blocked", "re_engage", "stale", "low_value", "junk"}

// classificationColor returns (r,g,b) for a classification badge.
func classificationColor(class string) (int, int, int) {
	switch class {
	case "merge_candidate":
		return 39, 174, 96  // green
	case "high_value":
		return 41, 128, 185 // blue
	case "needs_review":
		return 243, 156, 18 // amber
	case "duplicate":
		return 149, 165, 166 // grey
	case "blocked":
		return 231, 76, 60  // red
	case "re_engage":
		return 155, 89, 182 // purple
	case "stale":
		return 127, 140, 141 // dark grey
	case "low_value":
		return 189, 195, 199 // light grey
	case "junk":
		return 192, 57, 43  // dark red
	default:
		return 52, 73, 94   // dark blue
	}
}

func (s *DecisionTrailSection) Render(pdf *fpdf.Fpdf) {
	pdf.AddPage()
	pdf.SetFillColor(26, 82, 118)
	pdf.Rect(0, 0, 210, 35, "F")
	pdf.SetTextColor(255, 255, 255)
	pdf.SetFont("Arial", "B", 18)
	pdf.SetXY(15, 12)
	pdf.Cell(180, 10, "Decision Trail")
	pdf.SetTextColor(0, 0, 0)
	pdf.SetFont("Arial", "", 10)
	pdf.SetXY(15, 40)
	pdf.Cell(180, 6, fmt.Sprintf("Repository: %s | Generated: %s | %d PRs", s.Repo, s.GeneratedAt.Format(time.RFC1123), s.TotalRows))

	// Render grouped summary cards first.
	y := 55.0
	pdf.SetFont("Arial", "B", 13)
	pdf.SetXY(15, y)
	pdf.Cell(180, 8, "Classification Summary")
	y += 12

	for _, class := range renderOrder {
		rows := s.Grouped[class]
		if len(rows) == 0 {
			continue
		}
		r, g, b := classificationColor(class)
		// Category badge.
		pdf.SetFillColor(r, g, b)
		pdf.SetTextColor(255, 255, 255)
		pdf.SetFont("Arial", "B", 9)
		badgeW := 55.0
		pdf.Rect(15, y, badgeW, 7, "F")
		pdf.SetXY(15, y)
		pdf.CellFormat(badgeW, 7, fmt.Sprintf("%s: %d", class, len(rows)), "", 0, "C", false, 0, "")
		pdf.SetTextColor(0, 0, 0)
		pdf.SetFont("Arial", "", 8)
		pdf.SetXY(75, y)
		pdf.Cell(120, 7, truncate(rows[0].Title, 70))
		y += 9

		if y > 250 {
			pdf.AddPage()
			y = 20
		}
	}

	// Render top-N detailed rows per group (cap total at ~50).
	y += 8
	pdf.SetFont("Arial", "B", 13)
	pdf.SetXY(15, y)
	pdf.Cell(180, 8, "Detailed Trail (Top Actions)")
	y += 12

	rendered := 0
	maxDetailed := 50
	for _, class := range renderOrder {
		rows := s.Grouped[class]
		if len(rows) == 0 {
			continue
		}
		if rendered >= maxDetailed {
			break
		}
		// Section header.
		if y > 260 {
			pdf.AddPage()
			y = 20
		}
		r, g, b := classificationColor(class)
		pdf.SetFillColor(r, g, b)
		pdf.Rect(15, y, 180, 7, "F")
		pdf.SetTextColor(255, 255, 255)
		pdf.SetFont("Arial", "B", 9)
		pdf.SetXY(17, y)
		pdf.Cell(176, 7, fmt.Sprintf("%s (%d PRs)", class, len(rows)))
		pdf.SetTextColor(0, 0, 0)
		y += 9

		// Table header.
		pdf.SetFont("Arial", "B", 7)
		pdf.SetFillColor(236, 240, 241)
		pdf.SetXY(15, y)
		pdf.CellFormat(14, 6, "PR", "1", 0, "L", true, 0, "")
		pdf.CellFormat(52, 6, "Title", "1", 0, "L", true, 0, "")
		pdf.CellFormat(16, 6, "Action", "1", 0, "L", true, 0, "")
		pdf.CellFormat(12, 6, "Conf", "1", 0, "C", true, 0, "")
		pdf.CellFormat(86, 6, "Decision Trail", "1", 1, "L", true, 0, "")
		y += 6

		limit := 5
		if rendered+limit > maxDetailed {
			limit = maxDetailed - rendered
		}
		for i, row := range rows {
			if i >= limit || rendered >= maxDetailed {
				break
			}
			if y > 270 {
				pdf.AddPage()
				y = 20
			}
			pdf.SetFont("Arial", "", 7)
			pdf.SetXY(15, y)
			pdf.CellFormat(14, 6, fmt.Sprintf("#%d", row.PRNumber), "1", 0, "L", false, 0, "")
			pdf.CellFormat(52, 6, truncate(row.Title, 34), "1", 0, "L", false, 0, "")
			pdf.CellFormat(16, 6, truncate(row.Action, 10), "1", 0, "L", false, 0, "")
			pdf.CellFormat(12, 6, fmt.Sprintf("%.2f", row.Confidence), "1", 0, "C", false, 0, "")
			pdf.CellFormat(86, 6, truncate(strings.Join(row.Layers, " | "), 75), "1", 1, "L", false, 0, "")
			y += 6
			rendered++
		}
		y += 4
	}

	if s.TotalRows > maxDetailed {
		if y > 260 {
			pdf.AddPage()
			y = 20
		}
		pdf.SetFont("Arial", "I", 9)
		pdf.SetXY(15, y)
		pdf.Cell(180, 6, fmt.Sprintf("Showing %d of %d PRs. Full table in appendix.", maxDetailed, s.TotalRows))
	}
}

type FullPRTableSection struct {
	Repo        string
	GeneratedAt time.Time
	Rows        []AnalystRow
}

func LoadFullPRTableSection(inputDir, repo string) (*FullPRTableSection, error) {
	data, err := loadAnalystDataset(inputDir, repo)
	if err != nil {
		return nil, err
	}
	return &FullPRTableSection{Repo: repo, GeneratedAt: data.GeneratedAt, Rows: data.Rows}, nil
}

func (s *FullPRTableSection) Render(pdf *fpdf.Fpdf) {
	pdf.AddPage()
	pdf.SetFillColor(26, 82, 118)
	pdf.Rect(0, 0, 210, 35, "F")
	pdf.SetTextColor(255, 255, 255)
	pdf.SetFont("Arial", "B", 18)
	pdf.SetXY(15, 12)
	pdf.Cell(180, 10, "Appendix: Full PR Analysis")
	pdf.SetTextColor(0, 0, 0)
	pdf.SetFont("Arial", "", 10)
	pdf.SetXY(15, 40)
	pdf.Cell(180, 6, fmt.Sprintf("Repository: %s | Generated: %s | %d PRs total", s.Repo, s.GeneratedAt.Format(time.RFC1123), len(s.Rows)))

	// Group by classification and render summary table.
	grouped := make(map[string][]AnalystRow)
	for _, row := range s.Rows {
		grouped[row.Classification] = append(grouped[row.Classification], row)
	}

	y := 55.0
	pdf.SetFont("Arial", "B", 12)
	pdf.SetXY(15, y)
	pdf.Cell(180, 8, "Classification Distribution")
	y += 12

	// Summary bar.
	pdf.SetFont("Arial", "B", 7)
	pdf.SetFillColor(236, 240, 241)
	pdf.SetXY(15, y)
	pdf.CellFormat(40, 7, "Classification", "1", 0, "L", true, 0, "")
	pdf.CellFormat(20, 7, "Count", "1", 0, "C", true, 0, "")
	pdf.CellFormat(25, 7, "Pct", "1", 0, "C", true, 0, "")
	pdf.CellFormat(95, 7, "Top Example", "1", 1, "L", true, 0, "")
	y += 7

	renderOrder := []string{"merge_candidate", "high_value", "needs_review", "duplicate", "blocked", "re_engage", "stale", "low_value", "junk"}
	total := float64(len(s.Rows))
	for _, class := range renderOrder {
		rows := grouped[class]
		if len(rows) == 0 {
			continue
		}
		r, g, b := classificationColor(class)
		pdf.SetFillColor(r, g, b)
		pdf.Rect(15, y, 2, 6, "F")

		pdf.SetFont("Arial", "", 7)
		pdf.SetXY(18, y)
		pdf.CellFormat(37, 6, class, "1", 0, "L", false, 0, "")
		pdf.CellFormat(20, 6, fmt.Sprintf("%d", len(rows)), "1", 0, "C", false, 0, "")
		pdf.CellFormat(25, 6, fmt.Sprintf("%.1f%%", float64(len(rows))/total*100), "1", 0, "C", false, 0, "")
		pdf.CellFormat(95, 6, truncate(rows[0].Title, 70), "1", 1, "L", false, 0, "")
		y += 6

		if y > 260 {
			pdf.AddPage()
			y = 20
		}
	}

	// Note about full data.
	y += 4
	pdf.SetFont("Arial", "I", 8)
	pdf.SetXY(15, y)
	pdf.Cell(180, 5, fmt.Sprintf("Full data (%d PRs) available in analyze.json. See Decision Trail section for detailed examples.", len(s.Rows)))
}

type SpamJunkSection struct {
	Repo        string
	GeneratedAt time.Time
	Rows        []AnalystRow
}

func LoadSpamJunkSection(inputDir, repo string) (*SpamJunkSection, error) {
	data, err := loadAnalystDataset(inputDir, repo)
	if err != nil {
		return nil, err
	}
	return &SpamJunkSection{Repo: repo, GeneratedAt: data.GeneratedAt, Rows: data.JunkRows}, nil
}

func (s *SpamJunkSection) Render(pdf *fpdf.Fpdf) {
	pdf.AddPage()
	pdf.SetFillColor(192, 57, 43) // dark red header for junk
	pdf.Rect(0, 0, 210, 35, "F")
	pdf.SetTextColor(255, 255, 255)
	pdf.SetFont("Arial", "B", 18)
	pdf.SetXY(15, 12)
	pdf.Cell(180, 10, "Junk PRs")
	pdf.SetTextColor(0, 0, 0)
	pdf.SetFont("Arial", "", 10)
	pdf.SetXY(15, 40)
	pdf.Cell(180, 6, fmt.Sprintf("Repository: %s | Generated: %s | %d junk PRs", s.Repo, s.GeneratedAt.Format(time.RFC1123), len(s.Rows)))

	// Cap at 50 with note.
	maxRows := 50
	rows := s.Rows
	if len(rows) > maxRows {
		rows = rows[:maxRows]
	}

	startY := 50.0
	renderAnalystRowHeader(pdf, startY)
	y := startY + 8
	for _, row := range rows {
		if y > 270 {
			pdf.AddPage()
			renderAnalystRowHeader(pdf, 20)
			y = 28
		}
		pdf.SetFont("Arial", "", 7)
		pdf.SetXY(10, y)
		pdf.CellFormat(14, 8, fmt.Sprintf("#%d", row.PRNumber), "1", 0, "L", false, 0, "")
		pdf.CellFormat(52, 8, truncate(row.Title, 34), "1", 0, "L", false, 0, "")
		pdf.CellFormat(24, 8, truncate(row.Author, 14), "1", 0, "L", false, 0, "")
		pdf.CellFormat(14, 8, row.Age, "1", 0, "C", false, 0, "")
		pdf.CellFormat(16, 8, truncate(row.Cluster, 9), "1", 0, "L", false, 0, "")
		pdf.CellFormat(22, 8, truncate(row.Classification, 13), "1", 0, "L", false, 0, "")
		pdf.CellFormat(22, 8, truncate(row.Action, 13), "1", 0, "L", false, 0, "")
		pdf.CellFormat(66, 8, truncate(strings.Join(row.Reasons, "; "), 50), "1", 1, "L", false, 0, "")
		y += 8
	}

	if len(s.Rows) > maxRows {
		if y > 260 {
			pdf.AddPage()
			y = 20
		}
		pdf.SetFont("Arial", "I", 9)
		pdf.SetXY(15, y)
		pdf.Cell(180, 6, fmt.Sprintf("Showing %d of %d junk PRs. Full list in analyze.json.", maxRows, len(s.Rows)))
	}
}

type DuplicateDetailSection struct {
	Repo        string
	GeneratedAt time.Time
	Entries     []AnalystDuplicateEntry
}

func LoadDuplicateDetailSection(inputDir, repo string) (*DuplicateDetailSection, error) {
	data, err := loadAnalystDataset(inputDir, repo)
	if err != nil {
		return nil, err
	}
	return &DuplicateDetailSection{Repo: repo, GeneratedAt: data.GeneratedAt, Entries: data.Duplicates}, nil
}

func (s *DuplicateDetailSection) Render(pdf *fpdf.Fpdf) {
	pdf.AddPage()
	pdf.SetFillColor(26, 82, 118)
	pdf.Rect(0, 0, 210, 35, "F")
	pdf.SetTextColor(255, 255, 255)
	pdf.SetFont("Arial", "B", 18)
	pdf.SetXY(15, 12)
	pdf.Cell(180, 10, "Duplicate Detail")
	pdf.SetTextColor(0, 0, 0)
	pdf.SetFont("Arial", "", 10)
	pdf.SetXY(15, 40)
	pdf.Cell(180, 6, fmt.Sprintf("Repository: %s | Generated: %s", s.Repo, s.GeneratedAt.Format(time.RFC1123)))
	y := 52.0
	for _, entry := range s.Entries {
		pdf.SetFont("Arial", "B", 11)
		pdf.SetXY(15, y)
		pdf.Cell(180, 6, fmt.Sprintf("Canonical PR #%d — %s (similarity %.2f)", entry.CanonicalPRNumber, truncate(entry.CanonicalTitle, 70), entry.Similarity))
		y += 6
		pdf.SetFont("Arial", "", 9)
		pdf.SetXY(20, y)
		pdf.MultiCell(170, 5, fmt.Sprintf("Reason: %s", entry.Reason), "", "L", false)
		y = pdf.GetY() + 1
		for _, dup := range entry.DuplicatePRs {
			pdf.SetXY(24, y)
			pdf.Cell(165, 5, fmt.Sprintf("• #%d %s — %s", dup.PRNumber, truncate(dup.Title, 60), truncate(strings.Join(dup.Reasons, "; "), 50)))
			y += 5
		}
		y += 4
		if y > 260 {
			pdf.AddPage()
			y = 20
		}
	}
}

type AnalystRecommendationsSection struct {
	Repo        string
	GeneratedAt time.Time
	Useful      []AnalystRow
	Rejected    []AnalystRow
	Junk        []AnalystRow
}

func LoadAnalystRecommendationsSection(inputDir, repo string) (*AnalystRecommendationsSection, error) {
	data, err := loadAnalystDataset(inputDir, repo)
	if err != nil {
		return nil, err
	}
	return &AnalystRecommendationsSection{Repo: repo, GeneratedAt: data.GeneratedAt, Useful: data.TopUsefulRows, Rejected: data.PlanRejected, Junk: data.JunkRows}, nil
}

func (s *AnalystRecommendationsSection) Render(pdf *fpdf.Fpdf) {
	pdf.AddPage()
	pdf.SetFont("Arial", "B", 16)
	pdf.SetXY(15, 20)
	pdf.Cell(180, 10, "Recommendations")
	y := 35.0

	// Summary recommendations based on counts
	pdf.SetFont("Arial", "", 10)
	pdf.SetXY(15, y)
	summaryLines := []string{}
	if len(s.Junk) > 0 {
		summaryLines = append(summaryLines, fmt.Sprintf("• Close %d garbage/junk PRs (see details below)", len(s.Junk)))
	}
	if len(s.Useful) > 0 {
		summaryLines = append(summaryLines, fmt.Sprintf("• Inspect %d high-value PRs for merge readiness", len(s.Useful)))
	}
	if len(s.Rejected) > 0 {
		summaryLines = append(summaryLines, fmt.Sprintf("• Review %d plan-rejected PRs for blockers", len(s.Rejected)))
	}
	if len(summaryLines) == 0 {
		summaryLines = append(summaryLines, "No actionable recommendations at this time.")
	}
	for _, line := range summaryLines {
		pdf.MultiCell(180, 6, line, "", "L", false)
	}
	y = pdf.GetY() + 4

	y = renderRecommendationList(pdf, y, "Top PRs to inspect now", s.Useful, 10)
	y = renderRecommendationList(pdf, y+4, "PRs to close as junk", s.Junk, 10)
	_ = renderRecommendationList(pdf, y+4, "PRs rejected from current plan", s.Rejected, 10)
}

func renderRecommendationList(pdf *fpdf.Fpdf, y float64, title string, rows []AnalystRow, limit int) float64 {
	if y > 250 {
		pdf.AddPage()
		y = 20
	}
	pdf.SetFont("Arial", "B", 12)
	pdf.SetXY(15, y)
	pdf.Cell(180, 8, title)
	y += 8
	pdf.SetFont("Arial", "", 9)
	if len(rows) == 0 {
		pdf.SetXY(20, y)
		pdf.Cell(170, 5, "None")
		return y + 6
	}
	if len(rows) > limit {
		rows = rows[:limit]
	}
	for _, row := range rows {
		if y > 270 {
			pdf.AddPage()
			y = 20
		}
		pdf.SetXY(20, y)
		pdf.MultiCell(170, 5, fmt.Sprintf("• #%d %s — %s — %s", row.PRNumber, truncate(row.Title, 60), row.Action, truncate(strings.Join(row.Reasons, "; "), 60)), "", "L", false)
		y = pdf.GetY()
	}
	return y
}

func renderAnalystRows(pdf *fpdf.Fpdf, title, repo string, generatedAt time.Time, rows []AnalystRow) {
	pdf.AddPage()
	pdf.SetFillColor(26, 82, 118)
	pdf.Rect(0, 0, 210, 35, "F")
	pdf.SetTextColor(255, 255, 255)
	pdf.SetFont("Arial", "B", 18)
	pdf.SetXY(15, 12)
	pdf.Cell(180, 10, title)
	pdf.SetTextColor(0, 0, 0)
	pdf.SetFont("Arial", "", 10)
	pdf.SetXY(15, 40)
	pdf.Cell(180, 6, fmt.Sprintf("Repository: %s | Generated: %s | Rows: %d", repo, generatedAt.Format(time.RFC1123), len(rows)))
	startY := 50.0
	renderAnalystRowHeader(pdf, startY)
	y := startY + 8
	for _, row := range rows {
		if y > 270 {
			pdf.AddPage()
			renderAnalystRowHeader(pdf, 20)
			y = 28
		}
		pdf.SetFont("Arial", "", 7)
		pdf.SetXY(10, y)
		pdf.CellFormat(14, 8, fmt.Sprintf("#%d", row.PRNumber), "1", 0, "L", false, 0, "")
		pdf.CellFormat(52, 8, truncate(row.Title, 34), "1", 0, "L", false, 0, "")
		pdf.CellFormat(24, 8, truncate(row.Author, 14), "1", 0, "L", false, 0, "")
		pdf.CellFormat(14, 8, row.Age, "1", 0, "C", false, 0, "")
		pdf.CellFormat(16, 8, truncate(row.Cluster, 9), "1", 0, "L", false, 0, "")
		pdf.CellFormat(22, 8, truncate(row.Classification, 13), "1", 0, "L", false, 0, "")
		pdf.CellFormat(22, 8, truncate(row.Action, 13), "1", 0, "L", false, 0, "")
		pdf.CellFormat(66, 8, truncate(strings.Join(row.Reasons, "; "), 50), "1", 1, "L", false, 0, "")
		y += 8
	}
}

func renderAnalystRowHeader(pdf *fpdf.Fpdf, y float64) {
	pdf.SetFont("Arial", "B", 7)
	pdf.SetFillColor(52, 73, 94)
	pdf.SetTextColor(255, 255, 255)
	pdf.SetXY(10, y)
	pdf.CellFormat(14, 8, "PR", "1", 0, "L", true, 0, "")
	pdf.CellFormat(52, 8, "Title", "1", 0, "L", true, 0, "")
	pdf.CellFormat(24, 8, "Author", "1", 0, "L", true, 0, "")
	pdf.CellFormat(14, 8, "Age", "1", 0, "C", true, 0, "")
	pdf.CellFormat(16, 8, "Cluster", "1", 0, "L", true, 0, "")
	pdf.CellFormat(22, 8, "Class", "1", 0, "L", true, 0, "")
	pdf.CellFormat(22, 8, "Action", "1", 0, "L", true, 0, "")
	pdf.CellFormat(66, 8, "Reasons", "1", 1, "L", true, 0, "")
	pdf.SetTextColor(0, 0, 0)
}

func sortedCategoryKeys(counts map[string]int) []string {
	keys := make([]string, 0, len(counts))
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if counts[keys[i]] != counts[keys[j]] {
			return counts[keys[i]] > counts[keys[j]]
		}
		return keys[i] < keys[j]
	})
	return keys
}
