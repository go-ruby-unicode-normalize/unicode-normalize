<p align="center"><img src="https://raw.githubusercontent.com/go-ruby-unicode-normalize/brand/main/social/go-ruby-unicode-normalize-unicode-normalize.png" alt="go-ruby-unicode-normalize/unicode-normalize" width="720"></p>

# unicode-normalize — go-ruby-unicode-normalize

[![Docs](https://img.shields.io/badge/docs-mkdocs--material-DC2626)](https://go-ruby-unicode-normalize.github.io/docs/)
[![License](https://img.shields.io/badge/license-BSD--3--Clause-blue)](LICENSE)
[![Go](https://img.shields.io/badge/go-1.26.4%2B-00ADD8)](https://go.dev/dl/)
[![Coverage](https://img.shields.io/badge/coverage-100%25-1a7f37)](#tests--coverage)

**A pure-Go (no cgo) reimplementation of Ruby's
[`String#unicode_normalize`](https://docs.ruby-lang.org/en/master/String.html#method-i-unicode_normalize)
and `String#unicode_normalized?`** — the `unicode_normalize` standard library —
matching MRI 4.0.5 (Unicode 17.0.0) byte-for-byte. It normalizes UTF-8 text to
any of the four Unicode Standard Annex #15 forms (NFC, NFD, NFKC, NFKD) and
reports whether a string is already normalized, **without any Ruby runtime**.

It is the Unicode-normalization backend for
[go-embedded-ruby](https://github.com/go-embedded-ruby/ruby), but is a
**standalone, reusable** module — a sibling of
[go-ruby-regexp](https://github.com/go-ruby-regexp/regexp) (the Onigmo engine),
[go-ruby-marshal](https://github.com/go-ruby-marshal/marshal) and
[go-ruby-yaml](https://github.com/go-ruby-yaml/yaml).

## API

```go
import normalize "github.com/go-ruby-unicode-normalize/unicode-normalize"

// Normalize returns s in the given form (NFC is the default form for Ruby's
// unicode_normalize when called with no argument).
func Normalize(s string, form normalize.Form) string

// IsNormalized reports whether s is already in the given form
// (Ruby's unicode_normalized?).
func IsNormalized(s string, form normalize.Form) bool
```

```go
normalize.Normalize("é", normalize.NFC)   // "é"  (composes e + acute)
normalize.Normalize("é", normalize.NFD)          // "é" (decomposes)
normalize.Normalize("ﬁ", normalize.NFKC)         // "fi"  (ligature -> ascii)
normalize.Normalize("①", normalize.NFKC)         // "1"
normalize.Normalize("Ａ", normalize.NFKC)         // "A"   (full-width -> ascii)
normalize.IsNormalized("é", normalize.NFC)       // true  (composed)
normalize.IsNormalized("é", normalize.NFC) // false (decomposed)
```

The four forms are the `normalize.Form` values `NFC`, `NFD`, `NFKC`, `NFKD`,
corresponding to Ruby's `:nfc`, `:nfd`, `:nfkc`, `:nfkd`. The zero value is
`NFC`, mirroring Ruby's default. `Form.String` returns the Ruby symbol name
(`":nfc"`, …) for diagnostics and for the rbgo binding layer.

> **What the host binds.** This library is the deterministic, interpreter-free
> core. The rbgo binding wires `Normalize` to `String#unicode_normalize` and
> `IsNormalized` to `String#unicode_normalized?`, defaults the form to `:nfc`,
> maps the `:nfc`/`:nfd`/`:nfkc`/`:nfkd` symbols to `Form`, raises
> `ArgumentError` on an unknown form symbol and on invalid UTF-8 byte sequences
> (matching MRI), and otherwise hands the bytes straight through.

## How it works

The heavy lifting is delegated to
[`golang.org/x/text/unicode/norm`](https://pkg.go.dev/golang.org/x/text/unicode/norm),
a pure-Go (cgo-free) implementation of the same Unicode standard. Both MRI 4.0.5
and `x/text` target **Unicode 17.0.0**, so they agree across the official
NormalizationTest corpus and the Ruby-specific edge cases (Hangul composition,
full/half-width and ligature compatibility, combining-mark ordering).

`x/text v0.38.0` gates its Unicode 17.0.0 tables behind the `go1.27` build tag
and otherwise falls back to Unicode 15.0.0. On this module's `go 1.26.4` floor
that fallback leaves a handful of characters added in Unicode 16.0/17.0
un-normalized relative to MRI:

- new **canonical** decompositions / compositions in some Indic and historic
  scripts (e.g. `U+16121` ⇄ `U+1611E U+1611E`, `U+105C9` ⇄ `U+105D2 U+0307`);
- new **compatibility** mappings folding to ASCII (`U+A7F1` → `S`, the outlined
  alphanumerics `U+1CCD6..U+1CCF9` → `A`–`Z` / `0`–`9`).

This package patches exactly those characters (`tables_unicode17.go` +
`patch.go`), so the result matches MRI on **every** toolchain. On `go1.27+`,
where `x/text` already expands them, the override runes never appear in its
output and the patch is a no-op.

## Tests & coverage

Two complementary layers, both run in CI on every supported platform:

- **Deterministic, ruby-free tests** pin the MRI-observed results and alone keep
  statement coverage at **100%**, so the Windows and cross-architecture lanes
  (where MRI is absent) still pass the gate.
- **MRI oracle tests** run differentially against the `ruby` binary: a curated
  edge-case corpus, a combining-sequence fuzz, and an exhaustive sweep of every
  assigned scalar value × all four forms. They skip themselves when `ruby` is
  absent and gate on `RUBY_VERSION >= "4.0"` (Unicode 17.0.0); the oracle script
  binmodes stdin/stdout so the UTF-8 bytes survive intact.

```sh
go test -race -coverpkg=./... -coverprofile=cover.out ./...   # 100.0%
go test -short ./...                                          # skip the exhaustive sweep
```

CI enforces the 100% coverage gate plus builds and tests on **three operating
systems** (Linux, macOS, Windows) and **all six supported 64-bit architectures**
(amd64, arm64, riscv64, loong64, ppc64le, s390x). The package is **cgo-free**
(`CGO_ENABLED=0`).

## License

BSD-3-Clause — see [LICENSE](LICENSE). Copyright (c) 2026, the
go-ruby-unicode-normalize/unicode-normalize authors.

## WebAssembly

Being pure Go (CGO=0), this library also compiles to **WebAssembly** — both
`GOOS=js GOARCH=wasm` (browser / Node.js) and `GOOS=wasip1 GOARCH=wasm` (WASI).
CI builds both targets on every push, alongside the six 64-bit native/qemu arches.

```sh
GOOS=js     GOARCH=wasm go build ./...   # browser / Node
GOOS=wasip1 GOARCH=wasm go build ./...   # WASI (wasmtime, wasmer, wasmedge, …)
```
