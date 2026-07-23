package install

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const transactionName = "install-transaction.json"

type transaction struct {
	PreviousCommand string     `json:"previous_command,omitempty"`
	TargetVersion   string     `json:"target_version"`
	Mutations       []mutation `json:"mutations"`
}

type mutation struct {
	Path        string `json:"path"`
	Backup      string `json:"backup"`
	HadPrevious bool   `json:"had_previous"`
}

func transactionPath(home string) string {
	return filepath.Join(home, "Library", "Application Support", "Planreader", transactionName)
}

func writeTransaction(path string, tx transaction) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(tx, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".planreader-transaction-")
	if err != nil {
		return err
	}
	tmp := f.Name()
	if err := f.Chmod(0o600); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return err
	}
	if _, err = f.Write(append(data, '\n')); err == nil {
		err = f.Sync()
	}
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return syncDir(filepath.Dir(path))
}

func recoverTransaction(path string) error {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("reading interrupted installation journal: %w", err)
	}
	var tx transaction
	if err := json.Unmarshal(data, &tx); err != nil {
		return fmt.Errorf("reading interrupted installation journal: %w", err)
	}
	if err := rollbackMutations(tx.Mutations); err != nil {
		return fmt.Errorf("recovering interrupted installation: %w", err)
	}
	return removeTransaction(path)
}

func rollbackMutations(mutations []mutation) error {
	var errs []error
	for i := len(mutations) - 1; i >= 0; i-- {
		m := mutations[i]
		if m.HadPrevious {
			if _, err := os.Lstat(m.Backup); err == nil {
				if err := os.RemoveAll(m.Path); err != nil {
					errs = append(errs, err)
					continue
				}
				if err := os.Rename(m.Backup, m.Path); err != nil {
					errs = append(errs, err)
				}
			} else if !os.IsNotExist(err) {
				errs = append(errs, err)
			}
			continue
		}
		if err := os.RemoveAll(m.Path); err != nil {
			errs = append(errs, err)
		}
		if err := os.RemoveAll(m.Backup); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func commitTransaction(path string, mutations []mutation) error {
	// Removing the journal is the commit point. Backups remain intact until that
	// durable operation succeeds, so any returned failure is still recoverable.
	if err := removeTransaction(path); err != nil {
		return err
	}
	for _, m := range mutations {
		_ = os.RemoveAll(m.Backup)
	}
	return nil
}

func removeTransaction(path string) error {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return syncDir(filepath.Dir(path))
}

func syncDir(path string) error {
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}
