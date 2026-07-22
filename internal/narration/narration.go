package narration

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

type Narration struct {
	Title            string             `json:"title"`
	WhyItMatters     string             `json:"why_it_matters,omitempty"`
	WhatToListenFor  []string           `json:"what_to_listen_for,omitempty"`
	EstimatedMinutes int                `json:"estimated_minutes,omitempty"`
	Sections         []NarrationSection `json:"sections"`
	Remember         []string           `json:"remember,omitempty"`
	Decisions        []string           `json:"decisions,omitempty"`
	Actions          []string           `json:"actions,omitempty"`
	Verify           []string           `json:"verify,omitempty"`
}

type NarrationSection struct {
	ID               string   `json:"id"`
	Heading          string   `json:"heading"`
	SourceSectionIDs []string `json:"source_section_ids"`
	Sentences        []string `json:"sentences"`
	RecallQuestion   string   `json:"recall_question,omitempty"`
}

type claudeEnvelope struct {
	StructuredOutput json.RawMessage `json:"structured_output"`
	Result           string          `json:"result"`
}

func parseClaudeResponse(data []byte) (Narration, error) {
	var envelope claudeEnvelope
	if err := json.Unmarshal(data, &envelope); err != nil {
		return Narration{}, fmt.Errorf("decoding Claude response envelope: %w", err)
	}

	raw := bytes.TrimSpace(envelope.StructuredOutput)
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		raw = []byte(strings.TrimSpace(envelope.Result))
	}
	if len(raw) == 0 {
		return Narration{}, errors.New("Claude response did not contain structured output")
	}

	var narration Narration
	if err := json.Unmarshal(raw, &narration); err != nil {
		return Narration{}, fmt.Errorf("decoding narration: %w", err)
	}
	if err := validateNarration(narration); err != nil {
		return Narration{}, err
	}
	return narration, nil
}

func validateNarration(n Narration) error {
	if strings.TrimSpace(n.Title) == "" {
		return errors.New("narration title is empty")
	}
	if len(n.Sections) == 0 {
		return errors.New("narration contains no sections")
	}
	seen := make(map[string]struct{}, len(n.Sections))
	for i, section := range n.Sections {
		if strings.TrimSpace(section.ID) == "" {
			return fmt.Errorf("narration section %d has no id", i+1)
		}
		if _, ok := seen[section.ID]; ok {
			return fmt.Errorf("narration section id %q is duplicated", section.ID)
		}
		seen[section.ID] = struct{}{}
		if strings.TrimSpace(section.Heading) == "" {
			return fmt.Errorf("narration section %q has no heading", section.ID)
		}
		if len(section.Sentences) == 0 {
			return fmt.Errorf("narration section %q contains no sentences", section.ID)
		}
		for j, sentence := range section.Sentences {
			if strings.TrimSpace(sentence) == "" {
				return fmt.Errorf("narration section %q sentence %d is empty", section.ID, j+1)
			}
		}
	}
	return nil
}

func validateSourceMappings(narration Narration, sources []SourceSection) error {
	valid := make(map[string]struct{}, len(sources))
	for _, source := range sources {
		valid[source.ID] = struct{}{}
	}
	for _, section := range narration.Sections {
		if len(section.SourceSectionIDs) == 0 {
			return fmt.Errorf("narration section %q has no source mapping", section.ID)
		}
		for _, sourceID := range section.SourceSectionIDs {
			if _, ok := valid[sourceID]; !ok {
				return fmt.Errorf("narration section %q references unknown source %q", section.ID, sourceID)
			}
		}
	}
	return nil
}

func ValidateSourceMappings(narration Narration, sources []SourceSection) error {
	return validateSourceMappings(narration, sources)
}
