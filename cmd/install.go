package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"github.com/taylorwiebe/planreader/internal/buildinfo"
	"github.com/taylorwiebe/planreader/internal/install"
	planreaderskills "github.com/taylorwiebe/planreader/skills"
	"io"
	"os"
	"runtime"
)

func newInstallCommand(stdout io.Writer) *cobra.Command {
	var allowExternal bool
	command := &cobra.Command{Use: "install", Short: "Install Planreader for the current user", Args: cobra.NoArgs, RunE: func(_ *cobra.Command, _ []string) error {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		executable, err := os.Executable()
		if err != nil {
			return err
		}
		id := buildinfo.Current()
		result, err := (install.Service{Home: home, GOOS: runtime.GOOS, GOARCH: runtime.GOARCH, Executable: executable, Version: id.Version, Origin: id.Origin, CodexHome: os.Getenv("CODEX_HOME"), AllowExternalCodexHome: allowExternal, Skill: planreaderskills.Files}).Install()
		if err != nil {
			return err
		}
		fmt.Fprintf(stdout, "Planreader %s installed.\nCommand: %s\n", result.Version, result.Command)
		installedSkill := false
		for _, integration := range result.Integrations {
			fmt.Fprintln(stdout, install.FormatIntegration(integration))
			installedSkill = installedSkill || integration.Status == "installed"
		}
		if installedSkill {
			fmt.Fprintln(stdout, "Start a new Claude or Codex session to load the Planreader skill.")
		} else {
			fmt.Fprintln(stdout, "No agent skill was installed. After installing Claude Code or Codex, run planreader install again.")
		}
		if result.PathChanged {
			fmt.Fprintln(stdout, "Added ~/.local/bin to PATH in ~/.zprofile; open a new terminal to use planreader by name.")
		}
		return nil
	}}
	command.Flags().BoolVar(&allowExternal, "allow-external-codex-home", false, "allow skill installation to CODEX_HOME outside the user home")
	return command
}
