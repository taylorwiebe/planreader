package install

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const pathBlockStart = "# >>> Planreader managed PATH >>>"
const pathBlockEnd = "# <<< Planreader managed PATH <<<"

func ensureZshPath(profile string) (bool, error) {
	if info, err := os.Lstat(profile); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return false, fmt.Errorf("refusing to edit symlinked zsh profile %s", profile)
	} else if err != nil && !os.IsNotExist(err) {
		return false, err
	}
	data, err := os.ReadFile(profile)
	if err != nil && !os.IsNotExist(err) {
		return false, err
	}
	mode := os.FileMode(0o600)
	if info, statErr := os.Stat(profile); statErr == nil {
		mode = info.Mode().Perm()
	}
	if strings.Contains(string(data), pathBlockStart) {
		return false, nil
	}
	if len(data) > 0 && data[len(data)-1] != '\n' {
		data = append(data, '\n')
	}
	data = append(data, pathBlockStart+"\n"+`export PATH="$HOME/.local/bin:$PATH"`+"\n"+pathBlockEnd+"\n"...)
	tmp, err := os.CreateTemp(filepath.Dir(profile), ".planreader-profile-")
	if err != nil {
		return false, err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(mode); err != nil {
		_ = tmp.Close()
		return false, err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return false, err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return false, err
	}
	if err := tmp.Close(); err != nil {
		return false, err
	}
	if err := os.Rename(tmpName, profile); err != nil {
		return false, err
	}
	return true, nil
}
