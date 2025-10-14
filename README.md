# gx - My Personal Tooling for Go

**gx** provides tools I think would be useful for Go, inspired by Bun's developer experience.

## Installation

```bash
go install github.com/omarshaarawi/gx/cmd/gx@latest
```

## Commands

### `gx outdated`

Shows which dependencies have newer versions available. Displays them in a table grouped by direct and indirect dependencies.

Standard tooling for this is awkward - you need to run `go list -u -m all` and parse through verbose output. Or use `go get -u` blindly which updates everything at once.

This gives you a clear overview before making any changes.

```bash
# See all outdated packages
gx outdated

# Only show direct dependencies
gx outdated --direct-only

# Only show major version updates
gx outdated --major-only
```

### `gx audit`

Scans your dependencies against the Go vulnerability database and shows any known security issues. Groups findings by severity with descriptions and links to details.

The official tool `govulncheck` is thorough but slow and verbose. This focuses on the dependency graph check which is faster and gives you what you need to know. Plus it has cleaner output grouped by severity and supports filtering.

```bash
# Scan all dependencies
gx audit

# Filter by severity level
gx audit --severity=high,critical

# JSON output for scripts/CI
gx audit --json
```

