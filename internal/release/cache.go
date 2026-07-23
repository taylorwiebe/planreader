package release

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

type Check struct {
	Latest    string    `json:"latest"`
	CheckedAt time.Time `json:"checked_at"`
}

func WriteCache(path string, check Check, now time.Time) error {
	check.CheckedAt = now
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	b, err := json.Marshal(check)
	if err != nil {
		return err
	}
	tmp := path + ".new"
	if err = os.WriteFile(tmp, b, 0600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
func ReadCache(path string, now time.Time) (Check, bool) {
	var c Check
	b, err := os.ReadFile(path)
	if err != nil || json.Unmarshal(b, &c) != nil || now.Sub(c.CheckedAt) >= 24*time.Hour || now.Before(c.CheckedAt) {
		return c, false
	}
	return c, true
}
