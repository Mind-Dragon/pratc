package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jeffersonnunn/pratc/internal/report"
	"github.com/jeffersonnunn/pratc/internal/types"
	"github.com/spf13/cobra"
)

func RegisterReportCommand() {
	var (
		inputDir   string
		outputPath string
		skipReview bool
		skipCharts bool = true // charts are placeholder-only; skip by default
	)

	cmd := &cobra.Command{
		Use:   "report",
		Short: "Generate PDF report",
		Long: `Generate a PDF report from workflow artifacts.
Takes a workflow output directory (containing analyze.json, step-3-cluster.json,
step-4-graph.json, step-5-plan.json) and produces a formatted PDF.

Example:
	  pratc report --repo owner/repo --input-dir ./projects/owner_repo/runs/20260101-120000 --output report.pdf`,
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, _ := cmd.Flags().GetString("repo")
			if repo == "" {
				return fmt.Errorf("--repo is required")
			}
			repo = types.NormalizeRepoName(repo)

			return generatePDFReport(repo, inputDir, outputPath, skipReview, skipCharts, cmd.ErrOrStderr())
		},
	}

	cmd.Flags().String("repo", "", "Repository (owner/name) [required]")
	cmd.Flags().StringVar(&inputDir, "input-dir", "", "Directory containing workflow artifacts (defaults to projects/<repo>/runs/<timestamp>)")
	cmd.Flags().StringVar(&outputPath, "output", "report.pdf", "Output PDF file path")
	cmd.Flags().BoolVar(&skipReview, "skip-review", false, "Skip review section")
	cmd.Flags().BoolVar(&skipCharts, "skip-charts", true, "Skip placeholder-only charts by default")
	_ = cmd.MarkFlagRequired("repo")

	rootCmd.AddCommand(cmd)
}

