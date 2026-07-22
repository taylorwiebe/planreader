package speech

import "runtime"

type VoiceModel struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	Repository   string   `json:"repository"`
	Revision     string   `json:"revision"`
	ModelFile    string   `json:"-"`
	SizeBytes    int64    `json:"size_bytes"`
	License      string   `json:"license"`
	Voices       []string `json:"voices"`
	DefaultVoice string   `json:"default_voice"`
	Supported    bool     `json:"supported"`
	Installed    bool     `json:"installed"`
	InstallPath  string   `json:"install_path,omitempty"`
}

var kokoroEnglishVoices = []string{
	"Alloy (American)", "Aoede (American)", "Bella (American)", "Heart (American)",
	"Jessica (American)", "Kore (American)", "Nicole (American)", "Nova (American)",
	"River (American)", "Sarah (American)", "Sky (American)", "Adam (American)",
	"Echo (American)", "Eric (American)", "Fenrir (American)", "Liam (American)",
	"Michael (American)", "Onyx (American)", "Puck (American)", "Santa (American)",
	"Alice (British)", "Emma (British)", "Isabella (British)", "Lily (British)",
	"Daniel (British)", "Fable (British)", "George (British)", "Lewis (British)",
}

func approvedModels() []VoiceModel {
	supported := runtime.GOOS == "darwin" && runtime.GOARCH == "arm64"
	return []VoiceModel{
		{
			ID: "kokoro-82m-quality", Name: "Kokoro 82M — Higher quality", ModelFile: "model.onnx",
			Description: "Full-precision Kokoro for cleaner, more consistent long-form speech.",
			Repository:  "csukuangfj/kokoro-multi-lang-v1_0", Revision: "7e9b67b79bfdcbd2b4bc144370345fcceac3cb0c",
			SizeBytes: 367_000_000, License: "Apache 2.0", Voices: kokoroEnglishVoices,
			DefaultVoice: "Heart (American)", Supported: supported,
		},
		{
			ID: "kokoro-82m", Name: "Kokoro 82M — Compact", ModelFile: "model.int8.onnx",
			Description: "Smaller and faster, with slightly reduced audio fidelity.",
			Repository:  "csukuangfj/kokoro-int8-multi-lang-v1_0", Revision: "5d6cbe65546edb3ebae8bde976c8ad3438b3f34b",
			SizeBytes: 189453264, License: "Apache 2.0", Voices: kokoroEnglishVoices,
			DefaultVoice: "Heart (American)", Supported: supported,
		},
	}
}

func findModel(id string) (VoiceModel, bool) {
	for _, model := range approvedModels() {
		if model.ID == id {
			return model, true
		}
	}
	return VoiceModel{}, false
}
