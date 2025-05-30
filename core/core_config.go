// core_config.go
// Package core provides the central abstractions shared by all gizmo adapters.
// It follows Go's idioms: functional‑options, context propagation, and slog logging.

package core

import (
	"log/slog"
)

// Config captures execution parameters that are understood by every adapter.
// Adapter‑specific flags live in Extra.
// All fields are optional; zero values fall back to sensible defaults.

type Config struct {
	Pages   []int          // 1‑based page numbers; empty ⇒ all pages
	Format  string         // output format hint: "text", "png", … – adapter decides validity
	Extra   map[string]any // adapter‑specific key/value bag (string keys)
	WorkDir string         // override for any temp files the adapter needs
	Logger  *slog.Logger   // nil ⇒ slog.Default()
}

// Option mutates a Config – classic functional‑options pattern.

type Option func(*Config)

// WithPages restricts processing to the given 1‑based pages.
func WithPages(p ...int) Option {
	// Defensive copy to avoid surprises when caller’s slice changes.
	cp := append([]int(nil), p...)
	return func(c *Config) { c.Pages = cp }
}

// WithFormat sets an adapter‑agnostic format hint (e.g., "text", "png").
func WithFormat(f string) Option {
	return func(c *Config) { c.Format = f }
}

// WithExtra stores arbitrary adapter‑specific flags in Config.Extra.
func WithExtra(key string, val any) Option {
	return func(c *Config) {
		if c.Extra == nil {
			c.Extra = make(map[string]any, 1)
		}
		c.Extra[key] = val
	}
}

// WithWorkDir overrides the working directory temp path.
func WithWorkDir(dir string) Option {
	return func(c *Config) { c.WorkDir = dir }
}

// WithLogger injects a slog.Logger (use slog.Default when nil).
func WithLogger(l *slog.Logger) Option {
	return func(c *Config) { c.Logger = l }
}

// BuildConfig applies Option setters over defaults and returns the result.
// The returned Config is safe for concurrent read‑only access.
func BuildConfig(opts ...Option) *Config {
	cfg := &Config{
		Extra:  make(map[string]any, 4),
		Logger: slog.Default(),
	}
	for _, o := range opts {
		if o != nil {
			o(cfg)
		}
	}
	// Guarantee Extra is non‑nil for adapters; ensure Logger fallback
	if cfg.Extra == nil {
		cfg.Extra = make(map[string]any)
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	return cfg
}
