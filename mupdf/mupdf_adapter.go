package mupdf

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"github.com/SaiNageswarS/gizmo/core"
)

// public aliases for registry keys
const (
	TextProcessor    = "mupdf-text"
	RenderProcessor  = "mupdf-render"  // PNG / PPM images
	OutlineProcessor = "mupdf-outline" // JSON outline extraction
)

// Binary discovery ----------------------------------------------------------------

var (
	binPath string
	once    sync.Once
	binErr  error
)

// discover searches $MUPDF_BIN, then PATH for mutool / mupdf.
func discover() (string, error) {
	once.Do(func() {
		candidates := []string{}
		if env := strings.TrimSpace(envOr("MUPDF_BIN", "")); env != "" {
			candidates = append(candidates, env)
		}
		exe := "mutool"
		if runtime.GOOS == "windows" {
			exe += ".exe"
		}
		candidates = append(candidates, exe)
		for _, c := range candidates {
			if p, err := exec.LookPath(c); err == nil {
				binPath = p
				break
			}
		}
		if binPath == "" {
			binErr = errors.New("MuPDF CLI (mutool) not found – install mupdf-tools or set $MUPDF_BIN")
			return
		}
		_ = checkVersion(binPath)
	})
	return binPath, binErr
}

func envOr(key, def string) string {
	if v, ok := syscall.Getenv(key); ok {
		return v
	}
	return def
}

func checkVersion(path string) error {
	cmd := exec.Command(path, "--version")
	b, err := cmd.Output()
	if err != nil {
		return err
	}
	parts := strings.Fields(string(b))
	if len(parts) < 2 {
		return nil
	}
	var major int
	if _, err = fmt.Sscanf(parts[1], "v%d", &major); err == nil && major < 1 {
		binErr = fmt.Errorf("MuPDF version too old (%s)", parts[1])
	}
	return nil
}

// Processor implementations -------------------------------------------------------

type processor struct {
	mode string
	args []string
}

func (p *processor) Do(ctx context.Context, in io.Reader, out io.Writer, opts ...core.Option) error {
	if _, err := discover(); err != nil {
		return err
	}
	cfg := core.BuildConfig(opts...)
	fileIn, ok := in.(*os.File)
	if !ok {
		return fmt.Errorf("mupdf: input must be a *os.File (got %T)", in)
	}

	args := append([]string{p.mode}, p.args...)
	if len(cfg.Pages) > 0 {
		args = append(args, "-p", intsToPageSpec(cfg.Pages))
	}
	if cfg.Format != "" {
		args = append(args, "-F", cfg.Format)
	}
	if dpi, ok := cfg.Extra["dpi"].(int); ok && dpi > 0 {
		args = append(args, "-r", fmt.Sprint(dpi))
	}
	args = append(args, "-o", "-")
	args = append(args, fileIn.Name())

	log.Printf("mupdf: running %s %s\n", binPath, strings.Join(args, " "))
	cmd := exec.CommandContext(ctx, binPath, args...)
	cmd.Stdout = out
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("mupdf: %w: %s", err, stderr.String())
	}
	return nil
}

func intsToPageSpec(pages []int) string {
	if len(pages) == 1 {
		return fmt.Sprint(pages[0])
	}
	ss := make([]string, len(pages))
	for i, p := range pages {
		ss[i] = fmt.Sprint(p)
	}
	return strings.Join(ss, ",")
}

// Registration helpers -----------------------------------------------------------

func init() {
	core.Register(TextProcessor, NewTextExtractor)
	core.Register(RenderProcessor, NewRenderer)
	core.Register(OutlineProcessor, NewOutlineExtractor)
}

func NewTextExtractor() core.Processor {
	return &processor{mode: "draw", args: []string{"-F", "txt"}}
}

func NewRenderer() core.Processor {
	return &processor{mode: "draw"}
}

func NewOutlineExtractor() core.Processor {
	return &processor{mode: "show", args: []string{"-S", "outline"}}
}

// Convenience wrappers -----------------------------------------------------------

func ExtractTextFile(ctx context.Context, src, dst string, pages ...int) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	proc := NewTextExtractor()
	return proc.Do(ctx, in, out, core.WithPages(pages...))
}

func ExtractText(ctx context.Context, path string, pages ...int) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	var buf bytes.Buffer
	proc := NewTextExtractor()
	err = proc.Do(ctx, f, &buf, core.WithPages(pages...))
	return buf.String(), err
}

// GetPageCount returns the total number of pages in the PDF file.
func GetPageCount(ctx context.Context, path string) (int, error) {
	if _, err := discover(); err != nil {
		return 0, err
	}

	cmd := exec.CommandContext(ctx, binPath, "info", path)
	out, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("mupdf info error: %w", err)
	}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "Pages:") {
			parts := strings.Fields(line)
			if len(parts) == 2 {
				return strconv.Atoi(parts[1])
			}
		}
	}
	return 0, errors.New("page count not found in info output")
}

// StructuredBlock represents a chunk of body text and its hierarchical header path.
//
// HeaderHierarchy: full path of headers (e.g. "Title | Section | Subsection").
// Text:            contiguous body paragraph(s) under that header path.
// PageNumber:      1‑based index of the first page where the text appears.
//
// The extraction runs in two passes:
//   1. Compute dynamic font‑size thresholds for Title / Section / Subsection.
//   2. Walk blocks again, maintaining the current header stack and emitting body
//      paragraphs annotated with that hierarchy.
//
// Requirements: MuPDF ≥1.21 (`mutool draw -F stext.json`).

