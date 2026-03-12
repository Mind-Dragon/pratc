package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "pratc",
	Short: "PR Air Traffic Control",
	Long:  "prATC is a CLI for pull request analysis, clustering, graphing, planning, and API serving.",
}

func Execute() {
	cobra.CheckErr(rootCmd.Execute())
}

func RegisterAnalyzeCommand() {
	var repo string

	command := &cobra.Command{
		Use:   "analyze",
		Short: "Analyze pull requests for a repository",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("analyze placeholder for repo %s\n", repo)
		},
	}
	command.PersistentFlags().StringVar(&repo, "repo", "", "Repository in owner/repo format")
	_ = command.MarkFlagRequired("repo")
	rootCmd.AddCommand(command)
}

func RegisterClusterCommand() {
	var repo string

	command := &cobra.Command{
		Use:   "cluster",
		Short: "Cluster pull requests for a repository",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("cluster placeholder for repo %s\n", repo)
		},
	}
	command.PersistentFlags().StringVar(&repo, "repo", "", "Repository in owner/repo format")
	_ = command.MarkFlagRequired("repo")
	rootCmd.AddCommand(command)
}

func RegisterGraphCommand() {
	var repo string

	command := &cobra.Command{
		Use:   "graph",
		Short: "Render a dependency graph for a repository",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("graph placeholder for repo %s\n", repo)
		},
	}
	command.PersistentFlags().StringVar(&repo, "repo", "", "Repository in owner/repo format")
	_ = command.MarkFlagRequired("repo")
	rootCmd.AddCommand(command)
}

func RegisterPlanCommand() {
	var repo string

	command := &cobra.Command{
		Use:   "plan",
		Short: "Generate a merge plan for a repository",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("plan placeholder for repo %s\n", repo)
		},
	}
	command.PersistentFlags().StringVar(&repo, "repo", "", "Repository in owner/repo format")
	_ = command.MarkFlagRequired("repo")
	rootCmd.AddCommand(command)
}

func RegisterServeCommand() {
	var port int

	command := &cobra.Command{
		Use:   "serve",
		Short: "Serve the prATC API",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("serve placeholder on port %d\n", port)
		},
	}
	command.Flags().IntVar(&port, "port", 8080, "Port to bind the API server to")
	rootCmd.AddCommand(command)
}
