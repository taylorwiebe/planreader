//go:build darwin && arm64

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	sherpa "github.com/k2-fsa/sherpa-onnx-go/sherpa_onnx"
)

type sherpaSynthesizer struct {
	store   *ModelStore
	mu      sync.Mutex
	modelID string
	tts     *sherpa.OfflineTts
}

func newSpeechSynthesizer(store *ModelStore) speechSynthesizer {
	return &sherpaSynthesizer{store: store}
}

func (s *sherpaSynthesizer) Synthesize(ctx context.Context, model VoiceModel, text string, voice int, rate float64, destination string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	if s.tts == nil || s.modelID != model.ID {
		if s.tts != nil {
			sherpa.DeleteOfflineTts(s.tts)
		}
		base := s.store.modelDir(model.ID)
		config := sherpa.OfflineTtsConfig{}
		config.Model.Kokoro.Model = filepath.Join(base, model.ModelFile)
		config.Model.Kokoro.Voices = filepath.Join(base, "voices.bin")
		config.Model.Kokoro.Tokens = filepath.Join(base, "tokens.txt")
		config.Model.Kokoro.DataDir = filepath.Join(base, "espeak-ng-data")
		config.Model.Kokoro.Lexicon = filepath.Join(base, "lexicon-us-en.txt") + "," + filepath.Join(base, "lexicon-gb-en.txt")
		config.Model.NumThreads = 2
		config.Model.Provider = "cpu"
		config.MaxNumSentences = 1
		s.tts = sherpa.NewOfflineTts(&config)
		if s.tts == nil {
			return errors.New("the local voice pack could not be opened")
		}
		s.modelID = model.ID
	}
	generated := s.tts.GenerateWithConfig(text, &sherpa.GenerationConfig{Sid: voice, Speed: float32(rate)}, nil)
	if generated == nil || len(generated.Samples) == 0 {
		return errors.New("the local voice did not produce audio")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if !generated.Save(destination) {
		return fmt.Errorf("writing temporary audio %s", filepath.Base(destination))
	}
	return os.Chmod(destination, 0o600)
}

func (s *sherpaSynthesizer) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.tts != nil {
		sherpa.DeleteOfflineTts(s.tts)
		s.tts = nil
	}
}
