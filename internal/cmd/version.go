package cmd

import (
	"fmt"

	"github.com/jeffersonnunn/pratc/internal/version"
	"github.com/spf13/cobra"
)

func RegisterVersionCommand() {
	registerVersionCommand(rootCmd)
}

func registerVersionCommand(root *cobra.Command) {
	root.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print build provenance",
		RunE: func(cmd *cobra.Command, args []string) error {
			return writeVersion(cmd)
		},
	})
}

func writeVersion(cmd *cobra.Command) error {
	_, err := fmt.Fprintf(
		cmd.OutOrStdout(),
		"version=%s\ncommit=%s\nbuild_date=%s\ndirty=%s\n",
		version.Version,
		version.Commit,
		version.BuildDate,
		version.Dirty,
	)
	return err
}
