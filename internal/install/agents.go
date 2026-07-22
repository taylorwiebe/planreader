package install

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

type Integration struct{ Agent, Status, Path, Detail string }

func (s Service) agentTargets() ([]Integration, error) {
	var targets []Integration
	claudeRoot := filepath.Join(s.Home, ".claude")
	if exists(claudeRoot) {
		targets = append(targets, Integration{Agent: "Claude", Path: filepath.Join(claudeRoot, "skills", "read-with-planreader")})
	} else {
		targets = append(targets, Integration{Agent: "Claude", Status: "not detected", Path: claudeRoot, Detail: "install Claude Code, then run planreader install again"})
	}
	codexRoot := s.CodexHome
	if codexRoot == "" {
		codexRoot = filepath.Join(s.Home, ".codex")
	}
	if exists(codexRoot) {
		home, err := canonicalPath(s.Home)
		if err != nil {
			return nil, err
		}
		codex, err := canonicalPath(codexRoot)
		if err != nil {
			return nil, err
		}
		inside := codex == home || strings.HasPrefix(codex, home+string(os.PathSeparator))
		approved := s.AllowExternalCodexHome || s.externalCodexHomeApproved(codex)
		integration := Integration{Agent: "Codex", Path: filepath.Join(codexRoot, "skills", "read-with-planreader")}
		if !inside && !approved {
			integration.Status, integration.Detail = "needs attention", "CODEX_HOME is outside the user home; rerun with explicit approval"
		}
		targets = append(targets, integration)
	} else {
		targets = append(targets, Integration{Agent: "Codex", Status: "not detected", Path: codexRoot, Detail: "install Codex, then run planreader install again"})
	}
	return targets, nil
}

func (s Service) externalApprovalPath() string {
	return filepath.Join(s.Home, "Library", "Application Support", "Planreader", "approved-codex-home")
}

func (s Service) rememberExternalCodexHome() error {
	if !s.AllowExternalCodexHome || s.CodexHome == "" {
		return nil
	}
	resolved, err := canonicalPath(s.CodexHome)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.externalApprovalPath()), 0o700); err != nil {
		return err
	}
	return os.WriteFile(s.externalApprovalPath(), []byte(resolved+"\n"), 0o600)
}

func (s Service) externalCodexHomeApproved(resolved string) bool {
	data, err := os.ReadFile(s.externalApprovalPath())
	return err == nil && strings.TrimSpace(string(data)) == resolved
}

func canonicalPath(name string) (string, error) {
	abs, err := filepath.Abs(name)
	if err != nil {
		return "", err
	}
	probe := abs
	var suffix []string
	for {
		resolved, resolveErr := filepath.EvalSymlinks(probe)
		if resolveErr == nil {
			for i := len(suffix) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, suffix[i])
			}
			return filepath.Clean(resolved), nil
		}
		parent := filepath.Dir(probe)
		if parent == probe {
			return "", resolveErr
		}
		suffix = append(suffix, filepath.Base(probe))
		probe = parent
	}
}

type preparedSkill struct {
	target      Integration
	stage       string
	hadPrevious bool
}

func (s Service) prepareSkill(target Integration) (preparedSkill, Integration, error) {
	if target.Status != "" {
		return preparedSkill{}, target, nil
	}
	hadPrevious := false
	if info, err := os.Lstat(target.Path); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			target.Status, target.Detail = "needs attention", "skill destination is a symlink"
			return preparedSkill{}, target, nil
		}
		if !managed(target.Path) {
			target.Status, target.Detail = "needs attention", "a same-named skill exists and is not managed by Planreader"
			return preparedSkill{}, target, nil
		}
		hadPrevious = true
	} else if !os.IsNotExist(err) {
		target.Status, target.Detail = "needs attention", err.Error()
		return preparedSkill{}, target, nil
	}
	parent := filepath.Dir(target.Path)
	if err := os.MkdirAll(parent, 0o700); err != nil {
		target.Status, target.Detail = "needs attention", err.Error()
		if hadPrevious {
			return preparedSkill{}, target, err
		}
		return preparedSkill{}, target, nil
	}
	stage, err := os.MkdirTemp(parent, ".read-with-planreader-stage-")
	if err != nil {
		target.Status, target.Detail = "needs attention", err.Error()
		if hadPrevious {
			return preparedSkill{}, target, err
		}
		return preparedSkill{}, target, nil
	}
	err = fs.WalkDir(s.Skill, "read-with-planreader", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel("read-with-planreader", path)
		if err != nil || rel == "." {
			return err
		}
		destination := filepath.Join(stage, rel)
		if entry.IsDir() {
			return os.MkdirAll(destination, 0o700)
		}
		data, err := fs.ReadFile(s.Skill, path)
		if err != nil {
			return err
		}
		return os.WriteFile(destination, data, 0o600)
	})
	if err == nil {
		err = writeManifest(stage, manifest{"planreader", s.Version, s.Origin})
	}
	if err != nil {
		_ = os.RemoveAll(stage)
		target.Status, target.Detail = "needs attention", err.Error()
		if hadPrevious {
			return preparedSkill{}, target, err
		}
		return preparedSkill{}, target, nil
	}
	return preparedSkill{target: target, stage: stage, hadPrevious: hadPrevious}, Integration{}, nil
}

func exists(path string) bool { _, err := os.Stat(path); return err == nil }
func FormatIntegration(in Integration) string {
	if in.Detail == "" {
		return fmt.Sprintf("%s: %s (%s)", in.Agent, in.Status, in.Path)
	}
	return fmt.Sprintf("%s: %s — %s (%s)", in.Agent, in.Status, in.Detail, in.Path)
}
