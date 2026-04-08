# Versioneer

A blazing-fast dependency scanner and security audit tool for polyglot codebases. Zero config, zero external dependencies, instant results.

Scan **55,000 dependencies across 4,000+ projects in ~10 seconds** — then sweep them for known vulnerabilities in one command.

```
$ versioneer -resolve -check='axios@<1.6.0,lodash@<4.17.21' ~/

WARNING: 3 dependencies matched by name but version could not be resolved — marked UNRESOLVED.
FOUND 8 matching dependencies across rules.
NAME    SPEC     INSTALLED   VIA       ECO  TYPE    MANIFEST  DEPS DIR   SOURCE
──────  ───────  ──────────  ────────  ───  ──────  ────────  ─────────  ─────────────────────────────
axios   ^0.26.0  0.26.1      lockfile  npm  direct  1mo ago   —          ~/code/AgentGPT/next/package.json
axios   ^0.21.1  0.21.4      lockfile  npm  dev     9mo ago   —          ~/code/swc/package.json
lodash  4.17.15  4.17.15     disk      npm  direct  6mo ago   3mo ago    ~/code/legacy-app/package.json
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

**Scan** — walks a directory tree using a concurrent parallel walker (goroutine-per-directory bounded by NumCPU), discovers every project manifest, and parses out all dependencies with their version specs.

**Resolve** — looks up the *actual installed version* from lock files (`package-lock.json`, `yarn.lock`, `pnpm-lock.yaml`, `bun.lock`, `go.sum`, `Cargo.lock`), on-disk package directories (`node_modules`, `.venv`, `vendor`), and package manager exec (`bun pm ls` for binary `bun.lockb`). Walks up to monorepo roots automatically. Each resolved version is tagged with its source (`lockfile`, `disk`, or `exec`) so you know exactly where the version came from.

**Audit** — matches resolved versions against a set of rules (inline or from a file) to find vulnerable, malicious, or outdated packages. Flags unresolvable versions as `UNRESOLVED` so nothing slips through silently.

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

Six built-in formats, all streaming (no buffering):

| Format | Flag | Use case |
|--------|------|----------|
| `table` | `-format=table` | Terminal (default) |
| `json` | `-format=json` | Full structured report |
| `jsonl` | `-format=jsonl` | Streaming / piping / log ingest |
| `csv` | `-format=csv` | Spreadsheets, data pipelines |
| `markdown` | `-format=markdown` | PRs, wikis, reports |
| `summary` | `-format=summary` | Quick stats + staleness overview |

When `-resolve` is active, all formats include the resolved version, resolution source (`lockfile`/`disk`/`exec`), and timestamp columns. All formats report project count, dependency count, and scan duration — table, markdown, and summary embed it in the output; csv and jsonl print it to stderr.

## Usage

### Basic scan

```sh
# Scan current directory
versioneer .

# Scan with actual installed versions + timestamps
versioneer -resolve ~/app

# Full JSON report
versioneer -resolve -format=json ~/app > report.json
```

### Filtering

```sh
# Find every project that uses react
versioneer -dep=react ~/

# Only Python dependencies
versioneer -eco=python ~/

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
cmd/versioneer/main.go        CLI: flags, filtering, security checks, scan↔resolve pipeline
internal/
  scanner/scanner.go           Parallel directory walker + streaming project emitter
  parser/                      Per-ecosystem manifest parsers (8 ecosystems)
  resolver/                    Lock file + on-disk + exec version resolution
    resolver.go                npm, Go, Rust, Python, Bun resolvers with lock cache
    timestamps.go              Manifest, project dir, deps dir modified times
  matcher/                     Semver parsing + constraint matching
  output/                      Streaming formatters (table, json, jsonl, csv, md, summary)
  model/dependency.go          Core types: Dependency, Project, ScanResult
