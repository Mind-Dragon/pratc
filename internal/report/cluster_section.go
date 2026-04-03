package report

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/go-pdf/fpdf"
)

// ClusterSection renders the cluster analysis with PR groupings.
type ClusterSection struct {
	Repo        string
	GeneratedAt time.Time
	Clusters    []ClusterData
	Model       string
	Thresholds  ClusterThresholds
}

// ClusterData represents a cluster for rendering.
type ClusterData struct {
	ID                  string
	Size                int
	PRNumbers           []int
	RepresentativeTitle string
	Label               string
	HealthStatus        string
	AverageSimilarity   float64
}

// ClusterThresholds holds the clustering thresholds.
type ClusterThresholds struct {
	Duplicate float64
	Overlap   float64
}

// Render draws the cluster analysis section using fpdf primitives.
func (s *ClusterSection) Render(pdf *fpdf.Fpdf) {
	pdf.AddPage()

	// Header
	pdf.SetFillColor(26, 82, 118) // dark blue
	pdf.Rect(0, 0, 210, 35, "F")

	pdf.SetTextColor(255, 255, 255)
	pdf.SetFont("Arial", "B", 18)
	pdf.SetXY(15, 12)
	pdf.Cell(180, 10, "Cluster Analysis")

	// Repository info
	pdf.SetTextColor(0, 0, 0)
	pdf.SetFont("Arial", "", 10)
	pdf.SetXY(15, 40)
	pdf.Cell(180, 6, fmt.Sprintf("Repository: %s | Generated: %s", s.Repo, s.GeneratedAt.Format(time.RFC1123)))

	// Cluster statistics
	pdf.SetFont("Arial", "B", 12)
	pdf.SetXY(15, 52)
	pdf.Cell(180, 8, "Cluster Statistics")

	pdf.SetFont("Arial", "", 10)
	pdf.SetXY(15, 62)
	pdf.Cell(180, 6, fmt.Sprintf("Total Clusters: %d | Model: %s", len(s.Clusters), s.Model))

	// Calculate size distribution
	totalPRs := 0
	largestSize := 0
	largestCluster := ""
	for _, c := range s.Clusters {
		totalPRs += c.Size
		if c.Size > largestSize {
			largestSize = c.Size
			largestCluster = c.ID
		}
	}

	avgSize := 0.0
	if len(s.Clusters) > 0 {
		avgSize = float64(totalPRs) / float64(len(s.Clusters))
	}

	pdf.SetXY(15, 70)
	pdf.Cell(180, 6, fmt.Sprintf("Total PRs Clustered: %d | Average Size: %.1f | Largest: %s (%d PRs)", totalPRs, avgSize, largestCluster, largestSize))

	// Render cluster list
	yPos := pdf.GetY() + 15

	// Table header
	pdf.SetFont("Arial", "B", 10)
	pdf.SetFillColor(52, 73, 94)
	pdf.SetTextColor(255, 255, 255)
	pdf.SetXY(15, yPos)
	pdf.CellFormat(35, 8, "Cluster ID", "1", 0, "L", true, 0, "")
	pdf.CellFormat(20, 8, "Size", "1", 0, "C", true, 0, "")
	pdf.CellFormat(50, 8, "PR Numbers", "1", 0, "L", true, 0, "")
	pdf.CellFormat(85, 8, "Representative Title", "1", 1, "L", true, 0, "")

	// Render clusters
	pdf.SetTextColor(0, 0, 0)
	pdf.SetFont("Arial", "", 9)

	rowY := yPos + 8
	maxRows := 25 // Limit to fit on one page
	rowsRendered := 0

	for _, c := range s.Clusters {
		if rowsRendered >= maxRows {
			break
		}

		// Health status color
		healthColor := map[string]struct{ r, g, b int }{
			"green":  {39, 174, 96},
			"yellow": {241, 196, 15},
			"red":    {231, 76, 60},
		}
		color := healthColor["red"] // default
		if c, ok := healthColor[c.HealthStatus]; ok {
			color = c
		}

		pdf.SetFillColor(color.r, color.g, color.b)
		pdf.SetXY(15, rowY)

		// Cluster ID
		pdf.CellFormat(35, 7, c.ID, "1", 0, "L", false, 0, "")

		// Size
		pdf.CellFormat(20, 7, fmt.Sprintf("%d", c.Size), "1", 0, "C", false, 0, "")

		// PR Numbers (truncated if too many)
		prList := formatPRNumbers(c.PRNumbers)
		pdf.CellFormat(50, 7, prList, "1", 0, "L", false, 0, "")

		// Representative Title
		pdf.SetFillColor(236, 240, 241)
		title := truncate(c.RepresentativeTitle, 40)
		pdf.CellFormat(85, 7, title, "1", 1, "L", false, 0, "")

		rowY += 7
		rowsRendered++

		// Check if we need a new page
		if rowY > 270 && rowsRendered < len(s.Clusters) {
			// Add continuation note
			pdf.SetFont("Arial", "I", 9)
			pdf.SetXY(15, rowY-5)
			pdf.Cell(180, 6, fmt.Sprintf("(Showing %d of %d clusters - continued on next page)", rowsRendered, len(s.Clusters)))

			// New page for more clusters
			pdf.AddPage()
			rowY = 20

			// Re-render header on new page
			pdf.SetFont("Arial", "B", 10)
			pdf.SetFillColor(52, 73, 94)
			pdf.SetTextColor(255, 255, 255)
			pdf.SetXY(15, rowY)
			pdf.CellFormat(35, 8, "Cluster ID", "1", 0, "L", true, 0, "")
			pdf.CellFormat(20, 8, "Size", "1", 0, "C", true, 0, "")
			pdf.CellFormat(50, 8, "PR Numbers", "1", 0, "L", true, 0, "")
			pdf.CellFormat(85, 8, "Representative Title", "1", 1, "L", true, 0, "")

			pdf.SetTextColor(0, 0, 0)
			pdf.SetFont("Arial", "", 9)
			rowY += 8
		}
	}

	// If we have more clusters than shown, add note
	if len(s.Clusters) > maxRows {
		pdf.SetFont("Arial", "I", 9)
		pdf.SetXY(15, rowY+5)
		pdf.Cell(180, 6, fmt.Sprintf("Note: Showing first %d of %d clusters. Full data available in step-3-cluster.json", maxRows, len(s.Clusters)))
	}
}

