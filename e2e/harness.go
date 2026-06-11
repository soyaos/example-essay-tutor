package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/soyaos/soyaos/pkg/artifact"
	"github.com/soyaos/soyaos/pkg/auth"
	"github.com/soyaos/soyaos/pkg/kernel"
	"github.com/soyaos/soyaos/pkg/openaicompat"
	"github.com/soyaos/soyaos/pkg/soyapack"
)

// repoRoot is the example-essay-tutor checkout, relative to this package.
const repoRoot = ".."

// renderedDir is where rendered artifacts are archived (committed to the
// repo as the font-rendering evidence required by APP-1058).
const renderedDir = "../examples/rendered"

// sample describes one Chinese sample essay and its per-stage mock fixtures.
type sample struct {
	ID    int
	Title string
	// InputFile is the parent-submitted essay under examples/.
	InputFile string
	// EssayMarker is a substring of the essay that must surface in the
	// stage-1 user payload (proves the gateway threaded the submission).
	EssayMarker string
	// Fixture files: the canned model response for each chain stage.
	AnalyzeFile, GuideFile, RefinedFile string
	// MustRender lists strings that must appear in the final rendered
	// HTML — used to pin the font-critical content (生僻字 / 标点混排).
	MustRender []string
}

var samples = []sample{
	{
		ID:          1,
		Title:       "难忘的一次劳动",
		InputFile:   "../examples/sample-input-1.txt",
		EssayMarker: "第一次握锄头",
		AnalyzeFile: "testdata/analyze-1.md",
		GuideFile:   "testdata/guide-1.md",
		RefinedFile: "testdata/refined-1.md",
		MustRender:  []string{"难忘的一次劳动", "汗津津", "踉跄", "黝黑", "田埂"},
	},
	{
		ID:          2,
		Title:       "外婆的灶台",
		InputFile:   "../examples/sample-input-2.txt",
		EssayMarker: "揭开木甑子的盖",
		AnalyzeFile: "testdata/analyze-2.md",
		GuideFile:   "testdata/guide-2.md",
		RefinedFile: "testdata/refined-2.md",
		// 生僻字必须进入最终产物，缺一即 fail。
		MustRender: []string{"外婆的灶台", "氤氲", "馥郁", "皴裂", "摩挲", "黢黑", "佝偻", "煨", "甑", "醪糟", "糍粑", "笸箩", "簸箕"},
	},
	{
		ID:          3,
		Title:       "我和 AI 下了一盘棋",
		InputFile:   "../examples/sample-input-3.txt",
		EssayMarker: "AlphaGo",
		AnalyzeFile: "testdata/analyze-3.md",
		GuideFile:   "testdata/guide-3.md",
		RefinedFile: "testdata/refined-3.md",
		// 全角/半角标点与中英数字混排必须进入最终产物。
		MustRender: []string{"我和 AI 下了一盘棋", "19×19", "0.1 秒", "胜率 3%", "25.5 目", "……", "？！", "《围棋入门》", "——"},
	},
}

func mustRead(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

// --- mock OpenAI-compat upstream -------------------------------------------

// upstreamCall records one /chat/completions request the chain made.
type upstreamCall struct {
	Model  string
	Stream bool
	System string
	User   string
}

// mockUpstream is an in-process OpenAI-compat /chat/completions SSE server
// preloaded with the three stage fixtures of a single sample. Stage routing
// is by the system prompt header (each prompt file starts with a unique
// "# <step> — 第 N 步" line), exactly what the kernel injects per stage.
type mockUpstream struct {
	mu       sync.Mutex
	calls    []upstreamCall
	analyze  string
	guide    string
	refined  string
	server   *httptest.Server
	failures []string
}

func newMockUpstream(t *testing.T, s sample) *mockUpstream {
	m := &mockUpstream{
		analyze: mustRead(t, s.AnalyzeFile),
		guide:   mustRead(t, s.GuideFile),
		refined: mustRead(t, s.RefinedFile),
	}
	m.server = httptest.NewServer(http.HandlerFunc(m.handle))
	t.Cleanup(m.server.Close)
	return m
}

func (m *mockUpstream) handle(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/chat/completions" || r.Method != http.MethodPost {
		m.noteFailure(fmt.Sprintf("unexpected upstream request %s %s", r.Method, r.URL.Path))
		http.NotFound(w, r)
		return
	}
	var req struct {
		Model    string `json:"model"`
		Stream   bool   `json:"stream"`
		Messages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		m.noteFailure("bad upstream request body: " + err.Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	var system, user string
	for _, msg := range req.Messages {
		switch msg.Role {
		case "system":
			system = msg.Content
		case "user":
			user = msg.Content
		}
	}
	m.mu.Lock()
	m.calls = append(m.calls, upstreamCall{Model: req.Model, Stream: req.Stream, System: system, User: user})
	m.mu.Unlock()

	var reply string
	switch {
	case strings.Contains(system, "# analyze_sample"):
		reply = m.analyze
	case strings.Contains(system, "# generate_guide"):
		reply = m.guide
	case strings.Contains(system, "# refine_for_grade"):
		reply = m.refined
	default:
		m.noteFailure("system prompt matched no known stage: " + firstLine(system))
		http.Error(w, "unknown stage", http.StatusBadRequest)
		return
	}
	writeSSE(w, reply)
}

func (m *mockUpstream) noteFailure(msg string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.failures = append(m.failures, msg)
}

// Calls returns a snapshot of recorded upstream calls.
func (m *mockUpstream) Calls() []upstreamCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]upstreamCall(nil), m.calls...)
}