```

**Design decisions:**
- **Zero external dependencies** — stdlib only, no cobra/viper/lipgloss
- **3-stage pipeline** — directory walking, manifest parsing, and version resolution run as concurrent pipeline stages. When `-resolve` is active, resolution begins while scanning is still discovering projects
- **Parallel directory walker** — goroutine-per-directory bounded by NumCPU semaphore, faster than `filepath.WalkDir` on SSDs. Falls back to sequential walk when `--gitignore` is enabled (gitignore rules require parent-first ordering)
- **Streaming lock file parsers** — package-lock.json uses a streaming JSON token decoder (skips integrity hashes without allocating), yarn.lock uses a line-based state machine (13x faster than regex), Cargo.lock and go.sum use `bufio.Scanner`
- **Lock file caching** — `sync.Map` + `sync.Once` ensures monorepo lock files are parsed exactly once even across hundreds of sub-packages
- **Resolution source tracking** — every resolved version is tagged `lockfile`, `disk`, or `exec` so you can tell at a glance whether a version came from a lock file, was found on disk, or was obtained by running a package manager
- **Bun support** — parses `bun.lock` text format (JSONC with zero-dependency stripping), falls back to `bun pm ls` for binary `bun.lockb`, then to `node_modules/` traversal
- **Streaming output** — formatters write directly to `io.Writer`, no intermediate buffering
- **Monorepo-aware** — lock files and dependency directory lookups walk up to the repo root, stopping at `.git` boundaries

## Similar tools

| Tool | Description | License | Commercial | Ecosystems | Built With | Stars | Downloads | Last Updated |
| ---- | ----------- | ------- | ---------- | ---------- | ---------- | ----- | --------- | ------------ |
| [Snyk CLI](https://github.com/snyk/cli) | Developer-first security tool for finding and fixing vulnerabilities in dependencies, containers, and IaC | Apache-2.0 | Yes (freemium, login required) | npm, Java, Python, Go, .NET, PHP, Ruby, Rust, Swift, containers, IaC | TypeScript | [![GitHub Stars](https://img.shields.io/github/stars/snyk/cli?style=flat-square)](https://github.com/snyk/cli) | [![npm](https://img.shields.io/npm/dw/snyk?style=flat-square)](https://www.npmjs.com/package/snyk) | [![GitHub last commit](https://img.shields.io/github/last-commit/snyk/cli?style=flat-square)](https://github.com/snyk/cli/commits) |
| [Socket CLI](https://github.com/SocketDev/socket-cli) | Detects supply chain attacks, malware, and risky dependencies proactively | MIT | Yes (freemium, API key required) | npm, Python, Go | TypeScript | [![GitHub Stars](https://img.shields.io/github/stars/SocketDev/socket-cli?style=flat-square)](https://github.com/SocketDev/socket-cli) | [![npm](https://img.shields.io/npm/dw/socket?style=flat-square)](https://www.npmjs.com/package/socket) | [![GitHub last commit](https://img.shields.io/github/last-commit/SocketDev/socket-cli?style=flat-square)](https://github.com/SocketDev/socket-cli/commits) |
| [Trivy](https://github.com/aquasecurity/trivy) | All-in-one security scanner for vulnerabilities, misconfigurations, secrets, and SBOM | Apache-2.0 | Yes (Aqua platform), no login | npm, Python, Go, Java, .NET, PHP, Ruby, Rust, Dart, containers, IaC | Go | [![GitHub Stars](https://img.shields.io/github/stars/aquasecurity/trivy?style=flat-square)](https://github.com/aquasecurity/trivy) | — | [![GitHub last commit](https://img.shields.io/github/last-commit/aquasecurity/trivy?style=flat-square)](https://github.com/aquasecurity/trivy/commits) |
| [Grype](https://github.com/anchore/grype) | Vulnerability scanner for container images and filesystems | Apache-2.0 | Yes (Anchore Enterprise), no login | npm, Python, Go, Java, .NET, Ruby, Rust, PHP, containers, SBOM | Go | [![GitHub Stars](https://img.shields.io/github/stars/anchore/grype?style=flat-square)](https://github.com/anchore/grype) | — | [![GitHub last commit](https://img.shields.io/github/last-commit/anchore/grype?style=flat-square)](https://github.com/anchore/grype/commits) |
| [OSV-Scanner](https://github.com/google/osv-scanner) | Google-backed scanner using the OSV database for known vulnerabilities | Apache-2.0 | No, no login | npm, Python, Go, Java, .NET, Rust, Dart, Ruby, PHP, Elixir, R | Go | [![GitHub Stars](https://img.shields.io/github/stars/google/osv-scanner?style=flat-square)](https://github.com/google/osv-scanner) | — | [![GitHub last commit](https://img.shields.io/github/last-commit/google/osv-scanner?style=flat-square)](https://github.com/google/osv-scanner/commits) |
| [OWASP Dependency-Check](https://github.com/jeremylong/DependencyCheck) | SCA tool that detects publicly disclosed vulnerabilities in project dependencies | Apache-2.0 | No, NVD API key recommended | Java (primary), npm, .NET, Python, Ruby, PHP, Go | Java | [![GitHub Stars](https://img.shields.io/github/stars/jeremylong/DependencyCheck?style=flat-square)](https://github.com/jeremylong/DependencyCheck) | — | [![GitHub last commit](https://img.shields.io/github/last-commit/jeremylong/DependencyCheck?style=flat-square)](https://github.com/jeremylong/DependencyCheck/commits) |
| [Retire.js](https://github.com/RetireJS/retire.js) | Detects JavaScript libraries with known vulnerabilities | Apache-2.0 | No, no login | JavaScript/npm | JavaScript | [![GitHub Stars](https://img.shields.io/github/stars/RetireJS/retire.js?style=flat-square)](https://github.com/RetireJS/retire.js) | [![npm](https://img.shields.io/npm/dw/retire?style=flat-square)](https://www.npmjs.com/package/retire) | [![GitHub last commit](https://img.shields.io/github/last-commit/RetireJS/retire.js?style=flat-square)](https://github.com/RetireJS/retire.js/commits) |
| [safety](https://github.com/pyupio/safety) | Python dependency checker scanning against the Safety DB | MIT | Yes (freemium, API key required) | Python | Python | [![GitHub Stars](https://img.shields.io/github/stars/pyupio/safety?style=flat-square)](https://github.com/pyupio/safety) | — | [![GitHub last commit](https://img.shields.io/github/last-commit/pyupio/safety?style=flat-square)](https://github.com/pyupio/safety/commits) |
| [audit.js](https://github.com/sonatype-nexus-community/auditjs) | Sonatype-powered auditor for npm packages using the OSS Index | Apache-2.0 | Partial (free + Nexus IQ), no login | JavaScript/npm | TypeScript | [![GitHub Stars](https://img.shields.io/github/stars/sonatype-nexus-community/auditjs?style=flat-square)](https://github.com/sonatype-nexus-community/auditjs) | [![npm](https://img.shields.io/npm/dw/auditjs?style=flat-square)](https://www.npmjs.com/package/auditjs) | [![GitHub last commit](https://img.shields.io/github/last-commit/sonatype-nexus-community/auditjs?style=flat-square)](https://github.com/sonatype-nexus-community/auditjs/commits) |

## Contributing

```sh
# Run tests
go test ./...

# Run benchmarks
go test -bench=. -benchmem ./internal/resolver/

# Build
go build ./cmd/versioneer

# Test a scan
go run ./cmd/versioneer -resolve -format=summary ~/your-code
```

## License

MIT
