package mupdf

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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
