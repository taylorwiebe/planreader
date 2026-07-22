//go:build !darwin || !arm64

package main

import (
	"context"
	"errors"
)

type unavailableSynthesizer struct{}

func newSpeechSynthesizer(*ModelStore) speechSynthesizer { return unavailableSynthesizer{} }
func (unavailableSynthesizer) Synthesize(context.Context, VoiceModel, string, int, float64, string) error {
	return errors.New("local voice packs currently require a Mac with Apple silicon")
}
func (unavailableSynthesizer) Close() {}
