package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/taylorwiebe/planreader/internal/buildinfo"
	"github.com/taylorwiebe/planreader/internal/release"
)

func newUpdateCommand(stdout io.Writer) *cobra.Command {
	var check, replaceSource bool
	command := &cobra.Command{
		Use:   "update",
		Short: "Check for or install a verified Planreader release",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			home, err := os.UserHomeDir()
			if err != nil {
				return err
			}
			id := buildinfo.Current()
			executable, err := os.Executable()
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			u := release.Updater{Client: release.Client{}, CurrentVersion: id.Version, CurrentExecutable: executable, Origin: id.Origin, Home: home, ExpectedTeamID: buildinfo.TeamID, ReplaceSource: replaceSource}
			if check {
				result, err := u.Check(ctx)
				if err != nil {
					return err
				}
				if id.Origin != "release" {
					fmt.Fprintf(stdout, "Planreader is a source build; latest official release: %s. Use update --replace-source to switch.\n", result.Latest)
					return nil
				}
				comparison, err := release.CompareVersions(id.Version, result.Latest)
				if err != nil {
					return err
				}
				if comparison < 0 {
					fmt.Fprintf(stdout, "Planreader %s is available (current: %s).\n", result.Latest, id.Version)
				} else {
					fmt.Fprintf(stdout, "Planreader %s is up to date.\n", id.Version)
				}
				return nil
			}
			result, err := u.Update(ctx)
			if err != nil {
				return err
			}
			if result.Output != "" {
				fmt.Fprintln(stdout, result.Output)
			}
			if !result.Changed {
				fmt.Fprintf(stdout, "Planreader %s is already up to date; integrations were checked.\n", result.Version)
			} else {
				fmt.Fprintf(stdout, "Updated Planreader to %s. Start a new Claude or Codex session to load the updated skill.\n", result.Version)
			}
			return nil
		},
	}
	command.Flags().BoolVar(&check, "check", false, "check for an update without installing it")
	command.Flags().BoolVar(&replaceSource, "replace-source", false, "replace a source build with the official release")
	return command
}
