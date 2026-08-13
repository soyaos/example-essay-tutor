package e2e

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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
	if m.Entry != "prompts/fast_guide.md" {
		t.Fatalf("entry = %q, want prompts/fast_guide.md", m.Entry)
	}
	if m.Prompt == nil || len(m.Prompt.Steps) != 0 {
		t.Fatalf("parent-trial profile must use one prompt, got %+v", m.Prompt)
	}
}

// TestE2E_FastPrompt runs each sample essay through the real surface
// (OpenAI-compat gateway → kernel single-prompt path → mock upstream) and
// checks the parent-trial profile's guide.v1 contract.
func TestE2E_FastPrompt(t *testing.T) {
	for _, s := range samples {
		t.Run(fmt.Sprintf("sample-%d", s.ID), func(t *testing.T) {
			up := newMockUpstream(t, s)
			gw := startGateway(t, up.server.URL)

			final := gw.chatCompletion(t, userSubmission(t, s))

			if fails := up.Failures(); len(fails) > 0 {
				t.Fatalf("mock upstream observed protocol violations: %v", fails)
			}
			calls := up.Calls()
			if len(calls) != 1 {
				t.Fatalf("upstream calls = %d, want 1 fast-path call", len(calls))
			}

			call := calls[0]
			if !strings.Contains(call.System, "# fast_guide") {
				t.Errorf("system prompt is not fast_guide (got %q…)", firstLine(call.System))
			}
			if call.Model != "mock-compo-llm" {
				t.Errorf("model = %q, want resolved upstream model mock-compo-llm", call.Model)
			}
			if !call.Stream {
				t.Error("fast prompt must stream through the gateway")
			}
			if call.ResponseFormat != "json_object" {
				t.Errorf("response format = %q, want json_object", call.ResponseFormat)
			}

			if !strings.Contains(call.User, "标题："+s.Title) {
				t.Errorf("user payload missing title line 标题：%s", s.Title)
			}
			if !strings.Contains(call.User, s.EssayMarker) {
				t.Errorf("user payload missing essay marker %q", s.EssayMarker)
			}
			want, err := ExtractFencedJSON(mustRead(t, s.RefinedFile))
			if err != nil {
				t.Fatalf("fixture JSON: %v", err)
			}
			if got := strings.TrimSpace(final); got != want {
				t.Errorf("gateway response != bare guide JSON\ngot:  %s\nwant: %s", truncate(got, 300), truncate(want, 300))
			}
			if strings.Contains(final, "```") {
				t.Error("parent-trial response must be bare JSON, not a Markdown fence")
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

// TestE2E_FastPrompt_LiveUpstream is the opt-in live-model variant. It is
// skipped unless COMPO_E2E_LIVE=1 AND the operator has exported real
// SOYA_MODEL_API_KEY / SOYA_MODEL_BASE_URL / SOYA_MODEL_DEFAULT. Budget
// discipline: never run this in CI loops; one essay, one run.
func TestE2E_FastPrompt_LiveUpstream(t *testing.T) {
	if os.Getenv("COMPO_E2E_LIVE") != "1" {
		t.Skip("set COMPO_E2E_LIVE=1 (plus SOYA_MODEL_* env) to run against a real upstream")
	}
	if os.Getenv("SOYA_MODEL_API_KEY") == "" {
		t.Fatal("COMPO_E2E_LIVE=1 but SOYA_MODEL_API_KEY is empty")
	}
	if os.Getenv("SOYA_MODEL_ENABLE_THINKING") != "false" {
		t.Fatal("live parent-trial E2E requires SOYA_MODEL_ENABLE_THINKING=false")
	}
	s := samples[0]
	gw := startLiveGateway(t)
	started := time.Now()
	final := gw.chatCompletion(t, userSubmission(t, s))
	g, err := ParseGuide(final)
	if err != nil {
		t.Fatalf("live output is not guide.v1: %v\nraw: %s", err, truncate(final, 800))
	}
	if err := g.Validate(); err != nil {
		t.Fatalf("live guide.v1 invalid: %v", err)
	}

	outDir := os.Getenv("COMPO_E2E_OUTPUT_DIR")
	if outDir == "" {
		outDir = t.TempDir()
	}
	paths, err := RenderGuideArtifacts(context.Background(), g, filepath.Join(repoRoot, "templates"), outDir)
	if err != nil {
		t.Fatalf("render live artifacts: %v", err)
	}

	elapsed := time.Since(started)
	if elapsed > 30*time.Second {
		t.Fatalf("parent-trial artifact latency = %s, want <= 30s", elapsed.Round(time.Millisecond))
	}
	t.Logf("live fast path OK in %s: title=%q; JSON=%s; HTML=%s; PDF=%s", elapsed.Round(time.Millisecond), g.Title, paths.JSON, paths.HTML, paths.PDF)
}
