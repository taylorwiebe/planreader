package main

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sync"
)

type SpeechPreferences struct {
	Engine      string  `json:"engine"`
	ModelID     string  `json:"model_id,omitempty"`
	Voice       string  `json:"voice,omitempty"`
	SystemVoice string  `json:"system_voice,omitempty"`
	Rate        float64 `json:"rate"`
}

const (
	speechEngineSystem = "system"
	speechEngineLocal  = "local"
	minSpeechRate      = 0.7
	maxSpeechRate      = 1.4
)

type ModelStore struct {
	root string
	mu   sync.Mutex
}

func defaultModelStore() (*ModelStore, error) {
	config, err := os.UserConfigDir()
	if err != nil {
		return nil, fmt.Errorf("finding user settings directory: %w", err)
	}
	return newModelStore(filepath.Join(config, "Planreader"))
}

func newModelStore(root string) (*ModelStore, error) {
	if err := os.MkdirAll(filepath.Join(root, "models"), 0o700); err != nil {
		return nil, fmt.Errorf("creating speech settings directory: %w", err)
	}
	return &ModelStore{root: root}, nil
}

func defaultPreferences() SpeechPreferences {
	return SpeechPreferences{Engine: speechEngineSystem, Rate: 1}
}

func (s *ModelStore) preferences() (SpeechPreferences, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.preferencesLocked()
}

func (s *ModelStore) preferencesLocked() (SpeechPreferences, bool, error) {
	prefs := defaultPreferences()
	data, err := os.ReadFile(filepath.Join(s.root, "preferences.json"))
	if errors.Is(err, os.ErrNotExist) {
		return prefs, false, nil
	}
	if err != nil {
		return prefs, false, fmt.Errorf("reading speech preferences: %w", err)
	}
	if err := json.Unmarshal(data, &prefs); err != nil {
		return defaultPreferences(), true, nil
	}
	if validateSpeechRate(prefs.Rate) != nil {
		prefs.Rate = 1
	}
	fellBack := false
	if prefs.Engine == speechEngineLocal && !s.installedLocked(prefs.ModelID) {
		prefs.Engine, prefs.ModelID, prefs.Voice = speechEngineSystem, "", ""
		fellBack = true
		if err := writeJSONAtomic(filepath.Join(s.root, "preferences.json"), prefs); err != nil {
			return prefs, true, err
		}
	}
	if prefs.Engine != speechEngineLocal {
		prefs.Engine = speechEngineSystem
	}
	return prefs, fellBack, nil
}

func (s *ModelStore) savePreferences(prefs SpeechPreferences) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := validateSpeechRate(prefs.Rate); err != nil {
		return err
	}
	if prefs.Engine == speechEngineLocal {
		model, ok := findModel(prefs.ModelID)
		if !ok || !s.installedLocked(prefs.ModelID) {
			return errors.New("selected local voice pack is not installed")
		}
		if !slices.Contains(model.Voices, prefs.Voice) {
			return errors.New("selected local voice is not available")
		}
	} else {
		prefs.Engine, prefs.ModelID, prefs.Voice = speechEngineSystem, "", ""
	}
	return writeJSONAtomic(filepath.Join(s.root, "preferences.json"), prefs)
}

func writeJSONAtomic(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".preferences-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

func (s *ModelStore) modelDir(id string) string { return filepath.Join(s.root, "models", id) }

func (s *ModelStore) installed(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.installedLocked(id)
}

func (s *ModelStore) installedLocked(id string) bool {
	model, ok := findModel(id)
	if !ok {
		return false
	}
	base := s.modelDir(id)
	for _, path := range append(requiredModelAssets(model), ".complete") {
		info, err := os.Stat(filepath.Join(base, filepath.FromSlash(path)))
		if err != nil || !info.Mode().IsRegular() {
			return false
		}
	}
	data, err := os.ReadFile(filepath.Join(base, ".complete"))
	if err != nil {
		return false
	}
	var manifest installedManifest
	if json.Unmarshal(data, &manifest) != nil || manifest.Revision != model.Revision {
		return false
	}
	for _, name := range requiredModelAssets(model) {
		expected, ok := manifest.Files[name]
		if !ok {
			return false
		}
		file, err := os.Open(filepath.Join(base, filepath.FromSlash(name)))
		if err != nil {
			return false
		}
		hash := sha256.New()
		written, copyErr := io.Copy(hash, file)
		closeErr := file.Close()
		if copyErr != nil || closeErr != nil || written != expected.Size || fmt.Sprintf("%x", hash.Sum(nil)) != expected.SHA256 {
			return false
		}
	}
	return true
}

func (s *ModelStore) remove(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := findModel(id); !ok {
		return errors.New("unknown voice pack")
	}
	prefs, _, err := s.preferencesLocked()
	selected := prefs.Engine == speechEngineLocal && prefs.ModelID == id
	if err := os.RemoveAll(s.modelDir(id)); err != nil {
		return fmt.Errorf("removing voice pack: %w", err)
	}
	if err == nil && selected {
		prefs.Engine, prefs.ModelID, prefs.Voice = speechEngineSystem, "", ""
		return writeJSONAtomic(filepath.Join(s.root, "preferences.json"), prefs)
	}
	return err
}

func requiredModelAssets(model VoiceModel) []string {
	return []string{
		model.ModelFile,
		"voices.bin",
		"tokens.txt",
		"lexicon-us-en.txt",
		"lexicon-gb-en.txt",
		"espeak-ng-data/en_dict",
		"espeak-ng-data/intonations",
		"espeak-ng-data/phondata",
		"espeak-ng-data/phondata-manifest",
		"espeak-ng-data/phonindex",
		"espeak-ng-data/phontab",
		"espeak-ng-data/lang/gmw/en",
		"espeak-ng-data/lang/gmw/en-US",
	}
}

func validateSpeechRate(rate float64) error {
	if rate < minSpeechRate || rate > maxSpeechRate {
		return errors.New("reading speed must be between 0.7 and 1.4")
	}
	return nil
}
