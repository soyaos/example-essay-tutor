package e2e

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

// TestE2E_RenderArtifacts renders every sample's final guide.v1 through the
// real HTML and PDF renderers and archives the artifacts under
// examples/rendered/ (the committed font-rendering evidence).
func TestE2E_RenderArtifacts(t *testing.T) {
	for _, r := range renderAllSamples(t) {
		r := r
		t.Run(r.HTMLPath, func(t *testing.T) {
			html, err := os.ReadFile(r.HTMLPath)
			if err != nil {
				t.Fatalf("read rendered html: %v", err)
			}
			doc := string(html)

			// HTMLRenderer must have prepended the DESIGN §9 print CSS.
			if !strings.Contains(doc, "@media print") {
				t.Error("rendered HTML missing auto-injected @media print block")
			}
			// The CJK-capable font stack must survive into the artifact.
			if !strings.Contains(doc, `"PingFang SC"`) {
				t.Error(`rendered HTML missing "PingFang SC" in font stack`)
			}
			if got := strings.Count(doc, `class="question"`); got != 6 {
				t.Errorf("rendered HTML guided questions = %d, want 6", got)
			}
			for _, want := range []string{"先体验，再写作", "把孩子的口述变成写作素材", "隐私提醒"} {
				if !strings.Contains(doc, want) {
					t.Errorf("rendered HTML missing parent-coaching section %q", want)
				}
			}
			// Every font-critical string (常用字 / 生僻字 / 标点混排) must be
			// present in the final document.
			for _, want := range r.Sample.MustRender {
				if !strings.Contains(doc, want) {
					t.Errorf("rendered HTML missing %q", want)
				}
			}
			// Template loops must have been fed the full guide shape.
			for _, v := range r.Guide.Vocabulary {
				if !strings.Contains(doc, v.Word) {
					t.Errorf("rendered HTML missing vocabulary word %q", v.Word)
				}
			}
		})

		t.Run(r.PDFPath, func(t *testing.T) {
			pdf, err := os.ReadFile(r.PDFPath)
			if err != nil {
				t.Fatalf("read rendered pdf: %v", err)
			}
			if !bytes.HasPrefix(pdf, []byte("%PDF-")) {
				t.Fatalf("artifact is not a PDF (starts with %q)", truncate(string(pdf), 12))
			}
			// A blank/failed Chrome print of this guide is a few KB; a real
			// render with embedded CJK subsets is far larger.
			if len(pdf) < 30*1024 {
				t.Errorf("PDF suspiciously small (%d bytes) — likely blank page or missing font embedding", len(pdf))
			}
			// Chrome (Skia) embeds the fonts it actually used as subset
			// font programs. If no CJK glyph was drawn from PingFang, the
			// PingFang face never gets embedded — so its presence is direct
			// evidence the Chinese text was shaped with a real CJK font.
			if !bytes.Contains(pdf, []byte("PingFang")) {
				t.Error("PDF has no embedded PingFang subset — Chinese glyphs were not rendered with the expected CJK font")
			}
			if !bytes.Contains(pdf, []byte("FontFile")) {
				t.Error("PDF embeds no font programs at all")
			}
		})
	}
}
