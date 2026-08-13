package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"

	compoe2e "github.com/soyaos/example-essay-tutor/e2e"
)

func main() {
	templateDir := flag.String("templates", "../templates", "directory containing the Compo HTML/PDF templates")
	outDir := flag.String("out", "../trial-output", "directory for guide.json, guide.html, and guide.pdf")
	flag.Parse()

	raw, err := io.ReadAll(os.Stdin)
	if err != nil {
		fatalf("read guide.v1 from stdin: %v", err)
	}
	guide, err := compoe2e.ParseGuide(string(raw))
	if err != nil {
		fatalf("%v", err)
	}
	paths, err := compoe2e.RenderGuideArtifacts(context.Background(), guide, *templateDir, *outDir)
	if err != nil {
		fatalf("%v", err)
	}
	fmt.Printf("guide.v1 validated and rendered\nJSON: %s\nHTML: %s\nPDF:  %s\n", paths.JSON, paths.HTML, paths.PDF)
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "render-guide: "+format+"\n", args...)
	os.Exit(1)
}