func generatePDFReport(repo, inputDir, outputPath string, skipReview, skipCharts bool, stderr io.Writer) error {
	if outputPath == "" {
		outputPath = "report.pdf"
	}

	resolvedInputDir := strings.TrimSpace(inputDir)
	if resolvedInputDir == "" {
		resolvedInputDir = defaultReportInputDir(repo)
	}
	if _, err := os.Stat(resolvedInputDir); os.IsNotExist(err) {
		return fmt.Errorf("input directory does not exist: %s\nRun 'pratc workflow --repo %s --out-dir <dir>' first", resolvedInputDir, repo)
	}

	fmt.Fprintf(stderr, "Loading artifacts from: %s\n", resolvedInputDir)

	exporter := report.NewPDFExporter(repo, fmt.Sprintf("PR Analysis Report: %s", repo))

	analyzeTimestamp := time.Now()
	if analyzeData, err := report.ReadAnalyzeArtifact(resolvedInputDir); err == nil {
		var analyze types.AnalysisResponse
		if err := json.Unmarshal(analyzeData, &analyze); err == nil && analyze.GeneratedAt != "" {
			if t, err := time.Parse(time.RFC3339, analyze.GeneratedAt); err == nil {
				analyzeTimestamp = t
			}
		}
	}

	exporter.AddSection(&report.CoverSection{
		Repo:        repo,
		Title:       fmt.Sprintf("PR Analysis Report: %s", repo),
		GeneratedAt: analyzeTimestamp,
		Summary:     fmt.Sprintf("Automated PR analysis and merge planning report for %s", repo),
		CacheNote:   "Cluster, graph, and plan steps used --force-cache and may reflect a subset of the full corpus. Analyze step covers all PRs in the snapshot.",
	})

	if summary, err := report.LoadSummarySection(resolvedInputDir, repo); err == nil {
		exporter.AddSection(summary)
	} else {
		fmt.Fprintf(stderr, "Warning: could not load summary section: %v\n", err)
	}
	if metrics, err := report.LoadMetricsSection(resolvedInputDir, repo); err == nil {
		exporter.AddSection(metrics)
	} else {
		fmt.Fprintf(stderr, "Warning: could not load metrics section: %v\n", err)
	}
	if analystSummary, err := report.LoadAnalystSummarySection(resolvedInputDir, repo); err == nil {
		exporter.AddSection(analystSummary)
	} else {
		fmt.Fprintf(stderr, "Warning: could not load analyst summary section: %v\n", err)
	}
	if junk, err := report.LoadSpamJunkSection(resolvedInputDir, repo); err == nil {
		exporter.AddSection(junk)
	} else {
		fmt.Fprintf(stderr, "Warning: could not load junk section: %v\n", err)
	}
	if duplicates, err := report.LoadDuplicateDetailSection(resolvedInputDir, repo); err == nil {
		exporter.AddSection(duplicates)
	} else {
		fmt.Fprintf(stderr, "Warning: could not load duplicate detail section: %v\n", err)
	}
	if nearDups, err := report.LoadNearDuplicateDetailSection(resolvedInputDir, repo); err == nil {
		exporter.AddSection(nearDups)
	} else {
		fmt.Fprintf(stderr, "Warning: could not load near-duplicate detail section: %v\n", err)
	}
	if !skipReview {
		if review, err := report.LoadReviewSection(resolvedInputDir, repo); err == nil {
			exporter.AddSection(review)
		} else {
			fmt.Fprintf(stderr, "Warning: could not load review section: %v\n", err)
		}
	}
	if decisionTrail, err := report.LoadDecisionTrailSection(resolvedInputDir, repo); err == nil {
		exporter.AddSection(decisionTrail)
	} else {
		fmt.Fprintf(stderr, "Warning: could not load decision trail section: %v\n", err)
	}
	if recs, err := report.LoadAnalystRecommendationsSection(resolvedInputDir, repo); err == nil {
		exporter.AddSection(recs)
	} else {
		fmt.Fprintf(stderr, "Warning: could not load recommendations section: %v\n", err)
	}
	if cluster, err := report.LoadClusterSection(resolvedInputDir, repo); err == nil {
		exporter.AddSection(cluster)
	} else {
		fmt.Fprintf(stderr, "Warning: could not load cluster section: %v\n", err)
	}
	if graph, err := report.LoadGraphSection(resolvedInputDir, repo); err == nil {
		exporter.AddSection(graph)
	} else {
		fmt.Fprintf(stderr, "Warning: could not load graph section: %v\n", err)
	}
	if plan, err := report.LoadPlanSection(resolvedInputDir, repo); err == nil {
		exporter.AddSection(plan)
	} else {
		fmt.Fprintf(stderr, "Warning: could not load plan section: %v\n", err)
	}
	if fullTable, err := report.LoadFullPRTableSection(resolvedInputDir, repo); err == nil {
		exporter.AddSection(fullTable)
	} else {
		fmt.Fprintf(stderr, "Warning: could not load full PR table section: %v\n", err)
	}
	if !skipCharts {
		exporter.AddSection(&report.ChartsSection{
			Charts: []report.ChartPlaceholder{
				{Title: "PR Volume Over Time", Width: 180, Height: 60},
				{Title: "Cluster Size Distribution", Width: 180, Height: 60},
				{Title: "Conflict Density Map", Width: 180, Height: 60},
			},
		})
	}

	pdfBytes, err := exporter.Export()
	if err != nil {
		return fmt.Errorf("failed to generate PDF: %w", err)
	}

	outputAbs, err := filepath.Abs(outputPath)
	if err != nil {
		outputAbs = outputPath
	}
	if err := os.MkdirAll(filepath.Dir(outputAbs), 0o755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}
	if err := os.WriteFile(outputAbs, pdfBytes, 0o644); err != nil {
		return fmt.Errorf("failed to write PDF file: %w", err)
	}

	fmt.Fprintf(stderr, "PDF report written to: %s (%d bytes)\n", outputAbs, len(pdfBytes))
	return nil
}

// defaultReportInputDir returns the most recent workflow output directory for a repo.
func defaultReportInputDir(repo string) string {
	workflowsBase := projectRunsDir(repo)

	entries, err := os.ReadDir(workflowsBase)
	if err != nil {
		return workflowsBase
	}

	var latest string
	var latestTime int64
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if info.ModTime().UnixNano() > latestTime {
			latestTime = info.ModTime().UnixNano()
			latest = filepath.Join(workflowsBase, entry.Name())
		}
	}

	if latest != "" {
		return latest
	}
	return workflowsBase
}
