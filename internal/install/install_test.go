package install

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

var testSkill = fstest.MapFS{
	"read-with-planreader/SKILL.md":           &fstest.MapFile{Data: []byte("skill")},
	"read-with-planreader/agents/openai.yaml": &fstest.MapFile{Data: []byte("agent")},
}

func TestInstallConfiguresDetectedAgentsAndPathIdempotently(t *testing.T) {
	home := t.TempDir()
	for _, dir := range []string{".claude", ".codex"} {
		if err := os.Mkdir(filepath.Join(home, dir), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	exe := filepath.Join(t.TempDir(), "planreader")
	if err := os.WriteFile(exe, []byte("binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	svc := Service{Home: home, GOOS: "darwin", GOARCH: "arm64", Executable: exe, Version: "1.2.3", Origin: "release", Skill: testSkill}
	for i := 0; i < 2; i++ {
		result, err := svc.Install()
		if err != nil {
			t.Fatal(err)
		}
		if len(result.Integrations) != 2 {
			t.Fatalf("integrations = %#v", result.Integrations)
		}
	}
	link := filepath.Join(home, ".local/bin/planreader")
	target, err := os.Readlink(link)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(target, "versions/1.2.3/planreader") {
		t.Fatalf("target = %q", target)
	}
	profile, err := os.ReadFile(filepath.Join(home, ".zprofile"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(profile), pathBlockStart) != 1 {
		t.Fatalf("profile = %q", profile)
	}
	for _, path := range []string{
		filepath.Join(home, ".claude/skills/read-with-planreader", manifestName),
		filepath.Join(home, ".codex/skills/read-with-planreader", manifestName),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Errorf("managed skill %s: %v", path, err)
		}
	}
}

func TestInstallRefusesUnmanagedSkillCollision(t *testing.T) {
	home := t.TempDir()
	collision := filepath.Join(home, ".claude/skills/read-with-planreader")
	if err := os.MkdirAll(collision, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(collision, "personal.txt"), []byte("mine"), 0o600); err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(t.TempDir(), "planreader")
	if err := os.WriteFile(exe, []byte("binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	result, err := (Service{Home: home, GOOS: "darwin", GOARCH: "arm64", Executable: exe, Version: "dev", Origin: "source", Skill: testSkill}).Install()
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Integrations) != 2 || result.Integrations[0].Status != "needs attention" || result.Integrations[1].Status != "not detected" {
		t.Fatalf("result = %#v", result)
	}
	if _, err := os.Stat(filepath.Join(collision, "personal.txt")); err != nil {
		t.Fatal("collision was overwritten")
	}
}

func TestInstallRejectsUnsupportedPlatformBeforeWriting(t *testing.T) {
	home := t.TempDir()
	_, err := (Service{Home: home, GOOS: "linux", GOARCH: "arm64", Executable: "ignored", Skill: testSkill}).Install()
	if err == nil || !strings.Contains(err.Error(), "Apple-silicon macOS") {
		t.Fatalf("error = %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(home, ".local")); !os.IsNotExist(statErr) {
		t.Fatalf("partial install: %v", statErr)
	}
}

func TestInstallRecoversInterruptedSkillAndCommandTransaction(t *testing.T) {
	home := t.TempDir()
	skill := filepath.Join(home, ".claude/skills/read-with-planreader")
	skillBackup := skill + ".planreader-previous"
	if err := os.MkdirAll(skill, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skill, "state"), []byte("new"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(skillBackup, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillBackup, "state"), []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}

	command := filepath.Join(home, ".local/bin/planreader")
	commandBackup := command + ".planreader-previous"
	if err := os.MkdirAll(filepath.Dir(command), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("new-binary", command); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("old-binary", commandBackup); err != nil {
		t.Fatal(err)
	}

	journal := transactionPath(home)
	if err := writeTransaction(journal, transaction{Mutations: []mutation{
		{Path: skill, Backup: skillBackup, HadPrevious: true},
		{Path: command, Backup: commandBackup, HadPrevious: true},
	}}); err != nil {
		t.Fatal(err)
	}

	_, err := (Service{Home: home, GOOS: "darwin", GOARCH: "arm64", Executable: filepath.Join(home, "missing"), Skill: testSkill}).Install()
	if err == nil {
		t.Fatal("expected installation to fail after recovery")
	}
	data, err := os.ReadFile(filepath.Join(skill, "state"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "old" {
		t.Fatalf("skill state = %q", data)
	}
	target, err := os.Readlink(command)
	if err != nil {
		t.Fatal(err)
	}
	if target != "old-binary" {
		t.Fatalf("command target = %q", target)
	}
	if _, err := os.Stat(journal); !os.IsNotExist(err) {
		t.Fatalf("journal still exists: %v", err)
	}
}

func TestInstallTreatsManagedSkillPreparationFailureAsFatal(t *testing.T) {
	home := t.TempDir()
	skill := filepath.Join(home, ".claude/skills/read-with-planreader")
	if err := os.MkdirAll(skill, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := writeManifest(skill, manifest{"planreader", "1.0.0", "release"}); err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(t.TempDir(), "planreader")
	if err := os.WriteFile(exe, []byte("binary"), 0o755); err != nil {
		t.Fatal(err)
	}

	_, err := (Service{
		Home: home, GOOS: "darwin", GOARCH: "arm64", Executable: exe,
		Version: "2.0.0", Origin: "release", Skill: fstest.MapFS{},
	}).Install()
	if err == nil || !strings.Contains(err.Error(), "preparing managed Claude skill update") {
		t.Fatalf("error = %v", err)
	}
	if !managed(skill) {
		t.Fatal("previous managed skill was removed")
	}
	if _, err := os.Lstat(filepath.Join(home, ".local/bin/planreader")); !os.IsNotExist(err) {
		t.Fatalf("command activated despite fatal skill failure: %v", err)
	}
}

func TestAgentTargetsResolveCodexHomeSymlinks(t *testing.T) {
	home := t.TempDir()
	external := t.TempDir()
	if err := os.Mkdir(filepath.Join(external, "codex"), 0o700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(home, "linked-codex")
	if err := os.Symlink(filepath.Join(external, "codex"), link); err != nil {
		t.Fatal(err)
	}
	targets, err := (Service{Home: home, CodexHome: link}).agentTargets()
	if err != nil {
		t.Fatal(err)
	}
	if len(targets) != 2 || targets[0].Status != "not detected" || targets[1].Status != "needs attention" {
		t.Fatalf("targets = %#v", targets)
	}
}

func TestExternalCodexHomeApprovalPersists(t *testing.T) {
	home := t.TempDir()
	codex := filepath.Join(t.TempDir(), "codex")
	if err := os.Mkdir(codex, 0o700); err != nil {
		t.Fatal(err)
	}
	approved := Service{Home: home, CodexHome: codex, AllowExternalCodexHome: true}
	if err := approved.rememberExternalCodexHome(); err != nil {
		t.Fatal(err)
	}
	targets, err := (Service{Home: home, CodexHome: codex}).agentTargets()
	if err != nil {
		t.Fatal(err)
	}
	if len(targets) != 2 || targets[0].Status != "not detected" || targets[1].Status != "" {
		t.Fatalf("targets = %#v", targets)
	}
}

func TestEnsureZshPathUsesUnpredictableTemporaryFileAndPreservesMode(t *testing.T) {
	dir := t.TempDir()
	profile := filepath.Join(dir, ".zprofile")
	decoy := profile + ".planreader.tmp"
	target := filepath.Join(dir, "do-not-touch")
	if err := os.WriteFile(profile, []byte("export EDITOR=vi\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("safe"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, decoy); err != nil {
		t.Fatal(err)
	}
	if _, err := ensureZshPath(profile); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(target)
	if err != nil || string(data) != "safe" {
		t.Fatalf("decoy target = %q, %v", data, err)
	}
	info, err := os.Stat(profile)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o640 {
		t.Fatalf("profile mode = %o", info.Mode().Perm())
	}
}

func TestInstallLockRejectsConcurrentOwner(t *testing.T) {
	home := t.TempDir()
	first, err := acquireInstallLock(home)
	if err != nil {
		t.Fatal(err)
	}
	defer releaseInstallLock(first)
	if second, err := acquireInstallLock(home); err == nil {
		releaseInstallLock(second)
		t.Fatal("expected concurrent install lock error")
	}
}
