package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// RegisterReportCommand registers the report command with the root command
func RegisterReportCommand() {
	cmd := &cobra.Command{
		Use:   "report",
		Short: "Generate PDF report",
		Long:  "Generate a PDF report from analysis results.",
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, _ := cmd.Flags().GetString("repo")
			if repo == "" {
				return fmt.Errorf("--repo is required")
			}

			output, _ := cmd.Flags().GetString("output")
			if output == "" {
				output = "report.pdf"
			}

			// TODO: Implement report generation via internal/report package
			fmt.Fprintf(os.Stderr, "Report generation not yet implemented. Output would be: %s\n", output)
			return nil
		},
	}

	cmd.Flags().String("repo", "", "Repository (owner/name)")
	cmd.Flags().String("output", "report.pdf", "Output PDF file path")
	_ = cmd.MarkFlagRequired("repo")

	rootCmd.AddCommand(cmd)
}
