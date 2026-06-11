package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
)

// tofuScanJS walks every text node of the loaded document, collects each
// unique (non-ASCII character, computed font-family) pair, and rasterises
// the character on a canvas with that exact font stack. A character whose
// raster equals the .notdef reference (U+FDD0, a Unicode noncharacter no
// font maps) or the zero-ink reference is reported as missing — i.e. it
// would display as a tofu box or as nothing.
const tofuScanJS = `(() => {
  const pairs = new Map();
  const walker = document.createTreeWalker(document.body, NodeFilter.SHOW_TEXT);
  while (walker.nextNode()) {
    const node = walker.currentNode;
    const parent = node.parentElement;
    if (!parent) continue;
    const font = getComputedStyle(parent).fontFamily;
    for (const ch of node.textContent) {
      if (!/\S/.test(ch)) continue;
      if (ch.codePointAt(0) < 0x80) continue; // ASCII never falls back to tofu here
      pairs.set(font + '|' + ch, { font, ch });
    }
  }
  const canvas = document.createElement('canvas');
  canvas.width = 72; canvas.height = 72;
  const ctx = canvas.getContext('2d', { willReadFrequently: true });
  const draw = (ch, font) => {
    ctx.clearRect(0, 0, 72, 72);
    ctx.fillStyle = '#000';
    ctx.textBaseline = 'top';
    ctx.font = '48px ' + font;
    if (ch) ctx.fillText(ch, 4, 8);
    return canvas.toDataURL();
  };
  const refs = new Map();
  const missing = [];
  for (const { font, ch } of pairs.values()) {
    if (!refs.has(font)) {
      refs.set(font, { tofu: draw('\uFDD0', font), blank: draw('', font) });
    }
    const ref = refs.get(font);
    const got = draw(ch, font);
    if (got === ref.tofu || got === ref.blank) {
      missing.push(ch + ' U+' + ch.codePointAt(0).toString(16).toUpperCase());
    }
  }
  return JSON.stringify({ scanned: pairs.size, missing });
})()`

type tofuReport struct {
	Scanned int      `json:"scanned"`
	Missing []string `json:"missing"`
}