// Failures returns protocol violations observed by the mock.
func (m *mockUpstream) Failures() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]string(nil), m.failures...)
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

// writeSSE streams reply as OpenAI-shaped SSE chunks. The content is split
// into several chunks on purpose: the kernel's streamCollect must reassemble
// them, exactly as it does against DashScope/OpenAI in production.
func writeSSE(w http.ResponseWriter, reply string) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.WriteHeader(http.StatusOK)
	flusher, _ := w.(http.Flusher)

	emit := func(v any) {
		b, _ := json.Marshal(v)
		fmt.Fprintf(w, "data: %s\n\n", b)
		if flusher != nil {
			flusher.Flush()
		}
	}
	type delta struct {
		Content string `json:"content"`
	}
	type choice struct {
		Delta        delta   `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	}
	type frame struct {
		Choices []choice `json:"choices"`
	}

	runes := []rune(reply)
	const chunkRunes = 64
	for i := 0; i < len(runes); i += chunkRunes {
		end := i + chunkRunes
		if end > len(runes) {
			end = len(runes)
		}
		emit(frame{Choices: []choice{{Delta: delta{Content: string(runes[i:end])}}}})
	}
	stop := "stop"
	emit(frame{Choices: []choice{{FinishReason: &stop}}})
	fmt.Fprint(w, "data: [DONE]\n\n")
	if flusher != nil {
		flusher.Flush()
	}
}

// --- gateway harness --------------------------------------------------------

// gateway is a fully wired SoyaOS surface: manifest-registered kernel behind
// the OpenAI-compat HTTP gateway, authenticated with the seeded dev key.
type gateway struct {
	URL    string
	APIKey string
}

// startGateway loads + validates the repo manifest, registers the pack on a
// fresh kernel (provider pointed at the given upstream base URL via
// SOYA_MODEL_* env), and exposes it through pkg/openaicompat.
func startGateway(t *testing.T, upstreamBaseURL string) gateway {
	t.Helper()

	t.Setenv("SOYA_MODEL_API_KEY", "sk-e2e-upstream-dummy")
	t.Setenv("SOYA_MODEL_BASE_URL", upstreamBaseURL)
	t.Setenv("SOYA_MODEL_DEFAULT", "mock-compo-llm")

	m := loadValidManifest(t)

	packDir, err := filepath.Abs(repoRoot)
	if err != nil {
		t.Fatalf("abs repo root: %v", err)
	}
	k := kernel.New()
	if err := k.RegisterFromPack(m, packDir); err != nil {
		t.Fatalf("RegisterFromPack: %v", err)
	}

	store := auth.NewMemoryStore()
	key := store.SeedDevKey()

	srv := httptest.NewServer(openaicompat.NewServer(k, store).Handler())
	t.Cleanup(srv.Close)
	return gateway{URL: srv.URL, APIKey: key}
}

// startLiveGateway wires the same surface but leaves SOYA_MODEL_* env
// untouched, so the chain talks to whatever real upstream the operator
// exported. Used only by the opt-in live test.
func startLiveGateway(t *testing.T) gateway {
	t.Helper()
	m := loadValidManifest(t)
	packDir, err := filepath.Abs(repoRoot)
	if err != nil {
		t.Fatalf("abs repo root: %v", err)
	}
	k := kernel.New()
	if err := k.RegisterFromPack(m, packDir); err != nil {
		t.Fatalf("RegisterFromPack: %v", err)
	}
	store := auth.NewMemoryStore()
	key := store.SeedDevKey()
	srv := httptest.NewServer(openaicompat.NewServer(k, store).Handler())
	t.Cleanup(srv.Close)
	return gateway{URL: srv.URL, APIKey: key}
}

func loadValidManifest(t *testing.T) *soyapack.Manifest {
	t.Helper()
	m, err := soyapack.LoadFromFile(filepath.Join(repoRoot, "soyapack.yaml"))
	if err != nil {
		t.Fatalf("load soyapack.yaml: %v", err)
	}
	if err := soyapack.Validate(m); err != nil {
		t.Fatalf("validate soyapack.yaml: %v", err)
	}
	return m
}

// chatCompletion submits one essay through POST /v1/chat/completions
// (non-stream) and returns choices[0].message.content — the exact call shape
// from the README quickstart.
func (g gateway) chatCompletion(t *testing.T, userContent string) string {
	t.Helper()
	body, _ := json.Marshal(map[string]any{
		"model": "soya:compo",
		"messages": []map[string]string{
			{"role": "user", "content": userContent},
		},
	})
	req, err := http.NewRequest(http.MethodPost, g.URL+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+g.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /v1/chat/completions: %v", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("gateway returned %d: %s", resp.StatusCode, truncate(string(raw), 500))
	}
	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("parse chat completion: %v\nbody: %s", err, truncate(string(raw), 500))
	}
	if len(parsed.Choices) == 0 {
		t.Fatalf("chat completion has no choices: %s", truncate(string(raw), 500))
	}
	return parsed.Choices[0].Message.Content
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// userSubmission formats the parent's submission for one sample: a title
// line plus the raw essay text, like the README quickstart.
func userSubmission(t *testing.T, s sample) string {
	return "标题：" + s.Title + "\n\n" + mustRead(t, s.InputFile)
}

// --- shared artifact rendering ----------------------------------------------

type renderedSample struct {
	Sample        sample
	Guide         Guide
	HTMLPath      string // web template render
	PrintHTMLPath string // print template render (the HTML the PDF is printed from)
	PDFPath       string // A4 print render (via headless Chrome)
}

var (
	renderOnce sync.Once
	renderErr  error
	renderedAr []renderedSample
)

// renderAllSamples runs the full chain for every sample and renders the
// final guide.v1 through both real renderers, archiving the artifacts under
// examples/rendered/. Cached across tests in one `go test` run.
func renderAllSamples(t *testing.T) []renderedSample {
	t.Helper()
	renderOnce.Do(func() {
		renderErr = doRenderAll(t)
	})
	if renderErr != nil {
		t.Fatalf("render samples: %v", renderErr)
	}
	return renderedAr
}

func doRenderAll(t *testing.T) error {
	if err := os.MkdirAll(renderedDir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", renderedDir, err)
	}
	htmlTmpl := mustRead(t, filepath.Join(repoRoot, "templates", "guide.html.tmpl"))
	pdfTmpl := mustRead(t, filepath.Join(repoRoot, "templates", "guide.pdf.tmpl"))

	for _, s := range samples {
		// Run the real chain to obtain the final guide for this sample.
		up := newMockUpstream(t, s)
		gw := startGateway(t, up.server.URL)
		final := gw.chatCompletion(t, userSubmission(t, s))
		g, err := ParseGuide(final)
		if err != nil {
			return fmt.Errorf("sample %d: %w", s.ID, err)
		}
		if err := g.Validate(); err != nil {
			return fmt.Errorf("sample %d: %w", s.ID, err)
		}

		htmlPath := filepath.Join(renderedDir, fmt.Sprintf("guide-%d.html", s.ID))
		var htmlBuf bytes.Buffer
		art, err := artifact.HTMLRenderer{Template: htmlTmpl, Schema: "guide.v1"}.Render(context.Background(), g, &htmlBuf)
		if err != nil {
			return fmt.Errorf("sample %d: render html: %w", s.ID, err)
		}
		if art.MIMEType != "text/html; charset=utf-8" {
			return fmt.Errorf("sample %d: html artifact mime = %q", s.ID, art.MIMEType)
		}
		if err := os.WriteFile(htmlPath, htmlBuf.Bytes(), 0o644); err != nil {
			return fmt.Errorf("sample %d: write %s: %w", s.ID, htmlPath, err)
		}

		// Also archive the print-variant HTML — the exact document the PDF
		// renderer prints — so the glyph scan can cover both templates.
		printHTMLPath := filepath.Join(renderedDir, fmt.Sprintf("guide-%d.print.html", s.ID))
		var printBuf bytes.Buffer
		if _, err := (artifact.HTMLRenderer{Template: pdfTmpl, Schema: "guide.v1"}).Render(context.Background(), g, &printBuf); err != nil {
			return fmt.Errorf("sample %d: render print html: %w", s.ID, err)
		}
		if err := os.WriteFile(printHTMLPath, printBuf.Bytes(), 0o644); err != nil {
			return fmt.Errorf("sample %d: write %s: %w", s.ID, printHTMLPath, err)
		}

		pdfPath := filepath.Join(renderedDir, fmt.Sprintf("guide-%d.pdf", s.ID))
		var pdfBuf bytes.Buffer
		art, err = artifact.PDFRenderer{Template: pdfTmpl, Schema: "guide.v1"}.Render(context.Background(), g, &pdfBuf)
		if err != nil {
			return fmt.Errorf("sample %d: render pdf (headless Chrome required, set SOYAOS_CHROME if not auto-detected): %w", s.ID, err)
		}
		if art.MIMEType != "application/pdf" {
			return fmt.Errorf("sample %d: pdf artifact mime = %q", s.ID, art.MIMEType)
		}
		if err := os.WriteFile(pdfPath, pdfBuf.Bytes(), 0o644); err != nil {
			return fmt.Errorf("sample %d: write %s: %w", s.ID, pdfPath, err)
		}

		renderedAr = append(renderedAr, renderedSample{Sample: s, Guide: g, HTMLPath: htmlPath, PrintHTMLPath: printHTMLPath, PDFPath: pdfPath})
	}
	return nil
}
