package main

import "runtime"

type VoiceModel struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Repository  string   `json:"repository"`
	Revision    string   `json:"revision"`
	ModelFile   string   `json:"-"`
	SizeBytes   int64    `json:"size_bytes"`
	License     string   `json:"license"`
	Voices      []string `json:"voices"`
	Supported   bool     `json:"supported"`
	Installed   bool     `json:"installed"`
}

var kittenVoices = []string{
	"Bella", "Jasper", "Luna", "Bruno", "Rosie", "Hugo", "Kiki", "Leo",
}

func approvedModels() []VoiceModel {
	supported := runtime.GOOS == "darwin" && runtime.GOARCH == "arm64"
	return []VoiceModel{
		{
			ID: "kitten-nano", Name: "Kitten Nano", ModelFile: "model.int8.onnx",
			Description: "The smallest download. Natural enough for everyday reading and quickest to install.",
			Repository:  "csukuangfj2/kitten-nano-en-v0_8-int8", Revision: "90dfe12687f7822a90e5afc5931b536ba6caf22a",
			SizeBytes: 41833681, License: "Apache 2.0", Voices: kittenVoices, Supported: supported,
		},
		{
			ID: "kitten-micro", Name: "Kitten Micro", ModelFile: "model.onnx",
			Description: "A slightly larger voice pack with more room for natural phrasing.",
			Repository:  "csukuangfj2/kitten-micro-en-v0_8", Revision: "7131b31e5fbe7da21e32f84899d1d273a1477e15",
			SizeBytes: 58848672, License: "Apache 2.0", Voices: kittenVoices, Supported: supported,
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
