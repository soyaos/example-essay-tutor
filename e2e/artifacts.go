package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/soyaos/soyaos/pkg/artifact"
)

// ArtifactPaths identifies the validated parent-trial outputs written by
// RenderGuideArtifacts.
type ArtifactPaths struct {
	JSON string
	HTML string
	PDF  string
}

// RenderGuideArtifacts validates a guide.v1 value and writes the canonical
// JSON, browser-preview HTML, and printable PDF files into outDir.
func RenderGuideArtifacts(ctx context.Context, guide Guide, templateDir, outDir string) (ArtifactPaths, error) {
	if err := guide.Validate(); err != nil {
		return ArtifactPaths{}, err
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return ArtifactPaths{}, fmt.Errorf("create artifact directory: %w", err)
	}

	paths := ArtifactPaths{
		JSON: filepath.Join(outDir, "guide.json"),
		HTML: filepath.Join(outDir, "guide.html"),
		PDF:  filepath.Join(outDir, "guide.pdf"),
	}

	guideJSON, err := json.MarshalIndent(guide, "", "  ")
	if err != nil {
		return ArtifactPaths{}, fmt.Errorf("marshal guide.v1: %w", err)
	}
	if err := os.WriteFile(paths.JSON, guideJSON, 0o644); err != nil {
		return ArtifactPaths{}, fmt.Errorf("write guide JSON: %w", err)
	}

	htmlTemplate, err := os.ReadFile(filepath.Join(templateDir, "guide.html.tmpl"))
	if err != nil {
		return ArtifactPaths{}, fmt.Errorf("read HTML template: %w", err)
	}
	var html bytes.Buffer
	if _, err := (artifact.HTMLRenderer{Template: string(htmlTemplate), Schema: "guide.v1"}).Render(ctx, guide, &html); err != nil {
		return ArtifactPaths{}, fmt.Errorf("render HTML: %w", err)
	}
	if html.Len() == 0 {
		return ArtifactPaths{}, fmt.Errorf("render HTML: empty output")
	}
	if err := os.WriteFile(paths.HTML, html.Bytes(), 0o644); err != nil {
		return ArtifactPaths{}, fmt.Errorf("write HTML: %w", err)
	}

	pdfTemplate, err := os.ReadFile(filepath.Join(templateDir, "guide.pdf.tmpl"))
	if err != nil {
		return ArtifactPaths{}, fmt.Errorf("read PDF template: %w", err)
	}
	var pdf bytes.Buffer
	if _, err := (artifact.PDFRenderer{Template: string(pdfTemplate), Schema: "guide.v1"}).Render(ctx, guide, &pdf); err != nil {
		return ArtifactPaths{}, fmt.Errorf("render PDF: %w", err)
	}
	if pdf.Len() < 30*1024 || !bytes.HasPrefix(pdf.Bytes(), []byte("%PDF-")) {
		return ArtifactPaths{}, fmt.Errorf("render PDF: invalid or unexpectedly small output (%d bytes)", pdf.Len())
	}
	if err := os.WriteFile(paths.PDF, pdf.Bytes(), 0o644); err != nil {
		return ArtifactPaths{}, fmt.Errorf("write PDF: %w", err)
	}

	return paths, nil
}
