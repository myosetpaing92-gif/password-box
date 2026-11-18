package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var version = "dev"

func newVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the application version",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := fmt.Fprintln(cmd.OutOrStdout(), "pbox "+version)
			if err != nil {
				return fmt.Errorf("%w: %w", errOutput, err)
			}

			return nil
		},
	}
}
