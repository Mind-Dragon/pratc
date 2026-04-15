package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jeffersonnunn/pratc/internal/report"
	"github.com/spf13/cobra"
)

func RegisterReportCommand() {
	var (
		inputDir   string
		outputPath string
		skipReview bool
		skipCharts bool
	)

	cmd := &cobra.Command{
		Use:   "report",
		Short: "Generate PDF report",
		Long: `Generate a PDF report from workflow artifacts.
Takes a workflow output directory (containing analyze.json, step-3-cluster.json,
step-4-graph.json, step-5-plan.json) and produces a formatted PDF.

Example:
  pratc report --repo owner/repo --input-dir ~/.pratc/workflows/owner_repo/20260101-120000 --output report.pdf`,
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, _ := cmd.Flags().GetString("repo")
			if repo == "" {
				return fmt.Errorf("--repo is required")
			}

			if outputPath == "" {
				outputPath = "report.pdf"
			}

			// Resolve input directory
			resolvedInputDir := strings.TrimSpace(inputDir)
			if resolvedInputDir == "" {
				resolvedInputDir = defaultReportInputDir(repo)
			}

			// Validate input directory exists
			if _, err := os.Stat(resolvedInputDir); os.IsNotExist(err) {
				return fmt.Errorf("input directory does not exist: %s\nRun 'pratc workflow --repo %s --out-dir <dir>' first", resolvedInputDir, repo)
			}

			fmt.Fprintf(cmd.ErrOrStderr(), "Loading artifacts from: %s\n", resolvedInputDir)

			// Build the PDF exporter
			exporter := report.NewPDFExporter(repo, fmt.Sprintf("PR Analysis Report: %s", repo))

			// Add cover section
			exporter.AddSection(&report.CoverSection{
				Repo:        repo,
				Title:       fmt.Sprintf("PR Analysis Report: %s", repo),
				GeneratedAt: time.Now(),
				Summary:     fmt.Sprintf("Automated PR analysis and merge planning report for %s", repo),
			})

			// Load summary section
			if summary, err := report.LoadSummarySection(resolvedInputDir, repo); err == nil {
				exporter.AddSection(summary)
			} else {
				fmt.Fprintf(cmd.ErrOrStderr(), "Warning: could not load summary section: %v\n", err)
			}

			// Load metrics section
			if metrics, err := report.LoadMetricsSection(resolvedInputDir, repo); err == nil {
				exporter.AddSection(metrics)
			} else {
				fmt.Fprintf(cmd.ErrOrStderr(), "Warning: could not load metrics section: %v\n", err)
			}

			// Load cluster section
			if cluster, err := report.LoadClusterSection(resolvedInputDir, repo); err == nil {
				exporter.AddSection(cluster)
			} else {
				fmt.Fprintf(cmd.ErrOrStderr(), "Warning: could not load cluster section: %v\n", err)
			}

			// Load graph section
			if graph, err := report.LoadGraphSection(resolvedInputDir, repo); err == nil {
				exporter.AddSection(graph)
			} else {
				fmt.Fprintf(cmd.ErrOrStderr(), "Warning: could not load graph section: %v\n", err)
			}

			// Load plan section
			if plan, err := report.LoadPlanSection(resolvedInputDir, repo); err == nil {
				exporter.AddSection(plan)
			} else {
				fmt.Fprintf(cmd.ErrOrStderr(), "Warning: could not load plan section: %v\n", err)
			}

			// Review + analyst packet sections.
			if !skipReview {
				if review, err := report.LoadReviewSection(resolvedInputDir, repo); err == nil {
					exporter.AddSection(review)
				} else {
					fmt.Fprintf(cmd.ErrOrStderr(), "Warning: could not load review section: %v\n", err)
				}
			}
			if analystSummary, err := report.LoadAnalystSummarySection(resolvedInputDir, repo); err == nil {
				exporter.AddSection(analystSummary)
			} else {
				fmt.Fprintf(cmd.ErrOrStderr(), "Warning: could not load analyst summary section: %v\n", err)
			}
			if decisionTrail, err := report.LoadDecisionTrailSection(resolvedInputDir, repo); err == nil {
				exporter.AddSection(decisionTrail)
			} else {
				fmt.Fprintf(cmd.ErrOrStderr(), "Warning: could not load decision trail section: %v\n", err)
			}
			if fullTable, err := report.LoadFullPRTableSection(resolvedInputDir, repo); err == nil {
				exporter.AddSection(fullTable)
			} else {
				fmt.Fprintf(cmd.ErrOrStderr(), "Warning: could not load full PR table section: %v\n", err)
			}
			if duplicates, err := report.LoadDuplicateDetailSection(resolvedInputDir, repo); err == nil {
				exporter.AddSection(duplicates)
			} else {
				fmt.Fprintf(cmd.ErrOrStderr(), "Warning: could not load duplicate detail section: %v\n", err)
			}
			if junk, err := report.LoadSpamJunkSection(resolvedInputDir, repo); err == nil {
				exporter.AddSection(junk)
			} else {
				fmt.Fprintf(cmd.ErrOrStderr(), "Warning: could not load junk section: %v\n", err)
			}
			if recs, err := report.LoadAnalystRecommendationsSection(resolvedInputDir, repo); err == nil {
				exporter.AddSection(recs)
			} else {
				fmt.Fprintf(cmd.ErrOrStderr(), "Warning: could not load recommendations section: %v\n", err)
			}

			// Legacy charts remain optional and disabled by default via --skip-charts.
			if !skipCharts {
				exporter.AddSection(&report.ChartsSection{
					Charts: []report.ChartPlaceholder{
						{Title: "PR Volume Over Time", Width: 180, Height: 60},
						{Title: "Cluster Size Distribution", Width: 180, Height: 60},
						{Title: "Conflict Density Map", Width: 180, Height: 60},
					},
				})
			}

			// Generate the PDF
			pdfBytes, err := exporter.Export()
			if err != nil {
				return fmt.Errorf("failed to generate PDF: %w", err)
			}

			// Ensure output directory exists
			outputAbs, err := filepath.Abs(outputPath)
			if err != nil {
				outputAbs = outputPath
			}
			if err := os.MkdirAll(filepath.Dir(outputAbs), 0o755); err != nil {
				return fmt.Errorf("failed to create output directory: %w", err)
			}

			// Write the PDF
			if err := os.WriteFile(outputAbs, pdfBytes, 0o644); err != nil {
				return fmt.Errorf("failed to write PDF file: %w", err)
			}

			fmt.Fprintf(cmd.ErrOrStderr(), "PDF report written to: %s (%d bytes)\n", outputAbs, len(pdfBytes))
			return nil
		},
	}

	cmd.Flags().String("repo", "", "Repository (owner/name) [required]")
	cmd.Flags().StringVar(&inputDir, "input-dir", "", "Directory containing workflow artifacts")
	cmd.Flags().StringVar(&outputPath, "output", "report.pdf", "Output PDF file path")
	cmd.Flags().BoolVar(&skipReview, "skip-review", false, "Skip review section")
	cmd.Flags().BoolVar(&skipCharts, "skip-charts", false, "Skip charts and recommendations sections")
	_ = cmd.MarkFlagRequired("repo")

	rootCmd.AddCommand(cmd)
}

// defaultReportInputDir returns the most recent workflow output directory for a repo.
func defaultReportInputDir(repo string) string {
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		home = "."
	}
	slug := strings.NewReplacer("/", "_", string(os.PathSeparator), "_", " ", "_").Replace(strings.TrimSpace(repo))
	if slug == "" {
		slug = "repo"
	}
	workflowsBase := filepath.Join(home, ".pratc", "workflows", slug)

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
