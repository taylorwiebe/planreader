package main

import (
	"encoding/json"
	"testing"
)

func TestParseClaudeResponseStructuredOutput(t *testing.T) {
	want := Narration{
		Title: "A simpler plan",
		Sections: []NarrationSection{{
			ID:               "overview",
			Heading:          "What this changes",
			SourceSectionIDs: []string{"source-1"},
			Sentences:        []string{"The queued run keeps the user's permissions."},
		}},
	}
	structured, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	envelope, err := json.Marshal(map[string]json.RawMessage{
		"structured_output": structured,
	})
	if err != nil {
		t.Fatal(err)
	}

	got, err := parseClaudeResponse(envelope)
	if err != nil {
		t.Fatalf("parseClaudeResponse() error = %v", err)
	}
	if got.Title != want.Title || len(got.Sections) != 1 {
		t.Fatalf("parseClaudeResponse() = %#v, want %#v", got, want)
	}
	if got.Sections[0].SourceSectionIDs[0] != "source-1" {
		t.Fatalf("source mapping = %q, want source-1", got.Sections[0].SourceSectionIDs[0])
	}
}

func TestParseClaudeResponseResultString(t *testing.T) {
	narrationJSON := `{"title":"Briefing","sections":[{"id":"intro","heading":"Introduction","source_section_ids":["source-0"],"sentences":["This is the main idea."]}]}`
	envelope, err := json.Marshal(map[string]string{"result": narrationJSON})
	if err != nil {
		t.Fatal(err)
	}

	got, err := parseClaudeResponse(envelope)
	if err != nil {
		t.Fatalf("parseClaudeResponse() error = %v", err)
	}
	if got.Title != "Briefing" {
		t.Fatalf("title = %q, want Briefing", got.Title)
	}
}

func TestValidateNarrationRejectsEmptySentences(t *testing.T) {
	err := validateNarration(Narration{
		Title: "Empty",
		Sections: []NarrationSection{{
			ID:        "intro",
			Heading:   "Introduction",
			Sentences: nil,
		}},
	})
	if err == nil {
		t.Fatal("validateNarration() error = nil, want an error")
	}
}

func TestValidateSourceMappingsRejectsUnknownSection(t *testing.T) {
	narration := Narration{
		Title: "Mapped",
		Sections: []NarrationSection{{
			ID:               "intro",
			Heading:          "Introduction",
			SourceSectionIDs: []string{"source-99"},
			Sentences:        []string{"A sentence."},
		}},
	}
	err := validateSourceMappings(narration, []SourceSection{{ID: "source-0"}})
	if err == nil {
		t.Fatal("validateSourceMappings() error = nil, want an error")
	}
}
