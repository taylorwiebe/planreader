package speech

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type speechRequest struct {
	Text    string  `json:"text"`
	ModelID string  `json:"model_id"`
	Voice   string  `json:"voice"`
	Rate    float64 `json:"rate"`
}

type speechSynthesizer interface {
	Synthesize(context.Context, VoiceModel, string, int, float64, string) error
	Close()
}

type sessionAudio struct {
	dir string
}

func newSessionAudio() (*sessionAudio, error) {
	dir, err := os.MkdirTemp("", "planreader-audio-*")
	if err != nil {
		return nil, fmt.Errorf("creating temporary audio directory: %w", err)
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		os.RemoveAll(dir)
		return nil, err
	}
	return &sessionAudio{dir: dir}, nil
}

func (s *sessionAudio) path(name string) (string, error) {
	if name == "" || strings.ContainsAny(name, `/\\`) || filepath.Ext(name) != ".wav" {
		return "", errors.New("invalid audio name")
	}
	return filepath.Join(s.dir, name), nil
}

func (s *sessionAudio) Close() error { return os.RemoveAll(s.dir) }

func validateSpeechRequest(request speechRequest) (VoiceModel, int, error) {
	model, ok := findModel(request.ModelID)
	if !ok {
		return VoiceModel{}, 0, errors.New("unknown voice pack")
	}
	if len(strings.TrimSpace(request.Text)) == 0 || len(request.Text) > 4000 {
		return VoiceModel{}, 0, errors.New("speech text must be between 1 and 4000 characters")
	}
	if err := validateSpeechRate(request.Rate); err != nil {
		return VoiceModel{}, 0, err
	}
	for id, voice := range model.Voices {
		if voice == request.Voice {
			return model, id, nil
		}
	}
	return VoiceModel{}, 0, errors.New("unknown voice")
}
