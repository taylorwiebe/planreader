package speech

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

type Service struct {
	store     *ModelStore
	installer *modelInstaller
	synth     speechSynthesizer
	audio     *sessionAudio
}

func NewService() (*Service, error) {
	store, err := defaultModelStore()
	if err != nil {
		return nil, err
	}
	audio, err := newSessionAudio()
	if err != nil {
		return nil, err
	}
	return &Service{store: store, installer: newModelInstaller(), synth: newSpeechSynthesizer(store), audio: audio}, nil
}

func (s *Service) Close() {
	s.synth.Close()
	_ = s.audio.Close()
}

func (s *Service) HandleAPI(w http.ResponseWriter, r *http.Request, route, prefix string) {
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
		_ = json.NewEncoder(w).Encode(map[string]any{"preferences": prefs, "models": s.store.catalog(), "fell_back": fellBack})
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
		_ = json.NewEncoder(w).Encode(map[string]any{"installed": true, "install_path": s.store.modelDir(model.ID)})
	case strings.HasPrefix(route, "models/") && strings.HasSuffix(route, "/progress") && r.Method == http.MethodGet:
		id := strings.TrimSuffix(strings.TrimPrefix(route, "models/"), "/progress")
		if _, ok := findModel(id); !ok {
			writeError(http.StatusNotFound, errors.New("unknown voice pack"))
			return
		}
		_ = json.NewEncoder(w).Encode(s.installer.modelProgress(id))
	case strings.HasPrefix(route, "models/") && r.Method == http.MethodDelete:
		if err := s.store.remove(strings.TrimPrefix(route, "models/")); err != nil {
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
		path, err := s.audio.path(strings.TrimPrefix(route, "audio/"))
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
