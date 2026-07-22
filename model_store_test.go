package main

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestPreferencesFallBackWhenModelIsMissing(t *testing.T) {
	store, err := newModelStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := writeJSONAtomic(filepath.Join(store.root, "preferences.json"), SpeechPreferences{
		Engine: "local", ModelID: "kokoro-82m", Voice: "Heart (American)", Rate: 1.1,
	}); err != nil {
		t.Fatal(err)
	}
	prefs, fellBack, err := store.preferences()
	if err != nil {
		t.Fatal(err)
	}
	if !fellBack || prefs.Engine != "system" || prefs.Rate != 1.1 {
		t.Fatalf("preferences = %#v, fellBack = %v", prefs, fellBack)
	}
}

func TestInstalledModelRequiresCompleteAssets(t *testing.T) {
	store, err := newModelStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	model, _ := findModel("kokoro-82m")
	base := store.modelDir(model.ID)
	manifest := installedManifest{Revision: model.Revision, Files: make(map[string]fileIntegrity)}
	for _, name := range requiredModelAssets(model) {
		path := filepath.Join(base, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("test"), 0o600); err != nil {
			t.Fatal(err)
		}
		hash := sha256.Sum256([]byte("test"))
		manifest.Files[name] = fileIntegrity{Size: 4, SHA256: fmt.Sprintf("%x", hash[:])}
	}
	if err := writeJSONAtomic(filepath.Join(base, ".complete"), manifest); err != nil {
		t.Fatal(err)
	}
	if !store.installed(model.ID) {
		t.Fatal("complete model was not recognized")
	}
	if err := os.Remove(filepath.Join(base, "voices.bin")); err != nil {
		t.Fatal(err)
	}
	if store.installed(model.ID) {
		t.Fatal("corrupt model was recognized as installed")
	}
}

func TestSafeRelativePath(t *testing.T) {
	for _, path := range []string{"../secret", "/tmp/model", "dir/../../secret", ""} {
		if safeRelativePath(path) == nil {
			t.Errorf("safeRelativePath(%q) accepted an unsafe path", path)
		}
	}
	for _, path := range []string{"model.onnx", "espeak-ng-data/en_dict"} {
		if err := safeRelativePath(path); err != nil {
			t.Errorf("safeRelativePath(%q) = %v", path, err)
		}
	}
}

func TestSessionAudioRejectsTraversalAndCleansUp(t *testing.T) {
	audio, err := newSessionAudio()
	if err != nil {
		t.Fatal(err)
	}
	dir := audio.dir
	if _, err := audio.path("../private.wav"); err == nil {
		t.Fatal("audio path accepted traversal")
	}
	if err := audio.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("temporary audio directory still exists: %v", err)
	}
}

func TestApprovedCatalogStaysSmallAndPinned(t *testing.T) {
	models := approvedModels()
	if len(models) != 1 {
		t.Fatalf("catalog has %d models, want 1", len(models))
	}
	for _, model := range models {
		if len(model.Revision) != 40 || model.SizeBytes > 192<<20 || len(model.Voices) != 28 {
			t.Errorf("catalog entry is not bounded and pinned: %#v", model)
		}
	}
}

func TestNextPage(t *testing.T) {
	link := `<https://huggingface.co/api/models/catalog/tree/revision?cursor=next>; rel="next", <https://example.com>; rel="other"`
	if got := nextPage(link); got != "https://huggingface.co/api/models/catalog/tree/revision?cursor=next" {
		t.Fatalf("nextPage() = %q", got)
	}
	if got := nextPage(""); got != "" {
		t.Fatalf("nextPage(empty) = %q", got)
	}
}

func TestValidateDownloadURL(t *testing.T) {
	for _, raw := range []string{"http://huggingface.co/file", "https://127.0.0.1/file", "https://huggingface.co:444/file", "https://huggingface.co@127.0.0.1/file", "https://us.aws.cdn.hf.co.evil.example/file"} {
		if validateDownloadURL(raw) == nil {
			t.Errorf("accepted %q", raw)
		}
	}
	if err := validateDownloadURL("https://huggingface.co/api/models/repo/tree/revision"); err != nil {
		t.Fatal(err)
	}
	if err := validateDownloadURL("https://us.aws.cdn.hf.co/xet-bridge-us/model/file"); err != nil {
		t.Fatalf("official Hugging Face Xet CDN was rejected: %v", err)
	}
}

func TestProgressWriterReportsCumulativeBytes(t *testing.T) {
	var reports []int64
	writer := &progressWriter{onProgress: func(written int64) { reports = append(reports, written) }}
	if _, err := writer.Write([]byte("abc")); err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Write([]byte("defg")); err != nil {
		t.Fatal(err)
	}
	if len(reports) != 2 || reports[0] != 3 || reports[1] != 7 {
		t.Fatalf("progress reports = %v, want [3 7]", reports)
	}
}
