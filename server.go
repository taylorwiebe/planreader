package main

import (
	"bytes"
	"crypto/rand"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
)

//go:embed web/*
var webFiles embed.FS

type ReaderDocument struct {
	FileName  string                  `json:"file_name"`
	Narration Narration               `json:"narration"`
	Sources   []RenderedSourceSection `json:"sources"`
}

type RenderedSourceSection struct {
	ID      string `json:"id"`
	Heading string `json:"heading"`
	Level   int    `json:"level"`
	HTML    string `json:"html"`
}

type speechService struct {
	store     *ModelStore
	installer *modelInstaller
	synth     speechSynthesizer
	audio     *sessionAudio
}

func newSpeechService() (*speechService, error) {
	store, err := defaultModelStore()
	if err != nil {
		return nil, err
	}
	audio, err := newSessionAudio()
	if err != nil {
		return nil, err
	}
	return &speechService{store: store, installer: newModelInstaller(), synth: newSpeechSynthesizer(store), audio: audio}, nil
}

func (s *speechService) Close() {
	s.synth.Close()
	_ = s.audio.Close()
}

func renderSourceSections(sections []SourceSection) ([]RenderedSourceSection, error) {
	markdown := goldmark.New(goldmark.WithExtensions(extension.GFM))
	rendered := make([]RenderedSourceSection, 0, len(sections))
	for _, section := range sections {
		var output bytes.Buffer
		if err := markdown.Convert([]byte(section.Markdown), &output); err != nil {
			return nil, fmt.Errorf("rendering source section %q: %w", section.Heading, err)
		}
		rendered = append(rendered, RenderedSourceSection{
			ID:      section.ID,
			Heading: section.Heading,
			Level:   section.Level,
			HTML:    output.String(),
		})
	}
	return rendered, nil
}

func newReaderHandler(document ReaderDocument, token string) http.Handler {
	return newReaderHandlerWithSpeech(document, token, nil)
}

func newReaderHandlerWithSpeech(document ReaderDocument, token string, speech *speechService) http.Handler {
	prefix := "/reader/" + token + "/"
	data, err := json.Marshal(document)
	if err != nil {
		panic(fmt.Sprintf("encoding reader document: %v", err))
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asset, ok := strings.CutPrefix(r.URL.Path, prefix)
		if !ok {
			http.NotFound(w, r)
			return
		}
		if asset == "" {
			asset = "index.html"
		}

		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; media-src 'self' blob:; connect-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'")

		if speech != nil && strings.HasPrefix(asset, "api/") {
			speech.handleAPI(w, r, strings.TrimPrefix(asset, "api/"), prefix)
			return
		}
		if r.Method != http.MethodGet {
			http.NotFound(w, r)
			return
		}

		if asset == "data.json" {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			_, _ = w.Write(data)
			return
		}

		content, err := fs.ReadFile(webFiles, "web/"+asset)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		switch {
		case strings.HasSuffix(asset, ".html"):
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
		case strings.HasSuffix(asset, ".css"):
			w.Header().Set("Content-Type", "text/css; charset=utf-8")
		case strings.HasSuffix(asset, ".js"):
			w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		}
		_, _ = w.Write(content)
	})
}

