# Versioneer

A blazing-fast dependency scanner and security audit tool for polyglot codebases. Zero config, zero external dependencies, instant results.

Scan **55,000 dependencies across 4,000+ projects in ~10 seconds** — then sweep them for known vulnerabilities in one command.

```
$ versioneer -resolve -check='axios@<1.6.0,lodash@<4.17.21' ~/code

WARNING: 3 dependencies matched by name but version could not be resolved — marked UNRESOLVED.
FOUND 8 matching dependencies across rules.
NAME    SPEC     INSTALLED   ECO  TYPE    MANIFEST  DEPS DIR   SOURCE
──────  ───────  ──────────  ───  ──────  ────────  ─────────  ─────────────────────────────
axios   ^0.26.0  0.26.1      npm  direct  1mo ago   —          oss/AgentGPT/next/package.json
axios   ^0.21.1  0.21.4      npm  dev     9mo ago   —          oss/swc/package.json
lodash  4.17.15  4.17.15     npm  direct  6mo ago   3mo ago    oss/legacy-app/package.json
...
```

## Install

```sh
go install github.com/justsml/versioneer/cmd/versioneer@latest
```

Or build from source:

```sh
git clone https://github.com/justsml/versioneer && cd versioneer
go build -o versioneer ./cmd/versioneer
```

## What it does

**Scan** — walks a directory tree in parallel, discovers every project manifest, and parses out all dependencies with their version specs.

**Resolve** — looks up the *actual installed version* from lock files (`package-lock.json`, `yarn.lock`, `pnpm-lock.yaml`, `go.sum`, `Cargo.lock`) and on-disk package directories (`node_modules`, `.venv`, `vendor`). Walks up to monorepo roots automatically.

**Audit** — matches resolved versions against a set of rules (inline or from a file) to find vulnerable, malicious, or outdated packages. Flags unresolvable versions as `UNRESOLVED` so nothing slips through silently.

## Supported ecosystems

| Ecosystem | Manifests | Lock files / on-disk |
|-----------|-----------|---------------------|
| **npm** | `package.json` | `package-lock.json`, `yarn.lock`, `pnpm-lock.yaml`, `node_modules/` |
| **Go** | `go.mod` | `go.sum` |
| **Python** | `requirements.txt`, `Pipfile`, `pyproject.toml` | `.venv/`, `venv/` dist-info |
| **Rust** | `Cargo.toml` | `Cargo.lock` |
| **Java** | `pom.xml`, `build.gradle`, `build.gradle.kts` | — |
| **Ruby** | `Gemfile` | — |
| **PHP** | `composer.json` | — |
| **Dart** | `pubspec.yaml` | — |

## Output formats

Six built-in formats, all streaming (no buffering):

| Format | Flag | Use case |
|--------|------|----------|
| `table` | `-format=table` | Terminal (default) |
| `json` | `-format=json` | Full structured report |
| `jsonl` | `-format=jsonl` | Streaming / piping / log ingest |
| `csv` | `-format=csv` | Spreadsheets, data pipelines |
| `markdown` | `-format=markdown` | PRs, wikis, reports |
| `summary` | `-format=summary` | Quick stats + staleness overview |

When `-resolve` is active, all formats automatically include the resolved version and timestamp columns.

## Usage

### Basic scan

```sh
# Scan current directory
versioneer .

# Scan with actual installed versions + timestamps
versioneer -resolve ~/code

# Full JSON report
versioneer -resolve -format=json ~/code > report.json
```

### Filtering

```sh
# Find every project that uses react
versioneer -dep=react ~/code

# Only Python dependencies
versioneer -eco=python ~/code

# Only dev dependencies
versioneer -type=dev ~/code

# Combine filters
versioneer -resolve -dep=axios -eco=npm -format=csv ~/code
```

### Security sweeps

Check for known vulnerable or malicious packages — inline or from a rules file:

```sh
# Inline check (comma-separated rules)
versioneer -check='axios@<1.6.0,event-stream@=3.3.6,colors@>=1.4.1' ~/code

# From a rules file
versioneer -checkfile=vulns.txt ~/code
```

Exits with code **1** when matches are found (CI-friendly). Auto-enables version resolution.

### Rules file format

One rule per line. `#` comments and blank lines are fine.

```sh
# package@constraint — ranges are comma-separated within a rule
axios@<1.6.0                      # anything below 1.6.0
event-stream@=3.3.6               # exact malicious version
colors@>=1.4.1                    # protest-ware versions
ua-parser-js@>=0.7.29,<0.7.31    # supply chain attack range
lodash@<4.17.21                   # prototype pollution
@scope/pkg@>=2.0.0                # scoped packages work too
event-stream                      # name-only = any version
```

**Version matching behavior:**
- Matches against the **resolved/installed version** first (from lock files or `node_modules`)
- Pinned specs (e.g. `1.7.9`) are matched directly when no lock file is available
- Range expressions (e.g. `^1.3.5`) without a resolved version are flagged as `UNRESOLVED` — never silently skipped or false-matched

## Architecture

```
cmd/versioneer/main.go        CLI: flags, filtering, security checks
internal/
  scanner/scanner.go           Parallel filesystem walker (NumCPU workers)
  parser/                      Per-ecosystem manifest parsers
  resolver/                    Lock file + on-disk version resolution
    resolver.go                npm, Go, Rust, Python resolvers with lock cache
    timestamps.go              Manifest, project dir, deps dir modified times
  matcher/                     Semver parsing + constraint matching
  output/                      Streaming formatters (table, json, jsonl, csv, md, summary)
  model/dependency.go          Core types: Dependency, Project, ScanResult
```

**Design decisions:**
- **Zero external dependencies** — stdlib only, no cobra/viper/lipgloss
- **Parallel scanning** — `NumCPU` goroutine workers parse manifests concurrently after a single `WalkDir` pass
- **Lock file caching** — `sync.Map` cache ensures monorepo lock files are parsed exactly once even across hundreds of sub-packages
- **Streaming output** — formatters write directly to `io.Writer`, no intermediate buffering
- **Monorepo-aware** — lock files and `node_modules` lookups walk up to the repo root, stopping at `.git` boundaries

## Contributing

```sh
# Run tests
go test ./...

# Build
go build ./cmd/versioneer

# Test a scan
go run ./cmd/versioneer -resolve -format=summary ~/your-code
```

## License

MIT
