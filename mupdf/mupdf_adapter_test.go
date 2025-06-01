package mupdf

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const fixturePDF = "../testdata/SaiNageswarS_Resume.pdf"

func TestExtractTextFile(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "out.txt")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := ExtractTextFile(ctx, fixturePDF, tmp, 1)
	if err != nil {
		t.Fatalf("ExtractTextFile failed: %v", err)
	}

	b, err := os.ReadFile(tmp)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}
	if len(b) == 0 {
		t.Fatal("expected non-empty text output")
	}
	if !strings.Contains(string(b), "SaiNageswar") {
		t.Errorf("output did not contain expected content: %s", string(b))
	}
}

func TestExtractText(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	text, err := ExtractText(ctx, fixturePDF, 1)
	if err != nil {
		t.Fatalf("ExtractTextFile failed: %v", err)
	}

	if len(text) == 0 {
		t.Fatal("expected non-empty text output")
	}
	if !strings.Contains(text, "SaiNageswar") {
		t.Errorf("output did not contain expected content: %s", string(text))
	}
}

func TestExtractStructuredText(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	blocks, err := ExtractStructuredText(ctx, fixturePDF)
	if err != nil {
		t.Fatalf("ExtractStructuredText failed: %v", err)
	}

	if len(blocks) == 0 {
		t.Fatal("expected non-empty structured text output")
	}
	if blocks[0].HeaderHierarchy == "" || blocks[0].Text == "" {
		t.Error("expected non-empty header hierarchy and text in structured block")
	}
}

func TestGetPageCount(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	pages, err := GetPageCount(ctx, fixturePDF)
	if err != nil {
		t.Fatalf("GetPageCount failed: %v", err)
	}
	if pages <= 0 {
		t.Errorf("expected positive page count, got %d", pages)
	}
}

func TestTextExtractor_Do(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	f, err := os.Open(fixturePDF)
	if err != nil {
		t.Fatalf("failed to open fixture PDF: %v", err)
	}
	defer f.Close()

	var out bytes.Buffer
	extractor := NewTextExtractor()
	if err := extractor.Do(ctx, f, &out); err != nil {
		t.Fatalf("Do failed: %v", err)
	}
	if out.Len() == 0 {
		t.Error("expected some text output")
	}
}
