<p align="center">
  <h1 align="center">Versioneer</h1>
  <p align="center">
    <strong>Blazing-fast dependency scanner & security audit tool for polyglot codebases</strong>
  </p>
  <p align="center">
    <a href="https://github.com/justsml/versioneer/actions"><img src="https://img.shields.io/github/actions/workflow/status/justsml/versioneer/ci.yml?branch=main&style=flat-square&logo=github&label=CI" alt="CI"></a>
    <a href="https://goreportcard.com/report/github.com/justsml/versioneer"><img src="https://goreportcard.com/badge/github.com/justsml/versioneer?style=flat-square" alt="Go Report Card"></a>
    <a href="https://pkg.go.dev/github.com/justsml/versioneer"><img src="https://img.shields.io/badge/go.dev-reference-007d9c?style=flat-square&logo=go&logoColor=white" alt="Go Reference"></a>
    <a href="https://github.com/justsml/versioneer/blob/main/LICENSE"><img src="https://img.shields.io/github/license/justsml/versioneer?style=flat-square" alt="License"></a>
    <a href="https://github.com/justsml/versioneer/releases"><img src="https://img.shields.io/github/v/release/justsml/versioneer?style=flat-square&logo=github" alt="Release"></a>
    <a href="https://github.com/justsml/versioneer/stargazers"><img src="https://img.shields.io/github/stars/justsml/versioneer?style=flat-square" alt="Stars"></a>
  </p>
</p>

<br>

> Zero config. Zero external dependencies. Instant results.
>
> Scan **55,000 dependencies across 4,000+ projects in ~10 seconds** — then sweep them for known vulnerabilities in one command.

<br>

<details>
<summary><strong>See it in action</strong></summary>

```
$ versioneer -resolve -check='axios@<1.6.0,lodash@<4.17.21' ~/

WARNING: 3 dependencies matched by name but version could not be resolved — marked UNRESOLVED.
FOUND 8 matching dependencies across rules.

── code/AgentGPT/next/package.json (1 deps) ──────────────────────────
NAME   SPEC     INSTALLED  VIA       ECO  TYPE    MANIFEST  DEPS DIR
─────  ───────  ─────────  ────────  ───  ──────  ────────  ────────
axios  ^0.26.0  0.26.1     lockfile  npm  direct  1mo ago   —

── code/swc/package.json (1 deps) ─────────────────────────────────────
NAME   SPEC     INSTALLED  VIA       ECO  TYPE  MANIFEST  DEPS DIR
─────  ───────  ─────────  ────────  ───  ────  ────────  ────────
axios  ^0.21.1  0.21.4     lockfile  npm  dev   9mo ago   —

── code/legacy-app/package.json (1 deps) ──────────────────────────────
NAME    SPEC     INSTALLED  VIA   ECO  TYPE    MANIFEST  DEPS DIR
──────  ───────  ─────────  ────  ───  ──────  ────────  ────────
lodash  4.17.15  4.17.15    disk  npm  direct  6mo ago   3mo ago
...

8 dependencies across 4 projects (scanned in 1.23s)
```

</details>

---

## Highlights

- **8 ecosystems** — npm, Go, Python, Rust, Java, Ruby, PHP, Dart
- **Lock file resolution** — `package-lock.json`, `yarn.lock`, `pnpm-lock.yaml`, `bun.lock`, `go.sum`, `Cargo.lock`, plus on-disk and exec fallbacks
- **Security sweep** — match resolved versions against inline rules or a rules file; CI-friendly exit codes
- **Pure Go, zero deps** — stdlib only, single static binary
- **Concurrent pipeline** — parallel directory walking, streaming parsers, pipelined resolve
- **6 output formats** — table, JSON, JSONL, CSV, Markdown, summary

---

## Install

### Go

```sh
go install github.com/justsml/versioneer/cmd/versioneer@latest
```

### Curl (Linux / macOS)

```sh
curl -fsSL "https://github.com/justsml/versioneer/releases/latest/download/versioneer-$(uname -s | tr '[:upper:]' '[:lower:]')-$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/')" \
  -o /usr/local/bin/versioneer && chmod +x /usr/local/bin/versioneer
```

### From source

```sh
git clone https://github.com/justsml/versioneer && cd versioneer
go build -o versioneer ./cmd/versioneer
```

---

## Quick start

```sh
# Scan current directory (resolves versions by default)
versioneer .

# Skip version resolution for a faster scan
versioneer -no-resolve ~/app

# Full JSON report
versioneer -format=json ~/app > report.json
```

---

## Usage

### Filtering