func (s *speechService) handleAPI(w http.ResponseWriter, r *http.Request, route, prefix string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	writeError := func(status int, err error) {
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
	}
	if r.Method != http.MethodGet {
		origin := r.Header.Get("Origin")
		if origin != "" && origin != "http://"+r.Host {
			writeError(http.StatusForbidden, errors.New("request did not come from this reader"))
			return
		}
	}
	switch {
	case route == "speech" && r.Method == http.MethodGet:
		prefs, fellBack, err := s.store.preferences()
		if err != nil {
			writeError(http.StatusInternalServerError, err)
			return
		}
		models := approvedModels()
		for i := range models {
			models[i].Installed = s.store.installed(models[i].ID)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"preferences": prefs, "models": models, "fell_back": fellBack})
	case route == "preferences" && r.Method == http.MethodPut:
		var prefs SpeechPreferences
		if err := decodeJSON(r, &prefs); err != nil {
			writeError(http.StatusBadRequest, err)
			return
		}
		if err := s.store.savePreferences(prefs); err != nil {
			writeError(http.StatusBadRequest, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	case strings.HasPrefix(route, "models/") && strings.HasSuffix(route, "/install") && r.Method == http.MethodPost:
		id := strings.TrimSuffix(strings.TrimPrefix(route, "models/"), "/install")
		model, ok := findModel(id)
		if !ok {
			writeError(http.StatusNotFound, errors.New("unknown voice pack"))
			return
		}
		if err := s.installer.install(r.Context(), s.store, model); err != nil {
			writeError(http.StatusBadGateway, err)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]bool{"installed": true})
	case strings.HasPrefix(route, "models/") && strings.HasSuffix(route, "/progress") && r.Method == http.MethodGet:
		id := strings.TrimSuffix(strings.TrimPrefix(route, "models/"), "/progress")
		if _, ok := findModel(id); !ok {
			writeError(http.StatusNotFound, errors.New("unknown voice pack"))
			return
		}
		_ = json.NewEncoder(w).Encode(s.installer.modelProgress(id))
	case strings.HasPrefix(route, "models/") && r.Method == http.MethodDelete:
		id := strings.TrimPrefix(route, "models/")
		if err := s.store.remove(id); err != nil {
			writeError(http.StatusBadRequest, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	case route == "synthesize" && r.Method == http.MethodPost:
		var request speechRequest
		if err := decodeJSON(r, &request); err != nil {
			writeError(http.StatusBadRequest, err)
			return
		}
		model, voice, err := validateSpeechRequest(request)
		if err != nil || !s.store.installed(request.ModelID) {
			if err == nil {
				err = errors.New("selected voice pack is not installed")
			}
			writeError(http.StatusBadRequest, err)
			return
		}
		nameBytes := make([]byte, 12)
		if _, err := rand.Read(nameBytes); err != nil {
			writeError(http.StatusInternalServerError, err)
			return
		}
		name := hex.EncodeToString(nameBytes) + ".wav"
		path, err := s.audio.path(name)
		if err != nil {
			writeError(http.StatusInternalServerError, err)
			return
		}
		if err := s.synth.Synthesize(r.Context(), model, request.Text, voice, request.Rate, path); err != nil {
			writeError(http.StatusInternalServerError, err)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"audio_url": prefix + "api/audio/" + name})
	case strings.HasPrefix(route, "audio/") && r.Method == http.MethodGet:
		name := strings.TrimPrefix(route, "audio/")
		path, err := s.audio.path(name)
		if err != nil {
			writeError(http.StatusBadRequest, err)
			return
		}
		w.Header().Set("Content-Type", "audio/wav")
		http.ServeFile(w, r, path)
	default:
		http.NotFound(w, r)
	}
}

func decodeJSON(r *http.Request, value any) error {
	defer r.Body.Close()
	decoder := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 16<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return fmt.Errorf("invalid request: %w", err)
	}
	return nil
}

func startReaderServer(document ReaderDocument) (string, *http.Server, error) {
	speech, err := newSpeechService()
	if err != nil {
		return "", nil, err
	}
	tokenBytes := make([]byte, 24)
	if _, err := rand.Read(tokenBytes); err != nil {
		speech.Close()
		return "", nil, fmt.Errorf("creating local access token: %w", err)
	}
	token := hex.EncodeToString(tokenBytes)

	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		speech.Close()
		return "", nil, fmt.Errorf("starting local reader: %w", err)
	}
	server := &http.Server{
		Handler:           newReaderHandlerWithSpeech(document, token, speech),
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		_ = server.Serve(listener)
		speech.Close()
	}()

	url := fmt.Sprintf("http://%s/reader/%s/", listener.Addr().String(), token)
	return url, server, nil
}

func openBrowser(url string) error {
	var command string
	var args []string
	switch runtime.GOOS {
	case "darwin":
		command, args = "open", []string{url}
	case "windows":
		command, args = "rundll32", []string{"url.dll,FileProtocolHandler", url}
	default:
		command, args = "xdg-open", []string{url}
	}
	cmd := exec.Command(command, args...)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("opening browser: %w", err)
	}
	go func() { _ = cmd.Wait() }()
	return nil
}
