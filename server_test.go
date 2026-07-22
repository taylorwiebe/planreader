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

func TestVoiceConfigurationLivesInSettingsAndNewModelsUseTheirDefaultVoice(t *testing.T) {
	markup, err := fs.ReadFile(webFiles, "web/index.html")
	if err != nil {
		t.Fatal(err)
	}
	javascript, err := fs.ReadFile(webFiles, "web/reader.js")
	if err != nil {
		t.Fatal(err)
	}

	html := string(markup)
	settingsStart := strings.Index(html, `id="settings-dialog"`)
	settingsEnd := strings.Index(html, `</dialog>`)
	voiceMenu := strings.Index(html, `id="voice"`)
	playerStart := strings.Index(html, `class="player"`)
	if settingsStart < 0 || settingsEnd < 0 || voiceMenu < settingsStart || voiceMenu > settingsEnd {
		t.Fatal("voice selection is not contained in the speech settings dialog")
	}
	if playerStart < 0 || strings.Contains(html[playerStart:], `id="voice"`) || strings.Contains(html[playerStart:], `id="preview-voice"`) {
		t.Fatal("the playback bar still contains voice configuration")
	}
	if !strings.Contains(string(javascript), `state.modelID === model.id ? state.localVoice : model.default_voice`) {
		t.Fatal("switching Kokoro models does not start with that model's default voice")
	}
	for _, expected := range []string{`model.install_path`, `element("code", "", model.install_path)`, `model.install_path = result.install_path`} {
		if !strings.Contains(string(javascript), expected) {
			t.Fatalf("installed model location is not shown or refreshed; missing %q", expected)
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
		"warmLocalAudioAhead(index)",
		"current.then(() => prepareLocalAudio(index + 1)).then(() => prepareLocalAudio(index + 2))",
		"clearLocalAudio();",
		"warmLocalAudioAhead(0)",
	} {
		if !strings.Contains(javascript, expected) {
			t.Fatalf("reader JavaScript does not prefetch and invalidate local audio; missing %q", expected)
		}
	}
}

func TestPlaybackStateFollowsAudioAndKeyboardMediaControls(t *testing.T) {
	script, err := fs.ReadFile(webFiles, "web/reader.js")
	if err != nil {
		t.Fatal(err)
	}
	javascript := string(script)
	for _, expected := range []string{
		`audio.addEventListener("play"`,
		`audio.addEventListener("pause"`,
		`navigator.mediaSession.setActionHandler("play"`,
		`navigator.mediaSession.setActionHandler("pause"`,
		`navigator.mediaSession.playbackState`,
		"primedAudio: null",
		"audioContext: null",
		"primeLocalAudio()",
		"state.audioContext.resume()",
		"audio = bufferedAudio(context, buffer)",
	} {
		if !strings.Contains(javascript, expected) {
			t.Fatalf("playback UI is not synchronized with media playback; missing %q", expected)
		}
	}
}

func TestPlaybackToolbarUsesAnIconAndHidesTextStatus(t *testing.T) {
	markup, err := fs.ReadFile(webFiles, "web/index.html")
	if err != nil {
		t.Fatal(err)
	}
	javascript, err := fs.ReadFile(webFiles, "web/reader.js")
	if err != nil {
		t.Fatal(err)
	}
	styles, err := fs.ReadFile(webFiles, "web/styles.css")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(markup), `id="play" class="primary icon-button"`) {
		t.Fatal("playback control is not presented as an icon button")
	}
	if !strings.Contains(string(markup), `id="status" class="status player-status"`) {
		t.Fatal("toolbar status is not isolated for visually hidden announcements")
	}
	for _, expected := range []string{`"▶"`, `"Ⅱ"`, `elements.play.setAttribute("aria-label"`} {
		if !strings.Contains(string(javascript), expected) {
			t.Fatalf("playback icon state is missing %q", expected)
		}
	}
	if !strings.Contains(string(styles), ".player-status {") || !strings.Contains(string(styles), "clip-path: inset(50%)") {
		t.Fatal("toolbar text status remains visually visible")
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
	styles, err := fs.ReadFile(webFiles, "web/styles.css")
	if err != nil {
		t.Fatal(err)
	}
	javascript := string(script)
	for _, expected := range []string{
		"localRetryCount < 1",
		"evictLocalAudio(index)",
		"Kokoro paused. Press Play to retry.",
		"elements.play.disabled = waiting",
		`elements.play.setAttribute("aria-busy", String(waiting))`,
		"elements.audioLoading.hidden = !waiting",
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
	playStart := strings.Index(string(markup), `id="play"`)
	if playStart < 0 {
		t.Fatal("reader is missing the play button")
	}
	playEnd := playStart + strings.Index(string(markup)[playStart:], `</button>`)
	loading := strings.Index(string(markup), `id="audio-loading"`)
	if playEnd < playStart || loading < playStart || loading > playEnd {
		t.Fatal("audio loading indicator is not contained in the play button")
	}
	if !strings.Contains(string(styles), ".button-loading::after") {
		t.Fatal("play button loading indicator does not preserve the horizontal moving bar")
	}
}

func TestReaderKeepsNarrationAndSourceInIndependentScrollPanes(t *testing.T) {
	styles, err := fs.ReadFile(webFiles, "web/styles.css")
	if err != nil {
		t.Fatal(err)
	}
	css := string(styles)
	for _, expected := range []string{
		"height: 100vh",
		"overflow: hidden",
		"flex: 1",
		"overflow-y: auto",
		"overscroll-behavior: contain",
		"padding-bottom: 10rem",
		"scroll-padding-bottom: 10rem",
	} {
		if !strings.Contains(css, expected) {
			t.Fatalf("reader layout does not keep both documents in independent scroll panes; missing %q", expected)
		}
	}
}

func TestReaderUsesSimpleDesktopDividerAndMobileSourceOverlay(t *testing.T) {
	styles, err := fs.ReadFile(webFiles, "web/styles.css")
	if err != nil {
		t.Fatal(err)
	}
	markup, err := fs.ReadFile(webFiles, "web/index.html")
	if err != nil {
		t.Fatal(err)
	}
	javascript, err := fs.ReadFile(webFiles, "web/reader.js")
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"border-left: 1px solid var(--line)",
		"margin: -1.4rem -1.4rem 1rem",
		"top: calc(-1.4rem - 2px)",
		"box-shadow: 0 -4px 0 var(--paper)",
		"top: calc(-1.25rem - 2px)",
		"position: fixed",
		"transform: translateX(105%)",
		".source-pane.visible { transform: translateX(0); visibility: visible; transition-delay: 0s; }",
	} {
		if !strings.Contains(string(styles), expected) {
			t.Fatalf("reader layout is missing %q", expected)
		}
	}
	if !strings.Contains(string(markup), `id="source-close"`) {
		t.Fatal("mobile source overlay has no close control")
	}
	if !strings.Contains(string(javascript), `elements.sourceClose.addEventListener`) {
		t.Fatal("mobile source overlay close control is not wired")
	}
	if !strings.Contains(string(javascript), `event.key === "Escape"`) {
		t.Fatal("mobile source overlay cannot be dismissed with Escape")
	}
	for _, expected := range []string{
		`elements.sourcePane.contains(event.target)`,
		`elements.settingsDialog.getBoundingClientRect()`,
		`elements.settingsDialog.close()`,
	} {
		if !strings.Contains(string(javascript), expected) {
			t.Fatalf("reader click-away behavior is missing %q", expected)
		}
	}
}

func TestReaderControlsStayUsableOnMobile(t *testing.T) {
	styles, err := fs.ReadFile(webFiles, "web/styles.css")
	if err != nil {
		t.Fatal(err)
	}
	css := string(styles)
	for _, expected := range []string{
		"min-height: 2.75rem",
		"align-items: flex-start",
		"width: fit-content",
		"grid-template-columns: auto auto",
		"grid-template-columns: repeat(2, minmax(0, 1fr))",
		"grid-template-columns: repeat(4, minmax(0, 1fr))",
		".player-buttons button",
		"flex-direction: column",
		"white-space: nowrap",
	} {
		if !strings.Contains(css, expected) {
			t.Fatalf("mobile controls are missing responsive rule %q", expected)
		}
	}
}

func TestRecallPromptUsesAStableBlockCallout(t *testing.T) {
	styles, err := fs.ReadFile(webFiles, "web/styles.css")
	if err != nil {
		t.Fatal(err)
	}
	css := string(styles)
	for _, expected := range []string{
		".recall {",
		"display: block",
		"border-left: 3px solid var(--accent)",
		".recall.active { background: var(--accent-soft); box-shadow: none; }",
	} {
		if !strings.Contains(css, expected) {
			t.Fatalf("recall prompt is missing stable callout styling %q", expected)
		}
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