// formatPRNumbers formats PR numbers as a comma-separated list, truncated if needed.
func formatPRNumbers(prs []int) string {
	if len(prs) == 0 {
		return ""
	}
	if len(prs) <= 5 {
		result := ""
		for i, pr := range prs {
			if i > 0 {
				result += ", "
			}
			result += fmt.Sprintf("#%d", pr)
		}
		return result
	}
	// Show first 3 and last 2
	result := fmt.Sprintf("#%d, #%d, #%d", prs[0], prs[1], prs[2])
	if len(prs) > 3 {
		result += fmt.Sprintf(", ... #%d, #%d", prs[len(prs)-2], prs[len(prs)-1])
	}
	return result
}

// LoadClusterSection loads cluster data from step-3-cluster.json and returns a ClusterSection.
func LoadClusterSection(inputDir, repo string) (*ClusterSection, error) {
	section := &ClusterSection{
		Repo:        repo,
		GeneratedAt: time.Now(),
	}

	clusterPath := inputDir + "/step-3-cluster.json"
	clusterData, err := os.ReadFile(clusterPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read cluster file: %w", err)
	}

	var clusterResult struct {
		Repo        string `json:"repo"`
		GeneratedAt string `json:"generatedAt"`
		Model       string `json:"model"`
		Thresholds  struct {
			Duplicate float64 `json:"duplicate"`
			Overlap   float64 `json:"overlap"`
		} `json:"thresholds"`
		Clusters []struct {
			ClusterID         string   `json:"cluster_id"`
			ClusterLabel      string   `json:"cluster_label"`
			PRIDs             []int    `json:"pr_ids"`
			HealthStatus      string   `json:"health_status"`
			AverageSimilarity float64  `json:"average_similarity"`
			SampleTitles      []string `json:"sample_titles"`
		} `json:"clusters"`
	}

	if err := json.Unmarshal(clusterData, &clusterResult); err != nil {
		return nil, fmt.Errorf("failed to parse cluster JSON: %w", err)
	}

	// Parse generatedAt timestamp
	if clusterResult.GeneratedAt != "" {
		if t, err := time.Parse(time.RFC3339, clusterResult.GeneratedAt); err == nil {
			section.GeneratedAt = t
		}
	}

	section.Model = clusterResult.Model
	section.Thresholds.Duplicate = clusterResult.Thresholds.Duplicate
	section.Thresholds.Overlap = clusterResult.Thresholds.Overlap

	// Map clusters
	for _, c := range clusterResult.Clusters {
		// Sort PR numbers for deterministic output
		sortedPRs := make([]int, len(c.PRIDs))
		copy(sortedPRs, c.PRIDs)
		sort.Ints(sortedPRs)

		representativeTitle := ""
		if len(c.SampleTitles) > 0 {
			representativeTitle = c.SampleTitles[0]
		}

		section.Clusters = append(section.Clusters, ClusterData{
			ID:                  c.ClusterID,
			Size:                len(c.PRIDs),
			PRNumbers:           sortedPRs,
			RepresentativeTitle: representativeTitle,
			Label:               c.ClusterLabel,
			HealthStatus:        c.HealthStatus,
			AverageSimilarity:   c.AverageSimilarity,
		})
	}

	// Sort clusters by size (largest first), then by ID for determinism
	sort.Slice(section.Clusters, func(i, j int) bool {
		if section.Clusters[i].Size != section.Clusters[j].Size {
			return section.Clusters[i].Size > section.Clusters[j].Size
		}
		return section.Clusters[i].ID < section.Clusters[j].ID
	})

	return section, nil
}

// Ensure ClusterSection implements PDFSection interface
var _ PDFSection = (*ClusterSection)(nil)
