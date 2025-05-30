# Gizmo

**Gizmo** is a lightweight, extensible Go library that wraps command-line tools like [MuPDF](https://mupdf.com) to provide clean, idiomatic APIs for processing media — such as extracting text from PDFs or rendering pages as images. It’s designed with plugin-like flexibility, so future support for tools like `ffmpeg` or `ImageMagick` is seamless.

## ✨ Features

- 🧱 Modular `Processor` interface with built-in registry
- 🧾 PDF-to-text conversion using `mutool draw`
- 📄 Page count detection via `mutool info`
- 🧪 Simple functional-option config system
- 🧰 CLI-friendly: works with `os.File`, `io.Reader`, or file paths

## 📦 Installation

```bash
go get github.com/your-org/gizmo
```

> Requires `mutool` from MuPDF (`sudo apt install mupdf-tools` on Ubuntu).

## 🧑‍💻 Usage

### Extract text from a PDF

```go
import (
	"context"
	"log"
	"github.com/your-org/gizmo/mupdf"
)

func main() {
	text, err := mupdf.ExtractText(context.Background(), "input.pdf", 1, 2, 3)
	if err != nil {
		log.Fatal(err)
	}
}
```

### Get page count

```go
pages, err := mupdf.GetPageCount(context.Background(), "input.pdf")
```

## 🧪 Tests

A sample test PDF is stored under `testdata/sample.pdf`. Run all tests with:

```bash
go test ./...
```


## 📂 Project Layout

```
gizmo/
├── core/            # Shared interfaces, config
├── mupdf/           # MuPDF processor (text, render, outline)
├── testdata/        # Fixture PDFs
```

## 🔮 Roadmap

- [ ] `ffmpeg` processor for video/audio transformation
- [ ] OCR and searchable PDF extraction
- [ ] PDF-to-image rendering helpers

## 📜 License

MIT © 2025 [Your Name / Org]
