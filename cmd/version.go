package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"github.com/taylorwiebe/planreader/internal/buildinfo"
	"io"
)

func newVersionCommand(stdout io.Writer) *cobra.Command {
	return &cobra.Command{Use: "version", Short: "Show Planreader build identity", Args: cobra.NoArgs, RunE: func(_ *cobra.Command, _ []string) error {
		id := buildinfo.Current()
		_, err := fmt.Fprintf(stdout, "Planreader %s (%s, commit %s)\n", id.Version, id.Origin, id.Commit)
		return err
	}}
}
