package report

import (
	"testing"
	"time"
)

func TestPDFComposer_Compose(t *testing.T) {
	composer := NewPDFComposer("owner/repo", "Scalability Report")

	// Add a cover section
	composer.AddSection(&CoverSection{
		Repo:        "owner/repo",
		Title:       "Scalability Report",
		GeneratedAt: time.Now(),
		Summary:     "This is a test summary for the report.",
	})

	// Add gate context section
	composer.AddSection(&GateContextSection{
		Data: GateContextData{
			TotalPRs:       100,
			ClusterCount:   8,
			DuplicateCount: 3,
			OverlapCount:   5,
			ConflictCount:  15,
			StalePRCount:   12,
			GraphNodes:     45,
			GraphEdges:     67,
			SelectedCount:  20,
			RejectedCount:  80,
			TargetPRs:      20,
			CandidateCount: 100,
		},
		Repo:        "owner/repo",
		GeneratedAt: time.Now(),
	})

	// Add pool composition section
	composer.AddSection(&PoolCompositionSection{
		Data: PoolCompositionData{
			Selected: []CandidateRow{
				{PRNumber: 101, Title: "Add new feature", Cluster: "feature-1", Score: 0.95},
				{PRNumber: 102, Title: "Fix bug in auth", Cluster: "bugfix-1", Score: 0.88},
			},
			Rejected: []RejectedRow{
				{PRNumber: 103, Reason: "Too many conflicts"},
			},
			Clusters: []ClusterRow{
				{ClusterID: "c1", Label: "Feature Work", PRCount: 25, Percentage: 25.0},
				{ClusterID: "c2", Label: "Bug Fixes", PRCount: 15, Percentage: 15.0},
			},
		},
	})

	// Add charts section with placeholders
	composer.AddSection(&ChartsSection{
		Charts: []ChartPlaceholder{
			{Title: "Cluster Overview", Width: 170, Height: 80},
			{Title: "Staleness Distribution", Width: 170, Height: 80},
			{Title: "Merge Plan Composition", Width: 170, Height: 80},
		},
	})

	// Add recommendations section
	composer.AddSection(&RecommendationsSection{
		Recommendations: []RecommendationItem{
			{Priority: "HIGH", Text: "Address 12 stale PRs that are older than 30 days"},
			{Priority: "MEDIUM", Text: "Review duplicate groups to consolidate overlapping work"},
			{Priority: "LOW", Text: "Consider increasing sync frequency during peak hours"},
		},
	})

	// Compose the PDF
	pdfBytes, err := composer.Compose()
	if err != nil {
		t.Fatalf("Compose() returned error: %v", err)
	}

	// Verify PDF bytes were generated
	if len(pdfBytes) == 0 {
		t.Fatal("Compose() returned empty byte slice")
	}

	// Verify PDF starts with expected magic bytes
	if pdfBytes[0] != 0x25 || pdfBytes[1] != 0x50 || pdfBytes[2] != 0x44 || pdfBytes[3] != 0x46 {
		t.Errorf("PDF does not start with %%PDF magic bytes, got: %v", pdfBytes[:4])
	}
}

func TestPDFExporter_Export(t *testing.T) {
	exporter := NewPDFExporter("test/repo", "Test Report")

	exporter.AddSection(&CoverSection{
		Repo:        "test/repo",
		Title:       "Test Report",
		GeneratedAt: time.Now(),
		Summary:     "Minimal test summary.",
	})

	pdfBytes, err := exporter.Export()
	if err != nil {
		t.Fatalf("Export() returned error: %v", err)
	}

	if len(pdfBytes) == 0 {
		t.Fatal("Export() returned empty byte slice")
	}
}

func TestSectionFromPlan(t *testing.T) {
	selected := []CandidateRow{
		{PRNumber: 1, Title: "PR 1", Cluster: "c1", Score: 0.9},
	}
	rejected := []RejectedRow{
		{PRNumber: 2, Reason: "conflict"},
	}
	clusters := []ClusterRow{
		{ClusterID: "c1", Label: "Cluster 1", PRCount: 10, Percentage: 100.0},
	}

	section := SectionFromPlan(selected, rejected, clusters)
	if section == nil {
		t.Fatal("SectionFromPlan returned nil")
	}
	if len(section.Data.Selected) != 1 {
		t.Errorf("expected 1 selected, got %d", len(section.Data.Selected))
	}
	if len(section.Data.Rejected) != 1 {
		t.Errorf("expected 1 rejected, got %d", len(section.Data.Rejected))
	}
	if len(section.Data.Clusters) != 1 {
		t.Errorf("expected 1 cluster, got %d", len(section.Data.Clusters))
	}
}
