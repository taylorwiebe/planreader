package install

import "path/filepath"

type paths struct{ versionDir, binary, command, profile string }

func userPaths(home, version string) paths {
	versionDir := filepath.Join(home, "Library", "Application Support", "Planreader", "versions", version)
	return paths{versionDir, filepath.Join(versionDir, "planreader"), filepath.Join(home, ".local", "bin", "planreader"), filepath.Join(home, ".zprofile")}
}