type StructuredBlock struct {
	HeaderHierarchy string `json:"headerHierarchy"`
	Text            string `json:"text"`
	PageNumber      int    `json:"pageNumber"`
}

type stextJSON struct {
	Pages []struct {
		Blocks []struct {
			Type  string `json:"type"`
			Lines []struct {
				Font struct {
					Size float64 `json:"size"`
				} `json:"font"`
				Text string `json:"text"`
			} `json:"lines"`
		} `json:"blocks"`
	} `json:"pages"`
}

// ExtractStructuredText parses pdfPath with MuPDF, derives dynamic header
// thresholds, and returns aggregated body text blocks annotated with the full
// header hierarchy.
func ExtractStructuredText(ctx context.Context, pdfPath string) ([]StructuredBlock, error) {
	bin, err := discover()
	if err != nil {
		return nil, fmt.Errorf("mupdf binary not found: %w", err)
	}

	cmd := exec.CommandContext(ctx, bin, "draw", "-F", "stext.json", "-o", "-", pdfPath)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("mutool execution failed: %w\n%s", err, out.String())
	}

	var doc stextJSON
	if err := json.Unmarshal(out.Bytes(), &doc); err != nil {
		return nil, fmt.Errorf("unable to parse stext.json: %w", err)
	}

	titleSize, sectionSize, subSize, err := computeFontThresholds(&doc)
	if err != nil {
		return nil, err
	}

	classify := func(sz float64) string {
		switch {
		case titleSize > 0 && sz >= titleSize:
			return "Title"
		case sectionSize > 0 && sz >= sectionSize:
			return "Section"
		case subSize > 0 && sz >= subSize:
			return "Subsection"
		default:
			return ""
		}
	}

	// Current header context
	var curTitle, curSection, curSubsection string
	buildHierarchy := func() string {
		var parts []string
		if curTitle != "" {
			parts = append(parts, curTitle)
		}
		if curSection != "" {
			parts = append(parts, curSection)
		}
		if curSubsection != "" {
			parts = append(parts, curSubsection)
		}
		return strings.Join(parts, " | ")
	}

	// Aggregator state
	var (
		aggHierarchy string
		aggPage      int
		aggBuilder   strings.Builder
		blocks       []StructuredBlock
	)
	flush := func() {
		if aggBuilder.Len() == 0 {
			return
		}
		blocks = append(blocks, StructuredBlock{
			HeaderHierarchy: aggHierarchy,
			Text:            strings.TrimSpace(aggBuilder.String()),
			PageNumber:      aggPage,
		})
		aggBuilder.Reset()
	}

	for pageIdx, p := range doc.Pages {
		for _, blk := range p.Blocks {
			if blk.Type != "text" {
				continue
			}

			var maxSize float64
			var lineBuilder strings.Builder
			for _, ln := range blk.Lines {
				if ln.Font.Size > maxSize {
					maxSize = ln.Font.Size
				}
				t := strings.TrimSpace(ln.Text)
				if t != "" {
					lineBuilder.WriteString(t)
					lineBuilder.WriteString(" ")
				}
			}
			text := strings.TrimSpace(lineBuilder.String())
			if text == "" {
				continue
			}

			switch classify(maxSize) {
			case "Title":
				flush()
				curTitle, curSection, curSubsection = text, "", ""
			case "Section":
				flush()
				curSection, curSubsection = text, ""
			case "Subsection":
				flush()
				curSubsection = text
			default: // body paragraph
				hierarchy := buildHierarchy()
				if hierarchy == "" {
					// skip body paragraphs before any header is seen
					continue
				}
				if hierarchy != aggHierarchy {
					// new header context → flush existing aggregate
					flush()
					aggHierarchy = hierarchy
					aggPage = pageIdx + 1
				}
				// append paragraph to aggregator (newline‑separated)
				if aggBuilder.Len() > 0 {
					aggBuilder.WriteString("\n\n")
				}
				aggBuilder.WriteString(text)
			}
		}
	}
	// flush remaining aggregated text
	flush()

	return blocks, nil
}

// computeFontThresholds builds a histogram of rounded font sizes and returns up
// to the three largest distinct sizes as title, section, and subsection
// thresholds (missing ones are returned as 0).
func computeFontThresholds(doc *stextJSON) (title, section, sub float64, err error) {
	const eps = 0.5
	freq := map[int]int{}

	for _, p := range doc.Pages {
		for _, blk := range p.Blocks {
			if blk.Type != "text" {
				continue
			}
			for _, ln := range blk.Lines {
				sz := int(math.Round(ln.Font.Size + eps))
				freq[sz]++
			}
		}
	}
	if len(freq) == 0 {
		return 0, 0, 0, fmt.Errorf("no text detected in PDF")
	}

	var sizes []int
	for sz := range freq {
		sizes = append(sizes, sz)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(sizes)))

	switch len(sizes) {
	case 1:
		return float64(sizes[0]), 0, 0, nil
	case 2:
		return float64(sizes[0]), float64(sizes[1]), 0, nil
	default:
		return float64(sizes[0]), float64(sizes[1]), float64(sizes[2]), nil
	}
}
