package main

import (
	"context"
	"encoding/json"
	"io/fs"
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

func TestReaderSpeechKeepsSelectedVoiceAndAvoidsRepeatedOrientationPrefix(t *testing.T) {
	script, err := fs.ReadFile(webFiles, "web/reader.js")
	if err != nil {
		t.Fatal(err)
	}
	javascript := string(script)
	if strings.Contains(javascript, "Listen for: ${item.textContent}") {
		t.Fatal("orientation speech repeats the 'Listen for' prefix for every item")
	}
	for _, expected := range []string{
		"selectedVoiceURI: null",
		"state.selectedVoiceURI = elements.voice.value",
		"voice.voiceURI === state.selectedVoiceURI",
	} {
		if !strings.Contains(javascript, expected) {
			t.Fatalf("reader JavaScript does not preserve voice selection; missing %q", expected)
		}
	}
}

func TestReaderPrefetchesLocalAudioAndInvalidatesItWhenSettingsChange(t *testing.T) {
	script, err := fs.ReadFile(webFiles, "web/reader.js")
	if err != nil {
		t.Fatal(err)
	}
	javascript := string(script)
	for _, expected := range []string{
		"localAudio: new Map()",
		"const existing = state.localAudio.get(key)",
		"warmLocalAudioAhead(index + 1)",
		"current.then(() => warmLocalAudio(index + 1))",
		"clearLocalAudio();",
		"warmLocalAudioAhead(0)",
	} {
		if !strings.Contains(javascript, expected) {
			t.Fatalf("reader JavaScript does not prefetch and invalidate local audio; missing %q", expected)
		}
	}
}

func TestReaderKeepsKokoroSelectedThroughTransientFailuresAndShowsLoading(t *testing.T) {
	script, err := fs.ReadFile(webFiles, "web/reader.js")
	if err != nil {
		t.Fatal(err)
	}
	markup, err := fs.ReadFile(webFiles, "web/index.html")
	if err != nil {
		t.Fatal(err)
	}
	javascript := string(script)
	for _, expected := range []string{
		"localRetryCount < 1",
		"evictLocalAudio(index)",
		"Kokoro paused. Press Play to retry.",
		"elements.audioLoading.hidden",
	} {
		if !strings.Contains(javascript, expected) {
			t.Fatalf("reader JavaScript is missing local recovery behavior %q", expected)
		}
	}
	if strings.Contains(javascript, "The local voice stopped working") {
		t.Fatal("a transient local speech error still switches to the computer voice")
	}
	if !strings.Contains(string(markup), `role="progressbar"`) {
		t.Fatal("reader does not expose an accessible audio loading indicator")
	}
}

func TestReaderHandlerRequiresToken(t *testing.T) {
	document := ReaderDocument{
		FileName: "plan.md",
		Narration: Narration{
			Title:    "Plan",
			Sections: []NarrationSection{{ID: "intro", Heading: "Intro", Sentences: []string{"Hello."}}},
		},
	}
	handler := newReaderHandler(document, "secret-token")

	request := httptest.NewRequest(http.MethodGet, "/reader/wrong/", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNotFound)
	}
}

func TestReaderHandlerServesDocumentData(t *testing.T) {
	document := ReaderDocument{
		FileName: "plan.md",
		Narration: Narration{
			Title:    "Plan",
			Sections: []NarrationSection{{ID: "intro", Heading: "Intro", Sentences: []string{"Hello."}}},
		},
	}
	handler := newReaderHandler(document, "secret-token")

	request := httptest.NewRequest(http.MethodGet, "/reader/secret-token/data.json", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if contentType := response.Header().Get("Content-Type"); !strings.Contains(contentType, "application/json") {
		t.Fatalf("Content-Type = %q, want application/json", contentType)
	}
	var got ReaderDocument
	if err := json.NewDecoder(response.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got.Narration.Title != "Plan" {
		t.Fatalf("title = %q, want Plan", got.Narration.Title)
	}
}

func TestReaderHandlerExposesSpeechFallback(t *testing.T) {
	store, err := newModelStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	audio, err := newSessionAudio()
	if err != nil {
		t.Fatal(err)
	}
	defer audio.Close()
	speech := &speechService{store: store, installer: newModelInstaller(), synth: fakeSpeechSynthesizer{}, audio: audio}
	handler := newReaderHandlerWithSpeech(ReaderDocument{}, "secret", speech)
	request := httptest.NewRequest(http.MethodGet, "/reader/secret/api/speech", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.Code)
	}
	if !strings.Contains(response.Body.String(), `"engine":"system"`) || !strings.Contains(response.Body.String(), "kokoro-82m") {
		t.Fatalf("unexpected speech response: %s", response.Body.String())
	}
}

func TestReaderHandlerAllowsSpeechPreferenceWrites(t *testing.T) {
	store, err := newModelStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	audio, err := newSessionAudio()
	if err != nil {
		t.Fatal(err)
	}
	defer audio.Close()
	speech := &speechService{store: store, installer: newModelInstaller(), synth: fakeSpeechSynthesizer{}, audio: audio}
	handler := newReaderHandlerWithSpeech(ReaderDocument{}, "secret", speech)
	body := strings.NewReader(`{"engine":"system","system_voice":"Samantha","rate":1.15}`)
	request := httptest.NewRequest(http.MethodPut, "/reader/secret/api/preferences", body)
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	prefs, _, err := store.preferences()
	if err != nil {
		t.Fatal(err)
	}
	if prefs.SystemVoice != "Samantha" || prefs.Rate != 1.15 {
		t.Fatalf("preferences = %#v", prefs)
	}
}

func TestReaderHandlerKeepsSessionAudioAvailableForBrowserRangeRequests(t *testing.T) {
	store, err := newModelStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	audio, err := newSessionAudio()
	if err != nil {
		t.Fatal(err)
	}
	defer audio.Close()
	path, err := audio.path("sample.wav")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("RIFF-test-audio"), 0o600); err != nil {
		t.Fatal(err)
	}
	speech := &speechService{store: store, installer: newModelInstaller(), synth: fakeSpeechSynthesizer{}, audio: audio}
	handler := newReaderHandlerWithSpeech(ReaderDocument{}, "secret", speech)
	for attempt := range 2 {
		request := httptest.NewRequest(http.MethodGet, "/reader/secret/api/audio/sample.wav", nil)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("attempt %d status = %d", attempt+1, response.Code)
		}
	}
}

func TestRenderSourceSectionsEscapesRawHTML(t *testing.T) {
	sections, err := renderSourceSections([]SourceSection{{
		ID:       "source-0",
		Heading:  "Unsafe",
		Markdown: "# Unsafe\n<script>alert('no')</script>",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(sections[0].HTML, "<script>") {
		t.Fatalf("rendered HTML contains an executable script: %s", sections[0].HTML)
	}
}

func TestRenderSourceSectionsRendersTables(t *testing.T) {
	sections, err := renderSourceSections([]SourceSection{{
		ID:       "source-0",
		Heading:  "Table",
		Markdown: "| Item | Result |\n|---|---|\n| One | Good |",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sections[0].HTML, "<table>") {
		t.Fatalf("rendered HTML does not contain a table: %s", sections[0].HTML)
	}
}
