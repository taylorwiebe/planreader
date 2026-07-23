package install

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

type Service struct {
	Home, GOOS, GOARCH, Executable, Version, Origin, CodexHome string
	AllowExternalCodexHome                                     bool
	Skill                                                      fs.FS
}
type Result struct {
	Version, Command string
	PathChanged      bool
	Integrations     []Integration
}

func (s Service) Install() (Result, error) {
	var result Result
	if s.GOOS != "darwin" || s.GOARCH != "arm64" {
		return result, fmt.Errorf("release installation supports Apple-silicon macOS only")
	}
	if s.Home == "" || s.Executable == "" || s.Skill == nil {
		return result, fmt.Errorf("installation configuration is incomplete")
	}
	if s.Version == "" {
		s.Version = "dev"
	}
	if s.Origin == "" {
		s.Origin = "source"
	}
	lock, err := acquireInstallLock(s.Home)
	if err != nil {
		return result, err
	}
	defer releaseInstallLock(lock)
	journal := transactionPath(s.Home)
	if err := recoverTransaction(journal); err != nil {
		return result, err
	}
	p := userPaths(s.Home, s.Version)
	if err := os.MkdirAll(p.versionDir, 0o700); err != nil {
		return result, err
	}
	if err := copyExecutable(s.Executable, p.binary); err != nil {
		return result, err
	}
	if err := copyAdjacentLibraries(s.Executable, p.versionDir); err != nil {
		return result, err
	}
	if err := writeManifest(p.versionDir, manifest{"planreader", s.Version, s.Origin}); err != nil {
		return result, err
	}
	if err := s.rememberExternalCodexHome(); err != nil {
		return result, fmt.Errorf("remembering approved CODEX_HOME: %w", err)
	}
	targets, err := s.agentTargets()
	if err != nil {
		return result, err
	}
	var prepared []preparedSkill
	removeStages := func() {
		for _, item := range prepared {
			_ = os.RemoveAll(item.stage)
		}
	}
	for _, target := range targets {
		skill, integration, err := s.prepareSkill(target)
		if err != nil {
			removeStages()
			return result, fmt.Errorf("preparing managed %s skill update: %w", target.Agent, err)
		}
		if integration.Status != "" {
			result.Integrations = append(result.Integrations, integration)
			continue
		}
		prepared = append(prepared, skill)
	}
	if err := os.MkdirAll(filepath.Dir(p.command), 0o700); err != nil {
		removeStages()
		return result, err
	}
	mutations := make([]mutation, 0, len(prepared)+1)
	for _, skill := range prepared {
		mutations = append(mutations, mutation{Path: skill.target.Path, Backup: skill.target.Path + ".planreader-previous", HadPrevious: skill.hadPrevious})
	}
	_, commandErr := os.Lstat(p.command)
	commandExists := commandErr == nil
	if commandErr != nil && !os.IsNotExist(commandErr) {
		removeStages()
		return result, commandErr
	}
	mutations = append(mutations, mutation{Path: p.command, Backup: p.command + ".planreader-previous", HadPrevious: commandExists})
	// A backup without a journal is debris from a transaction whose commit point
	// was already made durable. Clear it before describing the next transaction.
	for _, mutation := range mutations {
		if err := os.RemoveAll(mutation.Backup); err != nil {
			removeStages()
			return result, err
		}
	}
	previousCommand := ""
	if commandExists {
		previousCommand, _ = os.Readlink(p.command)
	}
	if err := writeTransaction(journal, transaction{PreviousCommand: previousCommand, TargetVersion: s.Version, Mutations: mutations}); err != nil {
		removeStages()
		return result, err
	}
	rollback := func(cause error) (Result, error) {
		removeStages()
		if err := rollbackMutations(mutations); err != nil {
			return result, fmt.Errorf("%w (rollback failed: %v)", cause, err)
		}
		if err := removeTransaction(journal); err != nil {
			return result, fmt.Errorf("%w (clearing rollback journal: %v)", cause, err)
		}
		return result, cause
	}
	for i, skill := range prepared {
		m := mutations[i]
		if m.HadPrevious {
			if err := os.Rename(m.Path, m.Backup); err != nil {
				return rollback(fmt.Errorf("backing up managed %s skill: %w", skill.target.Agent, err))
			}
		}
		if err := os.Rename(skill.stage, m.Path); err != nil {
			if !m.HadPrevious {
				skill.target.Status, skill.target.Detail = "needs attention", err.Error()
				result.Integrations = append(result.Integrations, skill.target)
				_ = os.RemoveAll(skill.stage)
				continue
			}
			return rollback(fmt.Errorf("activating managed %s skill: %w", skill.target.Agent, err))
		}
		skill.target.Status = "installed"
		result.Integrations = append(result.Integrations, skill.target)
	}
	commandMutation := mutations[len(mutations)-1]
	if commandMutation.HadPrevious {
		if err := os.Rename(commandMutation.Path, commandMutation.Backup); err != nil {
			return rollback(err)
		}
	}
	linkStage, err := os.MkdirTemp(filepath.Dir(p.command), ".planreader-link-")
	if err != nil {
		return rollback(err)
	}
	defer os.RemoveAll(linkStage)
	tmpLink := filepath.Join(linkStage, "planreader")
	if err := os.Symlink(p.binary, tmpLink); err != nil {
		return rollback(err)
	}
	if err := os.Rename(tmpLink, p.command); err != nil {
		_ = os.Remove(tmpLink)
		return rollback(err)
	}
	changed, err := ensureZshPath(p.profile)
	if err != nil {
		return rollback(err)
	}
	if err := commitTransaction(journal, mutations); err != nil {
		return rollback(err)
	}
	result.Version, result.Command, result.PathChanged = s.Version, p.command, changed
	return result, nil
}

func copyAdjacentLibraries(executable, versionDir string) error {
	source := filepath.Join(filepath.Dir(executable), "lib")
	entries, err := os.ReadDir(source)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	destination := filepath.Join(versionDir, "lib")
	if err := os.MkdirAll(destination, 0o700); err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".dylib" {
			return fmt.Errorf("unexpected release library %q", entry.Name())
		}
		if err := copyFile(filepath.Join(source, entry.Name()), filepath.Join(destination, entry.Name()), 0o644); err != nil {
			return err
		}
	}
	return nil
}

func copyFile(source, destination string, mode os.FileMode) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.CreateTemp(filepath.Dir(destination), ".planreader-copy-")
	if err != nil {
		return err
	}
	tmp := out.Name()
	if err := out.Chmod(mode); err != nil {
		_ = out.Close()
		_ = os.Remove(tmp)
		return err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		_ = os.Remove(tmp)
		return copyErr
	}
	if closeErr != nil {
		_ = os.Remove(tmp)
		return closeErr
	}
	if err := os.Rename(tmp, destination); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

func copyExecutable(source, destination string) error {
	if err := copyFile(source, destination, 0o755); err != nil {
		return fmt.Errorf("installing executable: %w", err)
	}
	return nil
}
