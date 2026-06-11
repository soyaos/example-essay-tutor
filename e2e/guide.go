// Package e2e is the end-to-end test suite for the Compo SoyaPack
// (example-essay-tutor). It exercises the real production path:
//
//	OpenAI-Compat gateway (pkg/openaicompat)
//	  → kernel pack-agent 3-step prompt chain (pkg/kernel)
//	    → OpenAI-compat upstream (mocked in-process by default; see
//	      TestE2E_PromptChain_LiveUpstream for the opt-in live mode)
//	  → guide.v1 JSON
//	  → HTML / PDF artifact render (pkg/artifact, headless Chrome for PDF)
//	  → Chinese glyph coverage scan (canvas tofu detection in Chrome)
//
// Only the upstream LLM is substituted; everything else is the real
// SoyaOS runtime code that serves `soya:compo` in production.
package e2e

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Guide is the guide.v1 artifact snapshot. Field names match the dot paths
// used by templates/guide.html.tmpl and templates/guide.pdf.tmpl; JSON tags
// match the wire format the prompt chain emits (see prompts/generate_guide.md).
type Guide struct {
	Title            string         `json:"title"`
	OpeningDirection string         `json:"opening_direction"`
	WritingPoints    []WritingPoint `json:"writing_points"`
	Vocabulary       []VocabEntry   `json:"vocabulary"`
	GoodPhrases      []GoodPhrase   `json:"good_phrases"`
	Pitfalls         []Pitfall      `json:"pitfalls"`
	SampleParagraph  string         `json:"sample_paragraph"`
}

// WritingPoint is one of the three "写作要点" cards.
type WritingPoint struct {
	TitleZH string `json:"title_zh"`
	BodyZH  string `json:"body_zh"`
}

// VocabEntry is one of the eight "好词" rows.
type VocabEntry struct {
	Word      string `json:"word"`
	MeaningZH string `json:"meaning_zh"`
}

// GoodPhrase is one of the five "好句" rows.
type GoodPhrase struct {
	Phrase string `json:"phrase"`
	WhyZH  string `json:"why_zh"`
}

// Pitfall is one of the three "要避免的坑" rows.
type Pitfall struct {
	TitleZH string `json:"title_zh"`
	FixZH   string `json:"fix_zh"`
}

// ExtractFencedJSON returns the body of the first ```json fenced block in s.
// The prompt contract (generate_guide.md / refine_for_grade.md) requires the
// model to emit exactly one such block and nothing else.
func ExtractFencedJSON(s string) (string, error) {
	const open = "```json"
	start := strings.Index(s, open)
	if start < 0 {
		return "", fmt.Errorf("no ```json fence in model output (%d bytes)", len(s))
	}
	rest := s[start+len(open):]
	end := strings.Index(rest, "```")
	if end < 0 {
		return "", fmt.Errorf("unterminated ```json fence in model output")
	}
	return strings.TrimSpace(rest[:end]), nil
}

// ParseGuide extracts and unmarshals the guide.v1 JSON from a raw model
// response (fenced or bare JSON).
func ParseGuide(raw string) (Guide, error) {
	body, err := ExtractFencedJSON(raw)
	if err != nil {
		// Tolerate bare JSON (some upstreams strip fences).
		body = strings.TrimSpace(raw)
	}
	var g Guide
	if err := json.Unmarshal([]byte(body), &g); err != nil {
		return Guide{}, fmt.Errorf("parse guide.v1 JSON: %w", err)
	}
	return g, nil
}

// Validate enforces the hard guide.v1 shape constraints the templates rely
// on (see prompts/generate_guide.md "数量是硬约束"): 3 writing points,
// 8 vocabulary entries, 5 good phrases, 3 pitfalls, and no empty fields.
func (g Guide) Validate() error {
	var errs []string
	if strings.TrimSpace(g.Title) == "" {
		errs = append(errs, "title is empty")
	}
	if strings.TrimSpace(g.OpeningDirection) == "" {
		errs = append(errs, "opening_direction is empty")
	}
	if strings.TrimSpace(g.SampleParagraph) == "" {
		errs = append(errs, "sample_paragraph is empty")
	}
	if n := len(g.WritingPoints); n != 3 {
		errs = append(errs, fmt.Sprintf("writing_points: want 3, got %d", n))
	}
	if n := len(g.Vocabulary); n != 8 {
		errs = append(errs, fmt.Sprintf("vocabulary: want 8, got %d", n))
	}
	if n := len(g.GoodPhrases); n != 5 {
		errs = append(errs, fmt.Sprintf("good_phrases: want 5, got %d", n))
	}
	if n := len(g.Pitfalls); n != 3 {
		errs = append(errs, fmt.Sprintf("pitfalls: want 3, got %d", n))
	}
	for i, p := range g.WritingPoints {
		if strings.TrimSpace(p.TitleZH) == "" || strings.TrimSpace(p.BodyZH) == "" {
			errs = append(errs, fmt.Sprintf("writing_points[%d] has empty field", i))
		}
	}
	for i, v := range g.Vocabulary {
		if strings.TrimSpace(v.Word) == "" || strings.TrimSpace(v.MeaningZH) == "" {
			errs = append(errs, fmt.Sprintf("vocabulary[%d] has empty field", i))
		}
	}
	for i, p := range g.GoodPhrases {
		if strings.TrimSpace(p.Phrase) == "" || strings.TrimSpace(p.WhyZH) == "" {
			errs = append(errs, fmt.Sprintf("good_phrases[%d] has empty field", i))
		}
	}
	for i, p := range g.Pitfalls {
		if strings.TrimSpace(p.TitleZH) == "" || strings.TrimSpace(p.FixZH) == "" {
			errs = append(errs, fmt.Sprintf("pitfalls[%d] has empty field", i))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("guide.v1 shape violations: %s", strings.Join(errs, "; "))
	}
	return nil
}