```sh
# Find every project that uses react
versioneer -dep=react ~/

# Only Python dependencies
versioneer -eco=python ~/

# Only dev dependencies
versioneer -type=dev ~/code

# Combine filters
versioneer -dep=axios -eco=npm -format=csv ~/code
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

<details>
<summary><strong>Rules file format</strong></summary>

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

</details>

---

## Supported ecosystems

| Ecosystem | Manifests | Lock files / on-disk resolution |
|-----------|-----------|-------------------------------|
| **npm** | `package.json` | `package-lock.json`, `yarn.lock`, `pnpm-lock.yaml`, `bun.lock`, `bun pm ls` (binary lockb), `node_modules/` |
| **Go** | `go.mod` | `go.sum` |
| **Python** | `requirements.txt`, `Pipfile`, `pyproject.toml` | `.venv/`, `venv/` dist-info |
| **Rust** | `Cargo.toml` | `Cargo.lock` |
| **Java** | `pom.xml`, `build.gradle`, `build.gradle.kts` | — |
| **Ruby** | `Gemfile` | — |
| **PHP** | `composer.json` | — |
| **Dart** | `pubspec.yaml` | — |

## Output formats

| Format | Flag | Use case |
|--------|------|----------|
| `table` | `-format=table` | Terminal (default) |
| `json` | `-format=json` | Full structured report |
| `jsonl` | `-format=jsonl` | Streaming / piping / log ingest |
| `csv` | `-format=csv` | Spreadsheets, data pipelines |
| `markdown` | `-format=markdown` | PRs, wikis, reports |
| `summary` | `-format=summary` | Quick stats + staleness overview |

All formats include the resolved version, resolution source (`lockfile`/`disk`/`exec`), and timestamp columns by default. Use `-no-resolve` to omit them.

---

## Architecture

```
cmd/versioneer/main.go        CLI entry point — flags, filtering, security checks
internal/
  scanner/scanner.go           Parallel directory walker + streaming project emitter
  parser/                      Per-ecosystem manifest parsers (8 ecosystems)
  resolver/                    Lock file + on-disk + exec version resolution
    resolver.go                npm, Go, Rust, Python, Bun resolvers + lock cache
    timestamps.go              Manifest, project dir, deps dir modified times
  matcher/                     Semver parsing + constraint matching
  output/                      Streaming formatters (table, json, jsonl, csv, md, summary)
  model/dependency.go          Core types: Dependency, Project, ScanResult
```

<details>
<summary><strong>Design decisions</strong></summary>

- **Zero external dependencies** — stdlib only, no cobra/viper/lipgloss
- **3-stage pipeline** — directory walking, manifest parsing, and version resolution run as concurrent pipeline stages. Resolution runs by default and begins while scanning is still discovering projects
- **Parallel directory walker** — goroutine-per-directory bounded by NumCPU semaphore, faster than `filepath.WalkDir` on SSDs. Falls back to sequential walk when `--gitignore` is enabled
- **Streaming lock file parsers** — package-lock.json uses a streaming JSON token decoder (skips integrity hashes without allocating), yarn.lock uses a line-based state machine (13x faster than regex), Cargo.lock and go.sum use `bufio.Scanner`
- **Lock file caching** — `sync.Map` + `sync.Once` ensures monorepo lock files are parsed exactly once even across hundreds of sub-packages
- **Resolution source tracking** — every resolved version is tagged `lockfile`, `disk`, or `exec`
- **Bun support** — parses `bun.lock` JSONC, falls back to `bun pm ls` for binary `bun.lockb`, then to `node_modules/`
- **Streaming output** — formatters write directly to `io.Writer`, no intermediate buffering
- **Monorepo-aware** — lock file lookups walk up to the repo root, stopping at `.git` boundaries

</details>

---

## Comparison

<details>
<summary><strong>How does Versioneer compare to other tools?</strong></summary>

| Tool | License | Commercial | Ecosystems | Built With |
| ---- | ------- | ---------- | ---------- | ---------- |
| **Versioneer** | MIT | No | 8 | Go (zero deps) |
| [Snyk CLI](https://github.com/snyk/cli) | Apache-2.0 | Yes (freemium) | 10+ | TypeScript |
| [Socket CLI](https://github.com/SocketDev/socket-cli) | MIT | Yes (freemium) | 3 | TypeScript |
| [Trivy](https://github.com/aquasecurity/trivy) | Apache-2.0 | Yes (Aqua) | 10+ | Go |
| [Grype](https://github.com/anchore/grype) | Apache-2.0 | Yes (Anchore) | 10+ | Go |
| [OSV-Scanner](https://github.com/google/osv-scanner) | Apache-2.0 | No | 12+ | Go |
| [Dependency-Check](https://github.com/jeremylong/DependencyCheck) | Apache-2.0 | No | 7+ | Java |
| [Retire.js](https://github.com/RetireJS/retire.js) | Apache-2.0 | No | 1 | JavaScript |
| [safety](https://github.com/pyupio/safety) | MIT | Yes (freemium) | 1 | Python |
| [audit.js](https://github.com/sonatype-nexus-community/auditjs) | Apache-2.0 | Partial | 1 | TypeScript |

</details>

---

## Contributing

```sh
# Run tests
go test ./...

# Run benchmarks
go test -bench=. -benchmem ./internal/resolver/

# Build
go build ./cmd/versioneer

# Test a scan
go run ./cmd/versioneer -format=summary ~/your-code
```

---

## License

MIT
