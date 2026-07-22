package speech

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

type fakeSpeechSynthesizer struct{}

func (fakeSpeechSynthesizer) Synthesize(context.Context, VoiceModel, string, int, float64, string) error {
	return nil
}

func (fakeSpeechSynthesizer) Close() {}

func newTestService(t *testing.T) *Service {
	t.Helper()
	store, err := newModelStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	audio, err := newSessionAudio()
	if err != nil {
		t.Fatal(err)
	}
	service := &Service{store: store, installer: newModelInstaller(), synth: fakeSpeechSynthesizer{}, audio: audio}
	t.Cleanup(service.Close)
	return service
}

func TestServiceExposesSpeechFallback(t *testing.T) {
	service := newTestService(t)
	request := httptest.NewRequest(http.MethodGet, "/reader/secret/api/speech", nil)
	response := httptest.NewRecorder()
	service.HandleAPI(response, request, "speech", "/reader/secret/")
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.Code)
	}
	if !strings.Contains(response.Body.String(), `"engine":"system"`) || !strings.Contains(response.Body.String(), "kokoro-82m") {
		t.Fatalf("unexpected speech response: %s", response.Body.String())
	}
}

func TestServiceAllowsSpeechPreferenceWrites(t *testing.T) {
	service := newTestService(t)
	body := strings.NewReader(`{"engine":"system","system_voice":"Samantha","rate":1.15}`)
	request := httptest.NewRequest(http.MethodPut, "/reader/secret/api/preferences", body)
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	service.HandleAPI(response, request, "preferences", "/reader/secret/")
	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	prefs, _, err := service.store.preferences()
	if err != nil {
		t.Fatal(err)
	}
	if prefs.SystemVoice != "Samantha" || prefs.Rate != 1.15 {
		t.Fatalf("preferences = %#v", prefs)
	}
}

func TestServiceKeepsSessionAudioAvailableForBrowserRangeRequests(t *testing.T) {
	service := newTestService(t)
	path, err := service.audio.path("sample.wav")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("RIFF-test-audio"), 0o600); err != nil {
		t.Fatal(err)
	}
	for attempt := range 2 {
		request := httptest.NewRequest(http.MethodGet, "/reader/secret/api/audio/sample.wav", nil)
		response := httptest.NewRecorder()
		service.HandleAPI(response, request, "audio/sample.wav", "/reader/secret/")
		if response.Code != http.StatusOK {
			t.Fatalf("attempt %d status = %d", attempt+1, response.Code)
		}
	}
}