// runTofuScan loads an HTML file in headless Chrome, runs the glyph scan,
// and (optionally) captures a full-page screenshot.
func runTofuScan(t *testing.T, chrome, htmlPath string, screenshot bool) (tofuReport, []byte) {
	t.Helper()
	abs, err := filepath.Abs(htmlPath)
	if err != nil {
		t.Fatalf("abs %s: %v", htmlPath, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	allocOpts := append([]chromedp.ExecAllocatorOption{}, chromedp.DefaultExecAllocatorOptions[:]...)
	allocOpts = append(allocOpts, chromedp.ExecPath(chrome))
	allocCtx, cancelAlloc := chromedp.NewExecAllocator(ctx, allocOpts...)
	defer cancelAlloc()
	browserCtx, cancelBrowser := chromedp.NewContext(allocCtx)
	defer cancelBrowser()

	var rawReport string
	var shot []byte
	actions := []chromedp.Action{
		chromedp.Navigate("file://" + abs),
		chromedp.WaitReady("body", chromedp.ByQuery),
		chromedp.EvaluateAsDevTools("document.fonts.ready", nil),
		chromedp.Evaluate(tofuScanJS, &rawReport),
	}
	if screenshot {
		actions = append(actions, chromedp.FullScreenshot(&shot, 90))
	}
	if err := chromedp.Run(browserCtx, actions...); err != nil {
		t.Fatalf("chrome glyph scan: %v", err)
	}
	var rep tofuReport
	if err := json.Unmarshal([]byte(rawReport), &rep); err != nil {
		t.Fatalf("parse scan report %q: %v", truncate(rawReport, 200), err)
	}
	return rep, shot
}

// TestTofuScanner_SelfCheck proves the detector is not vacuously green: a
// codepoint guaranteed to have no glyph anywhere (U+FDD1, a Unicode
// noncharacter) must be flagged, while ordinary CJK must not be.
func TestTofuScanner_SelfCheck(t *testing.T) {
	chrome := findChrome(t)

	page := `<!DOCTYPE html><html lang="zh-CN"><head><meta charset="utf-8"></head>
<body style='font-family: "Plus Jakarta Sans", "PingFang SC", sans-serif'>
<p>汉字正常渲染基线。</p>
<p>缺字探针：&#xFDD1;</p>
</body></html>`
	tmp := filepath.Join(t.TempDir(), "selfcheck.html")
	if err := os.WriteFile(tmp, []byte(page), 0o644); err != nil {
		t.Fatalf("write self-check page: %v", err)
	}

	rep, _ := runTofuScan(t, chrome, tmp, false)
	if rep.Scanned == 0 {
		t.Fatal("self-check scanned nothing")
	}
	foundProbe := false
	for _, m := range rep.Missing {
		if strings.Contains(m, "U+FDD1") {
			foundProbe = true
		}
		if strings.Contains(m, "汉") {
			t.Errorf("detector false positive: ordinary CJK 汉 flagged as missing")
		}
	}
	if !foundProbe {
		t.Fatalf("detector failed to flag the guaranteed-missing glyph U+FDD1; missing=%v", rep.Missing)
	}
	t.Logf("self-check OK: scanned=%d, U+FDD1 flagged, CJK baseline clean", rep.Scanned)
}

// TestE2E_ChineseFontRendering_NoTofu loads each rendered artifact (web and
// print HTML variants) in headless Chrome — the same engine that prints the
// PDFs — and fails on any Chinese character that has no real glyph in the
// effective font stack. It also archives a full-page screenshot per artifact
// as reviewable evidence.
func TestE2E_ChineseFontRendering_NoTofu(t *testing.T) {
	chrome := findChrome(t)

	for _, r := range renderAllSamples(t) {
		for _, page := range []struct {
			label string
			path  string
		}{
			{"web", r.HTMLPath},
			{"print", r.PrintHTMLPath},
		} {
			page := page
			t.Run(fmt.Sprintf("sample-%d-%s", r.Sample.ID, page.label), func(t *testing.T) {
				rep, shot := runTofuScan(t, chrome, page.path, true)

				shotPath := filepath.Join(renderedDir, fmt.Sprintf("guide-%d.%s.png", r.Sample.ID, page.label))
				if err := os.WriteFile(shotPath, shot, 0o644); err != nil {
					t.Fatalf("write screenshot %s: %v", shotPath, err)
				}

				// Sanity: a guide page carries hundreds of CJK chars; a tiny
				// scan count means the page didn't actually render.
				if rep.Scanned < 150 {
					t.Fatalf("glyph scan covered only %d (char,font) pairs — page likely failed to render", rep.Scanned)
				}
				if len(rep.Missing) > 0 {
					t.Errorf("missing glyphs (tofu) in %s: %v", page.path, rep.Missing)
				}
				t.Logf("%s: scanned %d unique (char,font) pairs, 0 tofu; screenshot %s", page.path, rep.Scanned, shotPath)
			})
		}
	}
}

// findChrome mirrors pkg/artifact.PDFRenderer's resolution order: explicit
// SOYAOS_CHROME, PATH lookups, then the macOS application-bundle hints.
func findChrome(t *testing.T) string {
	t.Helper()
	if env := os.Getenv("SOYAOS_CHROME"); env != "" {
		return env
	}
	for _, name := range []string{"chromium-browser", "google-chrome", "Google Chrome"} {
		if p, err := exec.LookPath(name); err == nil {
			return p
		}
	}
	for _, hint := range []string{
		"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
		"/Applications/Chromium.app/Contents/MacOS/Chromium",
	} {
		if _, err := os.Stat(hint); err == nil {
			return hint
		}
	}
	t.Fatal("no Chrome/Chromium found; set SOYAOS_CHROME to an absolute path (required for PDF + font verification)")
	return ""
}
