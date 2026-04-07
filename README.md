# Go-sopter

[![CI](https://github.com/olenindenis/go-struct-size-optimizer/actions/workflows/ci.yml/badge.svg)](https://github.com/olenindenis/go-struct-size-optimizer/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-green.svg)](https://opensource.org/licenses/MIT)
[![Go Reference](https://pkg.go.dev/badge/github.com/olenindenis/go-struct-size-optimizer.svg)](https://pkg.go.dev/github.com/olenindenis/go-struct-size-optimizer)
[![Go Report Card](https://goreportcard.com/badge/github.com/olenindenis/go-version)](https://goreportcard.com/report/github.com/olenindenis/go-struct-size-optimizer)

## Go-sopter (Golang struct size optimization tool)

A static analysis tool for Go that detects structs with suboptimal field ordering and rewrites them to minimize memory usage through better alignment.

## How it works

The analyzer (`structalign`) inspects every struct in a Go package and simulates the memory layout using the target platform's alignment rules. If reordering the fields would reduce the struct size, it reports a diagnostic with the current and optimized sizes plus a colored inline diff.

**Optimization strategy:**

1. **Hot fields first** — fields whose names contain `count`, `flag`, `state`, or `status` are placed at the top of the struct (likely to be accessed frequently and benefit from cache locality).
2. **Descending alignment within each group** — fields are sorted by their alignment requirement (largest first) to minimize padding bytes inserted by the compiler.

**Skipped structs:**

- Structs where any field has a struct tag (`json:"..."`, `db:"..."`, etc.) — tag order often has semantic meaning (e.g. serialization order).
- Structs preceded by a `//structalign:ignore` comment.
- Empty structs.

## Installation

```bash
go install github.com/olenindenis/go-struct-size-optimizer/cmd/gosoper@latest
```

## Usage

```bash
# Analyze packages in the current directory
gosoper

# Analyze a specific directory
gosoper -path /path/to/project

# Analyze a specific package pattern within a directory
gosoper -path /path/to/project ./pkg/models/...

# Rewrite source files in place (also prints diagnostics)
gosoper -w

# Rewrite files in a specific directory
gosoper -path /path/to/project -w
```

**Flags:**

| Flag | Default | Description |
|------|---------|-------------|
| `-path` | current directory | Directory to analyze |
| `-w` | false | Rewrite source files in place |

When no package pattern is provided, `./...` is used — all packages under the target directory are analyzed.

## Example

Given this struct:

```go
type Request struct {
    A bool
    B int64
    C bool
}
```

Running the analyzer prints:

```
test.go:2:1: struct can be optimized: 24 -> 16 bytes
Diff:
────────────────────────────────────────────────────────
 1   struct {
 2 - 	A        bool
 3 - 	B        int64
 4 - 	C        bool
     + 	B        int64
     + 	A        bool
     + 	C        bool
 5   }
────────────────────────────────────────────────────────
analysis complete
```

Running with `-w` rewrites the file **and** prints the same diagnostics so you can see what was changed.

**Hot field example:**

A field is considered "hot" when its name contains `count`, `flag`, `state`, or `status`. Hot fields are placed first (sorted by alignment descending), followed by the remaining cold fields (also sorted by alignment descending). The reorder is only applied when the resulting struct is strictly smaller.

```go
// Before — status (hot, align=8) is buried after a smaller field, causing padding
type Response struct {
    Code   int32  // align=4
    status int64  // align=8, hot field
    Done   bool   // align=1
}
// Size: 24 bytes (4-byte padding inserted before status)

// After — hot field first eliminates the padding
type Response struct {
    status int64  // align=8, hot field
    Code   int32  // align=4
    Done   bool   // align=1
}
// Size: 16 bytes
```

## Ignore directive

To opt a struct out of analysis, add a comment on the preceding line:

```go
//structalign:ignore
type LegacyStruct struct {
    A bool
    B int64
    C bool
}
```

## Development

Requirements: Docker (used by the Makefile targets).

```bash
# Run linter
make lint

# Run tests
make test
```

To run without Docker:

```bash
go build -v -o tool cmd/gosoper/main.go
go test -v -race ./...
```