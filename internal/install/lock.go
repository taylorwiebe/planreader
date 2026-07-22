package install

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
)

func acquireInstallLock(home string) (*os.File, error) {
	dir := filepath.Join(home, "Library", "Application Support", "Planreader")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(filepath.Join(dir, "install.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = file.Close()
		return nil, errors.New("another Planreader install or repair is already running")
	}
	return file, nil
}

func releaseInstallLock(file *os.File) {
	_ = syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
	_ = file.Close()
}
