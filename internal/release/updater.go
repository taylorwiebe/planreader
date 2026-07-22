package release

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

func CompareVersions(a, b string) (int, error) {
	av, e := parseVersion(a)
	if e != nil {
		return 0, e
	}
	bv, e := parseVersion(b)
	if e != nil {
		return 0, e
	}
	for i := range av {
		if av[i] < bv[i] {
			return -1, nil
		}
		if av[i] > bv[i] {
			return 1, nil
		}
	}
	return 0, nil
}
func parseVersion(s string) ([3]int, error) {
	var v [3]int
	s = strings.TrimPrefix(s, "v")
	p := strings.Split(s, ".")
	if len(p) != 3 {
		return v, fmt.Errorf("invalid semantic version %q", s)
	}
	for i, x := range p {
		if x == "" || strings.ContainsAny(x, "+-") {
			return v, fmt.Errorf("invalid semantic version %q", s)
		}
		n, e := strconv.Atoi(x)
		if e != nil || n < 0 {
			return v, fmt.Errorf("invalid semantic version %q", s)
		}
		v[i] = n
	}
	return v, nil
}

type Updater struct {
	Client                                Client
	CurrentVersion, CurrentExecutable     string
	Origin, Home, ExpectedTeamID          string
	ReplaceSource, SkipAppleTrustForTests bool
}

type UpdateResult struct {
	Version string
	Output  string
	Changed bool
}

func (u Updater) Check(ctx context.Context) (Check, error) {
	r, e := u.Client.Resolve(ctx)
	if e != nil {
		return Check{}, e
	}
	return Check{Latest: r.Version}, nil
}
func (u Updater) Update(ctx context.Context) (UpdateResult, error) {
	if u.Origin != "release" && !u.ReplaceSource {
		return UpdateResult{}, errors.New("this is a source build; rerun with --replace-source to install an official release")
	}
	r, e := u.Client.Resolve(ctx)
	if e != nil {
		return UpdateResult{}, e
	}
	if u.Origin == "release" {
		cmp, e := CompareVersions(u.CurrentVersion, r.Version)
		if e != nil {
			return UpdateResult{}, e
		}
		if cmp >= 0 {
			if u.CurrentExecutable == "" {
				return UpdateResult{Version: u.CurrentVersion}, nil
			}
			output, err := u.runInstall(ctx, u.CurrentExecutable)
			return UpdateResult{Version: u.CurrentVersion, Output: output}, err
		}
	}
	base := filepath.Join(u.Home, "Library", "Application Support", "Planreader")
	if e = os.MkdirAll(base, 0700); e != nil {
		return UpdateResult{}, e
	}
	lock := filepath.Join(base, "update.lock")
	lockFile, e := acquireUpdateLock(lock)
	if e != nil {
		return UpdateResult{}, e
	}
	defer releaseUpdateLock(lockFile)
	stage, e := os.MkdirTemp(base, "update-")
	if e != nil {
		return UpdateResult{}, e
	}
	defer os.RemoveAll(stage)
	if e = u.Client.DownloadAndExtract(ctx, r, stage, VerifyOptions{SkipAppleTrustForTests: u.SkipAppleTrustForTests, ExpectedTeamID: u.ExpectedTeamID}); e != nil {
		return UpdateResult{}, e
	}
	output, e := u.runInstall(ctx, filepath.Join(stage, "planreader"))
	if e != nil {
		return UpdateResult{}, e
	}
	return UpdateResult{Version: r.Version, Output: output, Changed: true}, nil
}

func (u Updater) runInstall(ctx context.Context, executable string) (string, error) {
	command := exec.CommandContext(ctx, executable, "install")
	command.Env = append(os.Environ(), "HOME="+u.Home)
	out, e := command.CombinedOutput()
	if e != nil {
		return "", fmt.Errorf("activating release: %s", strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

func acquireUpdateLock(path string) (*os.File, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = file.Close()
		return nil, errors.New("another Planreader update is already running")
	}
	if err := file.Truncate(0); err != nil {
		releaseUpdateLock(file)
		return nil, err
	}
	if _, err := fmt.Fprintln(file, os.Getpid()); err != nil {
		releaseUpdateLock(file)
		return nil, err
	}
	return file, nil
}

func releaseUpdateLock(file *os.File) {
	_ = syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
	_ = file.Close()
}
