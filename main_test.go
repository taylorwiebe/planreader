package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestReadPreparedDocumentReusesCompleteReaderData(t *testing.T) {
	want := ReaderDocument{
		FileName:  "plan.md",
		Narration: Narration{Title: "Prepared plan", Sections: []NarrationSection{{ID: "intro", Heading: "Intro", Sentences: []string{"Already written."}}}},
		Sources:   []RenderedSourceSection{{ID: "source-intro", Heading: "Intro", HTML: "<p>Original</p>"}},
	}
	data, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "data.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := readPreparedDocument(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.FileName != want.FileName || got.Narration.Title != want.Narration.Title || len(got.Sources) != 1 {
		t.Fatalf("readPreparedDocument() = %#v", got)
	}
}

func TestReadPreparedDocumentRejectsIncompleteData(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.json")
	if err := os.WriteFile(path, []byte(`{"file_name":"plan.md"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readPreparedDocument(path); err == nil {
		t.Fatal("readPreparedDocument accepted incomplete data")
	}
}
