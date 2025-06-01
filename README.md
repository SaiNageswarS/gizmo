# Gizmo

**Gizmo** is a lightweight, extensible Go library that wraps command‑line tools like [MuPDF](https://mupdf.com) to provide clean, idiomatic APIs for processing media — such as extracting text from PDFs or rendering pages as images. It’s designed with plugin‑like flexibility, so future support for tools like `ffmpeg` or `ImageMagick` is seamless.

## ✨ Features

* 🧱 Modular **`Processor`** interface with built‑in registry
* 🧾 Plain PDF‑to‑text conversion using `mutool draw`
* 📚 **Layout‑aware structured text extraction** via **`ExtractStructuredText`** — a two‑pass algorithm that:

  1. Builds a font‑size histogram to auto‑detect **Title / Section / Subsection** thresholds.
  2. Aggregates consecutive paragraphs under each header path, returning blocks like `"Title | Section | Subsection" → text`.

  Perfect for **RAG pipelines, semantic chunking, and heading‑aware summarisation**.
* 📄 Page‑count detection via `mutool info`
* 🧪 Simple functional‑option config system
* 🧰 CLI‑friendly: works with `os.File`, `io.Reader`, or file paths

## 📦 Installation

```bash
go get github.com/your-org/gizmo
```

> Requires `mutool` from MuPDF (`sudo apt install mupdf-tools` on Ubuntu).

## 🧑‍💻 Usage

### Extract plain text from a PDF

```go
text, err := mupdf.ExtractText(ctx, "input.pdf", 1, 2, 3)
```

### Extract structured text with hierarchy (ideal for RAG)

```go
blocks, err := mupdf.ExtractStructuredText(ctx, "docs.pdf")
for _, b := range blocks {
    fmt.Printf("%s\n%s\n--- page %d\n\n", b.HeaderHierarchy, b.Text, b.PageNumber)
}
```

Each `StructuredBlock` contains:

| Field             | Example                          |               |
| ----------------- | -------------------------------- | ------------- |
| `HeaderHierarchy` | \`"Ch. 1 Introduction            | Background"\` |
| `Text`            | Body paragraphs (newline‑joined) |               |
| `PageNumber`      | `3`                              |               |

Use these blocks as ready‑made chunks for a **Retrieval‑Augmented Generation** index, preserving document structure for better answer grounding.

### Get page count

```go
pages, err := mupdf.GetPageCount(ctx, "input.pdf")
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

* [ ] `ffmpeg` processor for video/audio transformation
* [ ] OCR and searchable PDF extraction
* [ ] PDF‑to‑image rendering helpers

## 📜 License

Apache‑2.0 – see https://github.com/SaiNageswarS/gizmo/blob/master/LICENSE for details.
