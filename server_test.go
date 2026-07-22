package main

import (
	"encoding/json"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

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
