package e2e

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

// TestManifest_LoadsAndValidates pins the contract: the shipped
// soyapack.yaml must pass the authoritative pkg/soyapack validator.
func TestManifest_LoadsAndValidates(t *testing.T) {
	m := loadValidManifest(t)
	if m.Name != "essay-tutor" {
		t.Errorf("manifest name = %q, want essay-tutor", m.Name)
	}
	if m.Expose == nil || m.Expose.VirtualModelID != "soya:compo" {
		t.Errorf("expose.virtual_model_id missing or wrong: %+v", m.Expose)
	}
	if m.Prompt == nil || len(m.Prompt.Steps) != 3 {
		t.Fatalf("prompt.steps: want 3-step chain, got %+v", m.Prompt)
	}
	wantSteps := []string{"analyze", "generate", "refine"}
	for i, step := range m.Prompt.Steps {
		if step.ID != wantSteps[i] {
			t.Errorf("prompt.steps[%d].id = %q, want %q", i, step.ID, wantSteps[i])
		}
	}
}

// TestE2E_PromptChain runs each sample essay through the real surface
// (OpenAI-compat gateway → kernel 3-step chain → mock upstream) and checks
// stage threading, the final guide.v1 shape, and the font-critical content.
func TestE2E_PromptChain(t *testing.T) {
	for _, s := range samples {
		t.Run(fmt.Sprintf("sample-%d", s.ID), func(t *testing.T) {
			up := newMockUpstream(t, s)
			gw := startGateway(t, up.server.URL)

			final := gw.chatCompletion(t, userSubmission(t, s))

			if fails := up.Failures(); len(fails) > 0 {
				t.Fatalf("mock upstream observed protocol violations: %v", fails)
			}
			calls := up.Calls()
			if len(calls) != 3 {
				t.Fatalf("upstream calls = %d, want 3 (analyze → generate → refine)", len(calls))
			}

			// Stage order is fixed by soyapack.yaml prompt.steps.
			wantStage := []string{"# analyze_sample", "# generate_guide", "# refine_for_grade"}
			for i, c := range calls {
				if !strings.Contains(c.System, wantStage[i]) {
					t.Errorf("call %d system prompt is not %s (got %q…)", i, wantStage[i], firstLine(c.System))
				}
				if c.Model != "mock-compo-llm" {
					t.Errorf("call %d model = %q, want resolved upstream model mock-compo-llm (virtual id must not leak)", i, c.Model)
				}
				if !c.Stream {
					t.Errorf("call %d not streaming; chain stages must stream (see pack_agent.go)", i)
				}
			}

			// Stage 1 receives the parent submission (title + essay).
			if !strings.Contains(calls[0].User, "标题："+s.Title) {
				t.Errorf("stage 1 user payload missing title line 标题：%s", s.Title)
			}
			if !strings.Contains(calls[0].User, s.EssayMarker) {
				t.Errorf("stage 1 user payload missing essay marker %q", s.EssayMarker)
			}
			// Stage 2 receives stage 1's full response, verbatim.
			if got, want := strings.TrimSpace(calls[1].User), strings.TrimSpace(mustRead(t, s.AnalyzeFile)); got != want {
				t.Errorf("stage 2 user payload != stage 1 response\ngot:  %s\nwant: %s", truncate(got, 300), truncate(want, 300))
			}
			// Stage 3 receives stage 2's full response, verbatim.
			if got, want := strings.TrimSpace(calls[2].User), strings.TrimSpace(mustRead(t, s.GuideFile)); got != want {
				t.Errorf("stage 3 user payload != stage 2 response\ngot:  %s\nwant: %s", truncate(got, 300), truncate(want, 300))
			}
			// The caller sees exactly the final (refine) stage output.
			if got, want := strings.TrimSpace(final), strings.TrimSpace(mustRead(t, s.RefinedFile)); got != want {
				t.Errorf("gateway response != refine stage output\ngot:  %s\nwant: %s", truncate(got, 300), truncate(want, 300))
			}

			g, err := ParseGuide(final)
			if err != nil {
				t.Fatalf("final output is not guide.v1: %v", err)
			}
			if err := g.Validate(); err != nil {
				t.Fatalf("final guide.v1 invalid: %v", err)
			}
			if g.Title != s.Title {
				t.Errorf("guide title = %q, want %q", g.Title, s.Title)
			}
		})
	}
}

// TestE2E_PromptChain_LiveUpstream is the opt-in live-model variant. It is
// skipped unless COMPO_E2E_LIVE=1 AND the operator has exported real
// SOYA_MODEL_API_KEY / SOYA_MODEL_BASE_URL / SOYA_MODEL_DEFAULT. Budget
// discipline: never run this in CI loops; one essay, one run.
func TestE2E_PromptChain_LiveUpstream(t *testing.T) {
	if os.Getenv("COMPO_E2E_LIVE") != "1" {
		t.Skip("set COMPO_E2E_LIVE=1 (plus SOYA_MODEL_* env) to run against a real upstream")
	}
	if os.Getenv("SOYA_MODEL_API_KEY") == "" {
		t.Fatal("COMPO_E2E_LIVE=1 but SOYA_MODEL_API_KEY is empty")
	}
	s := samples[0]
	gw := startLiveGateway(t)
	final := gw.chatCompletion(t, userSubmission(t, s))
	g, err := ParseGuide(final)
	if err != nil {
		t.Fatalf("live output is not guide.v1: %v\nraw: %s", err, truncate(final, 800))
	}
	if err := g.Validate(); err != nil {
		t.Fatalf("live guide.v1 invalid: %v", err)
	}
	t.Logf("live chain OK: title=%q vocab=%d phrases=%d", g.Title, len(g.Vocabulary), len(g.GoodPhrases))
}
