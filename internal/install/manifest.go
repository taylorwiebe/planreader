package install

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const manifestName = ".planreader-managed.json"

type manifest struct {
	Product string `json:"product"`
	Version string `json:"version"`
	Origin  string `json:"origin"`
}

func writeManifest(dir string, value manifest) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, manifestName), append(data, '\n'), 0o600)
}
func managed(dir string) bool {
	data, err := os.ReadFile(filepath.Join(dir, manifestName))
	if err != nil {
		return false
	}
	var value manifest
	return json.Unmarshal(data, &value) == nil && value.Product == "planreader"
}
